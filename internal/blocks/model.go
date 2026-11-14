// Package blocks is the structured, lossless document model shared by native
// editors and exporters. Its schema follows the DocxXML block vocabulary.
package blocks

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Node keeps unknown attributes and children, as well as original XML bytes.
// IDs identify editing targets and are not injected into documents on disk.
type Node struct {
	ID         string
	Kind       string
	Attrs      map[string]string
	Text       string
	Children   []*Node
	Compact    *CompactTable
	raw        string
	baseline   [32]byte
	markdown   string
	mdBaseline [32]byte
}

// CompactTable keeps the source and the small amount of geometry needed for a
// large Markdown table preview without allocating a node tree for every cell.
// The full table remains available in Source mode and can be parsed on demand
// if a future editing path needs it.
type CompactTable struct {
	Rows, Columns int
	Widths        []float64
	Preview       map[[2]int]string
}

func New(kind string, children ...*Node) *Node {
	return &Node{ID: rand.Text(), Kind: kind, Children: children}
}
func Text(s string) *Node      { n := New("#text"); n.Text = s; return n }
func Paragraph(s string) *Node { return New("p", Text(s)) }
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	out := *n
	out.Attrs = maps.Clone(n.Attrs)
	out.Children = make([]*Node, len(n.Children))
	for i, c := range n.Children {
		out.Children[i] = c.Clone()
	}
	return &out
}
func (n *Node) Attr(key string) string {
	if n == nil {
		return ""
	}
	return n.Attrs[key]
}
func (n *Node) Set(key, value string) {
	if value == "" {
		delete(n.Attrs, key)
	} else {
		// Most text and structural nodes have no attributes. Allocate only when
		// adding one, including when a cloned node first acquires formatting.
		if n.Attrs == nil {
			n.Attrs = make(map[string]string)
		}
		n.Attrs[key] = value
	}
}
func (n *Node) Int(key string, fallback int) int {
	v, err := strconv.Atoi(n.Attr(key))
	if err != nil {
		return fallback
	}
	return v
}
func (n *Node) Float(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(n.Attr(key), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return fallback
	}
	return v
}
func (n *Node) Plain() string {
	if n.Kind == "#text" {
		return n.Text
	}
	if n.Kind == "br" {
		return "\n"
	}
	var b strings.Builder
	for _, c := range n.Children {
		b.WriteString(c.Plain())
	}
	return b.String()
}
func (n *Node) Find(id string) *Node {
	if n.ID == id {
		return n
	}
	for _, c := range n.Children {
		if found := c.Find(id); found != nil {
			return found
		}
	}
	return nil
}
func (n *Node) Elements() []*Node {
	var out []*Node
	for _, c := range n.Children {
		if !strings.HasPrefix(c.Kind, "#") {
			out = append(out, c)
		}
	}
	return out
}
func (n *Node) Walk(fn func(*Node)) {
	fn(n)
	for _, c := range n.Children {
		c.Walk(fn)
	}
}

func (n *Node) signature() [32]byte {
	h := sha256.New()
	// Keep the digest and output buffer on the stack. Formatting through io.Writer
	// made every node allocate during lossless source validation on each edit.
	h.Write([]byte(n.Kind))
	h.Write([]byte{0})
	h.Write([]byte(n.Text))
	h.Write([]byte{0})
	var localKeys [8]string
	keys := localKeys[:0]
	if len(n.Attrs) > len(localKeys) {
		keys = make([]string, 0, len(n.Attrs))
	}
	for k := range n.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(n.Attrs[k]))
		h.Write([]byte{0})
	}
	for _, c := range n.Children {
		s := c.signature()
		h.Write(s[:])
	}
	var sum [32]byte
	h.Sum(sum[:0])
	return sum
}

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// XML preserves an untouched subtree byte-for-byte, including its whitespace.
func (n *Node) XML() string {
	if n.raw != "" && n.signature() == n.baseline {
		return n.raw
	}
	if n.Kind == "#text" {
		return escape(n.Text)
	}
	if n.Kind == "#raw" {
		return n.Text
	}
	var b strings.Builder
	if n.Kind != "root" && n.Kind != "markdown-group" {
		b.WriteByte('<')
		b.WriteString(n.Kind)
		keys := make([]string, 0, len(n.Attrs))
		for k := range n.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(" " + k + `="` + escape(n.Attrs[k]) + `"`)
		}
		if len(n.Children) == 0 {
			b.WriteString("/>")
			return b.String()
		}
		b.WriteByte('>')
	}
	for _, c := range n.Children {
		b.WriteString(c.XML())
	}
	if n.Kind != "root" && n.Kind != "markdown-group" {
		b.WriteString("</" + n.Kind + ">")
	}
	return b.String()
}

// ParseXML accepts DocxXML fragments with any number of top-level elements.
func ParseXML(source string) (*Node, error) {
	const opening = "<mory-document>"
	wrapped := opening + source + "</mory-document>"
	decoder := xml.NewDecoder(strings.NewReader(wrapped))
	root := New("root")
	stack := []*Node{root}
	starts := []int{0}
	depth := 0
	for {
		before := int(decoder.InputOffset())
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse DocxXML: %w", err)
		}
		after := int(decoder.InputOffset())
		switch token := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				continue
			}
			if depth > 256 {
				return nil, fmt.Errorf("document nesting exceeds 256 levels")
			}
			n := New(token.Name.Local)
			for _, attr := range token.Attr {
				name := attr.Name.Local
				if attr.Name.Space != "" {
					name = attr.Name.Space + ":" + name
				}
				// Empty XML attributes must also survive a lossless round trip.
				if n.Attrs == nil {
					n.Attrs = make(map[string]string)
				}
				n.Attrs[name] = attr.Value
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
			starts = append(starts, before)
		case xml.EndElement:
			if depth == 1 {
				depth--
				continue
			}
			n := stack[len(stack)-1]
			n.raw = wrapped[starts[len(starts)-1]:after]
			n.baseline = n.signature()
			stack = stack[:len(stack)-1]
			starts = starts[:len(starts)-1]
			depth--
		case xml.CharData:
			if depth == 0 {
				continue
			}
			n := Text(string(token))
			n.raw = wrapped[before:after]
			n.baseline = n.signature()
			stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, n)
		case xml.Comment, xml.ProcInst, xml.Directive:
			n := New("#raw")
			n.Text = wrapped[before:after]
			n.raw = n.Text
			n.baseline = n.signature()
			stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, n)
		}
	}
	root.raw = source
	root.baseline = root.signature()
	return root, nil
}

type edit struct {
	Parent            string
	At                int
	Removed, Inserted []*Node
}
type transaction struct{ Edits []edit }

// Document keeps undo as affected subtrees, so editing one table cell does not
// copy the remainder of a large document.
type Document struct {
	Root       *Node
	Format     string
	original   string
	baseline   [32]byte
	Version    uint64
	undo, redo []transaction
	batch      *transaction
}

func Parse(source, format string) (*Document, error) {
	var root *Node
	var err error
	if format == "xml" {
		root, err = ParseXML(source)
	} else {
		root, err = ParseMarkdown(source)
	}
	if err != nil {
		return nil, err
	}
	return &Document{Root: root, Format: format, original: source, baseline: root.signature(), Version: 1}, nil
}
func (d *Document) Source() string {
	if d.Root.signature() == d.baseline {
		return d.original
	}
	if d.Format == "xml" {
		return d.Root.XML()
	}
	return d.Root.Markdown()
}
func (d *Document) Find(id string) *Node { return d.Root.Find(id) }
func (d *Document) Parent(id string) (*Node, int) {
	var parent *Node
	index := -1
	d.Root.Walk(func(n *Node) {
		for i, c := range n.Children {
			if c.ID == id {
				parent = n
				index = i
			}
		}
	})
	return parent, index
}
func cloneNodes(nodes []*Node) []*Node {
	out := make([]*Node, len(nodes))
	for i, n := range nodes {
		out[i] = n.Clone()
	}
	return out
}

func (d *Document) Splice(parentID string, at, count int, nodes ...*Node) error {
	p := d.Find(parentID)
	if p == nil || at < 0 || count < 0 || at+count > len(p.Children) {
		return fmt.Errorf("invalid block range")
	}
	e := edit{Parent: parentID, At: at, Removed: cloneNodes(p.Children[at : at+count]), Inserted: cloneNodes(nodes)}
	p.Children = slices.Replace(p.Children, at, at+count, nodes...)
	d.Version++
	d.redo = nil
	if d.batch != nil {
		d.batch.Edits = append(d.batch.Edits, e)
	} else {
		d.undo = append(d.undo, transaction{Edits: []edit{e}})
	}
	return nil
}
func (d *Document) Transform(id string, fn func(*Node)) error {
	n := d.Find(id)
	if n == nil {
		return fmt.Errorf("block no longer exists")
	}
	parent, at := d.Parent(id)
	if parent == nil {
		return fmt.Errorf("cannot replace the document root")
	}
	copy := n.Clone()
	fn(copy)
	if copy.signature() == n.signature() {
		return nil
	}
	return d.Splice(parent.ID, at, 1, copy)
}
func (d *Document) Transaction(fn func() error) error {
	if d.batch != nil {
		return fn()
	}
	batch := &transaction{}
	d.batch = batch
	err := fn()
	d.batch = nil
	if err != nil {
		d.apply(*batch, true)
		return err
	}
	if len(batch.Edits) > 0 {
		d.undo = append(d.undo, *batch)
	}
	return nil
}
func (d *Document) apply(t transaction, reverse bool) {
	edits := slices.Clone(t.Edits)
	if reverse {
		slices.Reverse(edits)
	}
	for _, e := range edits {
		p := d.Find(e.Parent)
		if p == nil {
			continue
		}
		removed, inserted := e.Removed, e.Inserted
		if reverse {
			removed, inserted = inserted, removed
		}
		p.Children = slices.Replace(p.Children, e.At, e.At+len(removed), cloneNodes(inserted)...)
	}
	d.Version++
}
func (d *Document) Undo() bool {
	if len(d.undo) == 0 {
		return false
	}
	t := d.undo[len(d.undo)-1]
	d.undo = d.undo[:len(d.undo)-1]
	d.apply(t, true)
	d.redo = append(d.redo, t)
	return true
}

// Coalesce joins consecutive typing edits into a single undo step. Structural
// commands and cursor moves end a group in the editor before calling this.
func (d *Document) Coalesce() {
	if len(d.undo) < 2 {
		return
	}
	i := len(d.undo) - 2
	d.undo[i].Edits = append(d.undo[i].Edits, d.undo[i+1].Edits...)
	d.undo = d.undo[:i+1]
}
func (d *Document) Redo() bool {
	if len(d.redo) == 0 {
		return false
	}
	t := d.redo[len(d.redo)-1]
	d.redo = d.redo[:len(d.redo)-1]
	d.apply(t, false)
	d.undo = append(d.undo, t)
	return true
}
func (d *Document) Remove(id string) error {
	p, at := d.Parent(id)
	if p == nil {
		return fmt.Errorf("block no longer exists")
	}
	return d.Splice(p.ID, at, 1)
}
func (d *Document) Move(id, parentID string, at int) error {
	n := d.Find(id)
	dest := d.Find(parentID)
	if n == nil || dest == nil || n.Find(parentID) != nil {
		return fmt.Errorf("invalid block destination")
	}
	p, from := d.Parent(id)
	if p == nil {
		return fmt.Errorf("cannot move the document root")
	}
	if p.ID == parentID && from < at {
		at--
	}
	copy := n.Clone()
	return d.Transaction(func() error {
		if err := d.Remove(id); err != nil {
			return err
		}
		return d.Splice(parentID, at, 0, copy)
	})
}

func IsText(n *Node) bool {
	return n.Kind == "p" || n.Kind == "li" || n.Kind == "checkbox" || n.Kind == "title" || len(n.Kind) == 2 && n.Kind[0] == 'h' && n.Kind[1] >= '1' && n.Kind[1] <= '9'
}

// IsBlock distinguishes nested blocks from opaque inline objects inside text.
func IsBlock(n *Node) bool {
	if IsText(n) {
		return true
	}
	switch n.Kind {
	case "ul", "ol", "pre", "blockquote", "callout", "grid", "column", "table", "hr", "countdown", "figure", "sub-page-list", "raw-html", "footnote":
		return true
	}
	return false
}
func Visible(nodes []*Node) []*Node {
	var out []*Node
	for _, n := range nodes {
		if n.Kind == "#text" && strings.TrimSpace(n.Text) == "" || n.Kind == "#raw" {
			continue
		}
		out = append(out, n)
	}
	return out
}

// FreshIDs makes a duplicated subtree independent from its source.
func FreshIDs(n *Node) { n.Walk(func(n *Node) { n.ID = rand.Text() }) }
