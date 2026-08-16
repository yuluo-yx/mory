package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yuluo-yx/mory/internal/windowshost"
)

type cdpMessage struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type cdpClient struct {
	connection *websocket.Conn
	nextID     int
	events     []cdpMessage
}

func exportWithEdge(ctx context.Context, request windowshost.ExportRequest, destination string) error {
	edgePath, err := findEdge()
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "mory-export-")
	if err != nil {
		return fmt.Errorf("创建导出临时目录：%w", err)
	}
	defer os.RemoveAll(temporary)
	htmlPath := filepath.Join(temporary, "document.html")
	if err := writeExportFile(htmlPath, []byte(request.HTML)); err != nil {
		return err
	}
	port, err := availablePort()
	if err != nil {
		return err
	}
	process := exec.CommandContext(ctx, edgePath,
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--allow-file-access-from-files",
		"--remote-debugging-port="+strconv.Itoa(port),
		"--user-data-dir="+filepath.Join(temporary, "profile"),
		"about:blank",
	)
	process.Stdout = nil
	process.Stderr = nil
	if err := process.Start(); err != nil {
		return fmt.Errorf("启动 Edge 导出进程：%w", err)
	}
	defer func() {
		_ = process.Process.Kill()
		_, _ = process.Process.Wait()
	}()

	debugURL, err := waitForDebugPage(ctx, port)
	if err != nil {
		return err
	}
	connection, _, err := websocket.DefaultDialer.DialContext(ctx, debugURL, nil)
	if err != nil {
		return fmt.Errorf("连接 Edge 导出页面：%w", err)
	}
	defer connection.Close()
	client := &cdpClient{connection: connection}
	if _, err := client.call(ctx, "Page.enable", nil); err != nil {
		return err
	}
	fileURL := fileURL(htmlPath)
	if _, err := client.call(ctx, "Page.navigate", map[string]any{"url": fileURL}); err != nil {
		return err
	}
	if err := client.waitEvent(ctx, "Page.loadEventFired"); err != nil {
		return err
	}
	// 等待字体与所有 data URL 图片解码，避免首次导出缺图或字体跳变。
	readyExpression := `(async()=>{await document.fonts.ready;await Promise.all([...document.images].map(img=>img.complete?Promise.resolve():new Promise(resolve=>{img.addEventListener('load',resolve,{once:true});img.addEventListener('error',resolve,{once:true})})));return Math.max(document.documentElement.scrollHeight,document.body.scrollHeight)})()`
	ready, err := client.call(ctx, "Runtime.evaluate", map[string]any{"expression": readyExpression, "awaitPromise": true, "returnByValue": true})
	if err != nil {
		return err
	}
	height := runtimeNumber(ready)
	if height < 300 {
		height = 300
	}

	var encoded string
	switch request.Format {
	case "pdf":
		paperWidth, paperHeight := paperSize(request.Paper)
		result, callErr := client.call(ctx, "Page.printToPDF", map[string]any{
			"printBackground":   request.Background,
			"paperWidth":        paperWidth,
			"paperHeight":       paperHeight,
			"marginTop":         0.45,
			"marginBottom":      0.45,
			"marginLeft":        0.5,
			"marginRight":       0.5,
			"preferCSSPageSize": false,
		})
		if callErr != nil {
			return callErr
		}
		encoded = stringField(result, "data")
	case "png", "jpeg":
		if height > 28000 {
			return errors.New("文档超过 28000 像素，请降低图片宽度或改用 PDF 导出")
		}
		width := request.Width
		if width < 480 {
			width = 480
		}
		if width > 2400 {
			width = 2400
		}
		if _, err := client.call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
			"width": width, "height": int(height), "deviceScaleFactor": 1, "mobile": false,
		}); err != nil {
			return err
		}
		parameters := map[string]any{"format": request.Format, "fromSurface": true, "captureBeyondViewport": true}
		if request.Format == "jpeg" {
			parameters["quality"] = 92
		}
		result, callErr := client.call(ctx, "Page.captureScreenshot", parameters)
		if callErr != nil {
			return callErr
		}
		encoded = stringField(result, "data")
	default:
		return fmt.Errorf("不支持的导出格式：%s", request.Format)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 {
		return errors.New("Edge 返回的导出数据无效")
	}
	return writeExportFile(destination, data)
}

func (client *cdpClient) call(ctx context.Context, method string, params any) (map[string]any, error) {
	client.nextID++
	id := client.nextID
	message := map[string]any{"id": id, "method": method}
	if params != nil {
		message["params"] = params
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = client.connection.SetWriteDeadline(deadline)
	}
	if err := client.connection.WriteJSON(message); err != nil {
		return nil, fmt.Errorf("发送 Edge 命令 %s：%w", method, err)
	}
	for {
		response, err := client.read(ctx)
		if err != nil {
			return nil, err
		}
		if response.ID != id {
			if response.Method != "" {
				client.events = append(client.events, response)
			}
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("Edge 命令 %s 失败（%d）：%s", method, response.Error.Code, response.Error.Message)
		}
		var result map[string]any
		if len(response.Result) > 0 {
			if err := json.Unmarshal(response.Result, &result); err != nil {
				return nil, fmt.Errorf("解析 Edge 命令 %s：%w", method, err)
			}
		}
		return result, nil
	}
}

func (client *cdpClient) waitEvent(ctx context.Context, method string) error {
	for index, message := range client.events {
		if message.Method == method {
			client.events = append(client.events[:index], client.events[index+1:]...)
			return nil
		}
	}
	for {
		message, err := client.read(ctx)
		if err != nil {
			return err
		}
		if message.Method == method {
			return nil
		}
	}
}

func (client *cdpClient) read(ctx context.Context) (cdpMessage, error) {
	if deadline, ok := ctx.Deadline(); ok {
		_ = client.connection.SetReadDeadline(deadline)
	} else {
		_ = client.connection.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
	var message cdpMessage
	if err := client.connection.ReadJSON(&message); err != nil {
		return cdpMessage{}, fmt.Errorf("读取 Edge 导出响应：%w", err)
	}
	return message, nil
}

func waitForDebugPage(ctx context.Context, port int) (string, error) {
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		response, err := client.Do(request)
		if err == nil {
			var pages []struct {
				Type                 string `json:"type"`
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&pages)
			_ = response.Body.Close()
			if decodeErr == nil {
				for _, page := range pages {
					if page.Type == "page" && page.WebSocketDebuggerURL != "" {
						return page.WebSocketDebuggerURL, nil
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(80 * time.Millisecond):
		}
	}
	return "", errors.New("等待 Edge 导出进程超时")
}

func findEdge() (string, error) {
	if path, err := exec.LookPath("msedge.exe"); err == nil {
		return path, nil
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("未找到 Microsoft Edge，PDF 与图片导出需要系统 Edge")
}

func availablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("分配导出调试端口：%w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func fileURL(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func paperSize(name string) (float64, float64) {
	switch strings.ToUpper(name) {
	case "LETTER":
		return 8.5, 11
	case "LEGAL":
		return 8.5, 14
	case "A3":
		return 11.69, 16.54
	default:
		return 8.27, 11.69
	}
}

func runtimeNumber(result map[string]any) float64 {
	remote, _ := result["result"].(map[string]any)
	value, _ := remote["value"].(float64)
	return value
}

func stringField(result map[string]any, key string) string {
	value, _ := result[key].(string)
	return value
}

func writeExportFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建导出目录：%w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入导出文件：%w", err)
	}
	return nil
}
