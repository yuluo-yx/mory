package blocks

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
)

type Cell struct {
	Row, Column, Rows, Columns int
	Node                       *Node
}
type Rect struct{ Top, Left, Bottom, Right int }
type Table struct {
	Rows, Columns int
	Widths        []float64
	Cells         []Cell
	Compact       *CompactTable
}

func ReadTable(node *Node) *Table {
	if node.Compact != nil {
		return &Table{
			Rows:    node.Compact.Rows,
			Columns: node.Compact.Columns,
			Widths:  slices.Clone(node.Compact.Widths),
			Compact: node.Compact,
		}
	}
	t := &Table{}
	var rows []*Node
	var walk func(*Node)
	walk = func(n *Node) {
		if n != node && n.Kind == "table" {
			return
		}
		if n.Kind == "tr" {
			rows = append(rows, n)
			return
		}
		if n.Kind == "col" {
			t.Widths = append(t.Widths, n.Float("width", 160))
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(node)
	occupied := map[[2]int]bool{}
	for r, row := range rows {
		c := 0
		for _, n := range row.Elements() {
			if n.Kind != "td" && n.Kind != "th" {
				continue
			}
			for occupied[[2]int{r, c}] {
				c++
			}
			rs, cs := max(1, n.Int("rowspan", 1)), max(1, n.Int("colspan", 1))
			rs = min(rs, 1000)
			cs = min(cs, 100)
			cell := Cell{Row: r, Column: c, Rows: rs, Columns: cs, Node: n}
			t.Cells = append(t.Cells, cell)
			for rr := r; rr < r+rs; rr++ {
				for cc := c; cc < c+cs; cc++ {
					occupied[[2]int{rr, cc}] = true
				}
			}
			t.Rows = max(t.Rows, r+rs)
			t.Columns = max(t.Columns, c+cs)
			c += cs
		}
	}
	t.Rows = max(1, max(t.Rows, len(rows)))
	t.Columns = max(1, max(t.Columns, len(t.Widths)))
	for len(t.Widths) < t.Columns {
		t.Widths = append(t.Widths, 160)
	}
	t.fill()
	return t
}

func (t *Table) fill() {
	occupied := map[[2]int]bool{}
	for _, cell := range t.Cells {
		for r := cell.Row; r < cell.Row+cell.Rows; r++ {
			for c := cell.Column; c < cell.Column+cell.Columns; c++ {
				occupied[[2]int{r, c}] = true
			}
		}
	}
	for r := 0; r < t.Rows; r++ {
		for c := 0; c < t.Columns; c++ {
			if !occupied[[2]int{r, c}] {
				t.Cells = append(t.Cells, Cell{Row: r, Column: c, Rows: 1, Columns: 1, Node: New("td", Paragraph(""))})
			}
		}
	}
	t.sort()
}
func (t *Table) sort() {
	sort.Slice(t.Cells, func(i, j int) bool {
		if t.Cells[i].Row != t.Cells[j].Row {
			return t.Cells[i].Row < t.Cells[j].Row
		}
		return t.Cells[i].Column < t.Cells[j].Column
	})
}
func (t *Table) At(row, column int) *Cell {
	for i := range t.Cells {
		c := &t.Cells[i]
		if row >= c.Row && row < c.Row+c.Rows && column >= c.Column && column < c.Column+c.Columns {
			return c
		}
	}
	return nil
}
func (t *Table) Expand(r Rect) Rect {
	r.Top = max(0, min(r.Top, t.Rows-1))
	r.Bottom = max(r.Top, min(r.Bottom, t.Rows-1))
	r.Left = max(0, min(r.Left, t.Columns-1))
	r.Right = max(r.Left, min(r.Right, t.Columns-1))
	for {
		old := r
		for _, c := range t.Cells {
			if c.Row <= r.Bottom && c.Row+c.Rows > r.Top && c.Column <= r.Right && c.Column+c.Columns > r.Left {
				r.Top = min(r.Top, c.Row)
				r.Bottom = max(r.Bottom, c.Row+c.Rows-1)
				r.Left = min(r.Left, c.Column)
				r.Right = max(r.Right, c.Column+c.Columns-1)
			}
		}
		if old == r {
			return r
		}
	}
}

func (t *Table) Merge(selection Rect) error {
	if selection.Top < 0 || selection.Left < 0 || selection.Bottom >= t.Rows || selection.Right >= t.Columns || selection.Bottom < selection.Top || selection.Right < selection.Left {
		return fmt.Errorf("invalid cell selection")
	}
	r := t.Expand(selection)
	origin := t.At(r.Top, r.Left)
	if origin == nil {
		return fmt.Errorf("missing merge origin")
	}
	merged := origin.Node.Clone()
	merged.Children = nil
	var keep []Cell
	for _, cell := range t.Cells {
		if cell.Row >= r.Top && cell.Row <= r.Bottom && cell.Column >= r.Left && cell.Column <= r.Right {
			for _, child := range Visible(cell.Node.Children) {
				if child.Kind != "p" || child.Plain() != "" || len(child.Elements()) > 0 {
					merged.Children = append(merged.Children, child.Clone())
				}
			}
		} else {
			keep = append(keep, cell)
		}
	}
	if len(merged.Children) == 0 {
		merged.Children = []*Node{Paragraph("")}
	}
	t.Cells = append(keep, Cell{Row: r.Top, Column: r.Left, Rows: r.Bottom - r.Top + 1, Columns: r.Right - r.Left + 1, Node: merged})
	t.sort()
	return nil
}
func (t *Table) Split(row, column int) error {
	cell := t.At(row, column)
	if cell == nil {
		return fmt.Errorf("cell outside table")
	}
	cell.Rows = 1
	cell.Columns = 1
	t.fill()
	return nil
}
func (t *Table) InsertRow(at int) error {
	if at < 0 || at > t.Rows || t.Rows >= 1000 {
		return fmt.Errorf("invalid row insertion")
	}
	for i := range t.Cells {
		c := &t.Cells[i]
		if c.Row >= at {
			c.Row++
		} else if c.Row+c.Rows > at {
			c.Rows++
		}
	}
	t.Rows++
	t.fill()
	return nil
}
func (t *Table) InsertColumn(at int) error {
	if at < 0 || at > t.Columns || t.Columns >= 100 {
		return fmt.Errorf("invalid column insertion")
	}
	for i := range t.Cells {
		c := &t.Cells[i]
		if c.Column >= at {
			c.Column++
		} else if c.Column+c.Columns > at {
			c.Columns++
		}
	}
	t.Widths = slices.Insert(t.Widths, at, 160)
	t.Columns++
	t.fill()
	return nil
}
func collapse(x, start, end int) int {
	if x <= start {
		return x
	}
	if x >= end {
		return x - (end - start)
	}
	return start
}
func (t *Table) DeleteRows(start, end int) error {
	if start < 0 || start >= end || end > t.Rows {
		return fmt.Errorf("invalid row deletion")
	}
	keep := t.Cells[:0]
	for _, c := range t.Cells {
		top, bottom := collapse(c.Row, start, end), collapse(c.Row+c.Rows, start, end)
		if bottom > top {
			c.Row = top
			c.Rows = bottom - top
			keep = append(keep, c)
		}
	}
	t.Cells = keep
	t.Rows = max(1, t.Rows-(end-start))
	t.fill()
	return nil
}
func (t *Table) DeleteColumns(start, end int) error {
	if start < 0 || start >= end || end > t.Columns {
		return fmt.Errorf("invalid column deletion")
	}
	keep := t.Cells[:0]
	for _, c := range t.Cells {
		left, right := collapse(c.Column, start, end), collapse(c.Column+c.Columns, start, end)
		if right > left {
			c.Column = left
			c.Columns = right - left
			keep = append(keep, c)
		}
	}
	t.Cells = keep
	t.Columns = max(1, t.Columns-(end-start))
	t.Widths = slices.Delete(t.Widths, start, end)
	if len(t.Widths) == 0 {
		t.Widths = []float64{160}
	}
	t.fill()
	return nil
}

func permutation(count, start, end, to int) ([]int, error) {
	if start < 0 || end <= start || end > count || to < 0 || to > count || to > start && to < end {
		return nil, fmt.Errorf("invalid move range")
	}
	order := []int{}
	for i := 0; i < count; i++ {
		if i < start || i >= end {
			order = append(order, i)
		}
	}
	index := 0
	for _, i := range order {
		if i < to {
			index++
		}
	}
	moved := []int{}
	for i := start; i < end; i++ {
		moved = append(moved, i)
	}
	return slices.Insert(order, index, moved...), nil
}
func (t *Table) move(start, end, to int, rows bool) error {
	count := t.Columns
	if rows {
		count = t.Rows
	}
	order, err := permutation(count, start, end, to)
	if err != nil {
		return err
	}
	inverse := make([]int, count)
	for i, old := range order {
		inverse[old] = i
	}
	changed := slices.Clone(t.Cells)
	for i := range changed {
		c := &changed[i]
		origin, span := c.Column, c.Columns
		if rows {
			origin, span = c.Row, c.Rows
		}
		lo, hi := count, 0
		for old := origin; old < origin+span; old++ {
			lo = min(lo, inverse[old])
			hi = max(hi, inverse[old])
		}
		if hi-lo+1 != span {
			return fmt.Errorf("split the merged cells crossing this move first")
		}
		if rows {
			c.Row = lo
		} else {
			c.Column = lo
		}
	}
	t.Cells = changed
	if !rows {
		widths := make([]float64, count)
		for i, old := range order {
			widths[i] = t.Widths[old]
		}
		t.Widths = widths
	}
	t.sort()
	return nil
}
func (t *Table) MoveRows(start, end, to int) error    { return t.move(start, end, to, true) }
func (t *Table) MoveColumns(start, end, to int) error { return t.move(start, end, to, false) }
func (t *Table) Resize(column int, width float64) error {
	if column < 0 || column >= t.Columns {
		return fmt.Errorf("column outside table")
	}
	t.Widths[column] = max(40, min(width, 1200))
	return nil
}

// Write updates the table tree while retaining cell IDs, rich children, unknown
// cell attributes, and other table-level elements such as captions.
func (t *Table) Write(node *Node) {
	var extras []*Node
	for _, child := range node.Children {
		switch child.Kind {
		case "colgroup", "thead", "tbody", "tfoot", "tr", "#text":
		default:
			extras = append(extras, child)
		}
	}
	cols := New("colgroup")
	for _, width := range t.Widths {
		col := New("col")
		col.Set("width", strconv.FormatFloat(width, 'f', -1, 64))
		cols.Children = append(cols.Children, col)
	}
	body := New("tbody")
	for r := 0; r < t.Rows; r++ {
		row := New("tr")
		for _, cell := range t.Cells {
			if cell.Row != r {
				continue
			}
			n := cell.Node.Clone()
			if cell.Rows > 1 {
				n.Set("rowspan", strconv.Itoa(cell.Rows))
			} else {
				n.Set("rowspan", "")
			}
			if cell.Columns > 1 {
				n.Set("colspan", strconv.Itoa(cell.Columns))
			} else {
				n.Set("colspan", "")
			}
			row.Children = append(row.Children, n)
		}
		body.Children = append(body.Children, row)
	}
	node.Children = append([]*Node{cols, body}, extras...)
}
