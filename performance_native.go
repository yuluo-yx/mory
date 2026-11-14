//go:build performance

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// This local measurement adapter is absent from release and E2E builds. It
// drives the real app paths on the UI thread without transferring large editor
// values over accessibility. Captures include rendering and PNG encoding.
func init() {
	// Benchmark windows must not inherit or overwrite the user's window state.
	mygo.App.SetWindowStateStore(&performanceWindowStore{})
	nativeReadyHook = func(a *app) {
		path := os.Getenv("MORY_PERFORMANCE_SOCKET")
		if path == "" {
			return
		}
		width, _ := strconv.Atoi(os.Getenv("MORY_PERFORMANCE_WIDTH"))
		height, _ := strconv.Atoi(os.Getenv("MORY_PERFORMANCE_HEIGHT"))
		if width > 0 && height > 0 {
			a.win.SetContentSize(width, height)
		}
		listener, err := net.Listen("unix", path)
		if err != nil {
			panic(err)
		}
		if err := os.Chmod(path, 0600); err != nil {
			listener.Close()
			panic(err)
		}
		go func() {
			defer listener.Close()
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			decoder, encoder := json.NewDecoder(conn), json.NewEncoder(conn)
			for {
				var req struct{ Action, Path, Mode, Position, Text string }
				if err := decoder.Decode(&req); err != nil {
					return
				}
				if req.Action == "quit" {
					mygo.RunOnMain(func() { mygo.App.Quit() })
					return
				}
				if req.Action == "stats" {
					if err := encoder.Encode(performanceResult(0, 0, "")); err != nil {
						return
					}
					continue
				}
				begin := time.Now()
				var actionMS float64
				var width, height int
				var problem string
				var tableState *ui.ListState
				var tableRow, tableRows int
				done := make(chan struct{})
				a.win.Update(func() {
					defer close(done)
					start := time.Now()
					width, height = a.win.ContentSize()
					switch req.Action {
					case "open":
						a.openExternal(req.Path)
						if req.Mode != "" && req.Mode != "Write" {
							a.changeMode(req.Mode)
						}
					case "mode":
						a.changeMode(req.Mode)
					case "edit":
						performanceInsert(a, req.Text, req.Position)
						a.capture()
					case "undo":
						if a.mode == "Source" {
							a.code.Buffer.Undo()
						} else if a.blockEditor != nil {
							a.blockUndo(false)
						} else {
							a.doc.Undo()
						}
						a.capture()
					case "save":
						// Insert before timing, within this callback, so autosave cannot
						// turn the measured save into a no-op. Serialization stays timed.
						performanceInsert(a, "s", "start")
						start = time.Now()
						a.save()
					case "scroll":
						if a.mode == "Source" {
							a.code.SetScrollY(a.code.ScrollY() + 400)
						}
					case "table-scroll":
						tableState, tableRow, tableRows = performanceTableScroll(a, req.Position)
						if tableState == nil {
							problem = "no virtualized table is available"
						}
					case "capture", "stats":
					default:
						problem = fmt.Sprintf("unknown action: %s", req.Action)
					}
					if a.problem != "" {
						problem = a.problem
					}
					actionMS = float64(time.Since(start)) / float64(time.Millisecond)
				})
				<-done
				if req.Action != "stats" {
					_, err = a.win.CapturePage()
					if err != nil {
						problem = err.Error()
					}
				}
				elapsedMS := float64(time.Since(begin)) / float64(time.Millisecond)
				result := performanceResult(actionMS, elapsedMS, problem)
				if tableState != nil {
					inspected := make(chan struct{})
					a.win.Update(func() {
						first, last := tableState.Visible()
						result["table_first_visible_row"], result["table_last_visible_row"] = first, last
						close(inspected)
					})
					<-inspected
					result["table_target_row"], result["table_rows"] = tableRow, tableRows
				}
				result["window_width"], result["window_height"] = width, height
				result["effective_mode"] = a.mode
				if err := encoder.Encode(result); err != nil {
					return
				}
			}
		}()
	}
}

// Drive the actual nested list, including jumps beyond rows measured so far.
func performanceTableScroll(a *app, position string) (*ui.ListState, int, int) {
	if a.blockEditor == nil {
		return nil, 0, 0
	}
	for index, node := range a.blockEditor.visibleBlocks() {
		if node.Kind != "table" {
			continue
		}
		table, simple := a.tableLayout(node)
		if !simple || table.Rows < 200 {
			continue
		}
		a.blockEditor.list.ScrollTo(index, ui.Start)
		row := 1
		switch position {
		case "middle":
			row = table.Rows / 2
		case "end":
			row = table.Rows - 1
		}
		state := a.blockEditor.tables[node.ID].rows
		state.ScrollTo(row, ui.Start)
		return state, row, table.Rows
	}
	return nil, 0, 0
}

type performanceWindowStore struct{ states map[string]mygo.WindowState }

func (s *performanceWindowStore) LoadWindowStates() (map[string]mygo.WindowState, error) {
	return s.states, nil
}
func (s *performanceWindowStore) SaveWindowStates(states map[string]mygo.WindowState) error {
	s.states = states
	return nil
}

func performanceResult(actionMS, capturedMS float64, problem string) map[string]any {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		problem = err.Error()
	}
	cpu := float64(usage.Utime.Sec+usage.Stime.Sec) + float64(usage.Utime.Usec+usage.Stime.Usec)/1e6
	return map[string]any{"action_ms": actionMS, "captured_ms": capturedMS, "heap_bytes": memory.HeapAlloc, "go_sys_bytes": memory.Sys, "cpu_seconds": cpu, "error": problem}
}

func performanceInsert(a *app, text, position string) {
	if a.mode == "Source" || a.mode == "Split" {
		pos := 0
		if position == "end" {
			pos = a.code.Buffer.Snapshot().Len()
		}
		a.code.Select(pos, pos)
		a.code.Insert(text)
	} else {
		if a.doc == nil && a.blockEditor != nil {
			a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
				if a.doc != nil || n.Kind == "title" || !blocks.IsText(n) {
					return
				}
				a.blockEditor.active = n.ID
				a.blockField(n, a.documentColor)
			})
		}
		if a.doc == nil {
			if a.blockEditor != nil {
				a.insertNodes(blocks.Paragraph(text))
			}
			return
		}
		pos := 0
		if position == "end" {
			pos = a.doc.Text().Len()
		}
		a.doc.SetSelection(pos, pos)
		a.doc.InsertText(text)
	}
}
