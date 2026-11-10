package blocks

import (
	"regexp"
	"strings"
)

var headingID = regexp.MustCompile(`\s+\{#([^{}\s]+)\}\s*$`)
var staticAttribute = regexp.MustCompile(`([A-Za-z][A-Za-z0-9]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
var defaultAttribute = regexp.MustCompile(`\sdefault(?:\s|>)`)

// parseDocusaurus splits only recognized block extensions outside code fences.
// Source spelling is attached to each node; editing a child retains its wrapper.
func parseDocusaurus(source string, frontmatter bool) (*Node, error) {
	root := New("root")
	flush := func(s string) error {
		if s == "" {
			return nil
		}
		if strings.TrimSpace(s) == "" {
			n := New("#raw")
			n.Text = s
			root.Children = append(root.Children, n)
			return nil
		}
		n, err := parseCommonMarkdown(s)
		if err == nil {
			root.Children = append(root.Children, n.Children...)
		}
		return err
	}
	start, offset := 0, 0
	var fence string
	for offset < len(source) {
		end := lineEnd(source, offset)
		line := strings.TrimSpace(source[offset:end])
		if fence != "" {
			if strings.HasPrefix(line, fence) && strings.Trim(line, string(fence[0])) == "" {
				fence = ""
			}
			offset = end
			continue
		}
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fence = line[:len(line)-len(strings.TrimLeft(line, string(line[0])))]
			offset = end
			continue
		}
		n, stop := extensionBlock(source, offset, end, frontmatter)
		if n == nil {
			offset = end
			continue
		}
		if err := flush(source[start:offset]); err != nil {
			return nil, err
		}
		n.markdown = source[offset:stop]
		n.mdBaseline = n.signature()
		root.Children = append(root.Children, n)
		offset, start = stop, stop
	}
	if err := flush(source[start:]); err != nil {
		return nil, err
	}
	return root, nil
}

func lineEnd(s string, offset int) int {
	if i := strings.IndexByte(s[offset:], '\n'); i >= 0 {
		return offset + i + 1
	}
	return len(s)
}

func extensionBlock(s string, start, end int, frontmatter bool) (*Node, int) {
	line := strings.TrimSpace(s[start:end])
	if frontmatter && start == 0 && strings.TrimPrefix(line, "\ufeff") == "---" {
		for p := end; p < len(s); {
			e := lineEnd(s, p)
			if l := strings.TrimSpace(s[p:e]); l == "---" || l == "..." {
				n := New("#raw")
				n.Text = s[start:e]
				return n, e
			}
			p = e
		}
	}
	if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "export ") {
		// ESM remains source-only. Its execution belongs to the site's MDX build.
		p := end
		for p < len(s) && strings.TrimSpace(s[p:lineEnd(s, p)]) != "" && !strings.HasSuffix(strings.TrimSpace(s[start:p]), ";") {
			p = lineEnd(s, p)
		}
		n := New("mdx-source", Text(s[start:p]))
		return n, p
	}
	if strings.HasPrefix(line, ":::") && strings.Trim(line, ":") != "" {
		marker := line[:len(line)-len(strings.TrimLeft(line, ":"))]
		info := strings.TrimSpace(strings.TrimPrefix(line, marker))
		kind, title, _ := strings.Cut(info, " ")
		if i := strings.IndexByte(kind, '['); i >= 0 {
			title = strings.TrimSuffix(info[i+1:], "]")
			kind = kind[:i]
		}
		switch kind {
		case "note", "tip", "info", "warning", "danger", "caution":
		default:
			return nil, start
		}
		level := 1
		var codeFence string
		for p := end; p < len(s); {
			e := lineEnd(s, p)
			l := strings.TrimSpace(s[p:e])
			if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
				if codeFence == "" {
					codeFence = l[:3]
				} else if strings.HasPrefix(l, codeFence) {
					codeFence = ""
				}
			} else if codeFence == "" && strings.HasPrefix(l, marker) {
				if strings.Trim(l, ":") == "" {
					level--
				} else {
					level++
				}
				if level == 0 {
					children, err := parseDocusaurus(s[end:p], false)
					if err != nil {
						return nil, start
					}
					n := New("callout", children.Children...)
					n.Set("directive", kind)
					n.Set("title", title)
					n.Set("opening", s[start:end])
					n.Set("closing", s[p:e])
					return n, e
				}
			}
			p = e
		}
	}
	for _, tag := range []string{"details", "Tabs", "TabItem"} {
		prefix := "<" + tag
		if !strings.HasPrefix(line, prefix+">") && !strings.HasPrefix(line, prefix+" ") && line != prefix {
			continue
		}
		openEnd := tagEnd(s, start)
		if openEnd < 0 {
			continue
		}
		closeStart, closeEnd := matchingTag(s, openEnd, tag)
		if closeStart < 0 {
			continue
		}
		n := New(map[string]string{"details": "details", "Tabs": "tabs", "TabItem": "tab-item"}[tag])
		opening := s[start:openEnd]
		for _, m := range staticAttribute.FindAllStringSubmatch(opening, -1) {
			n.Set(m[1], m[2]+m[3])
		}
		if defaultAttribute.MatchString(opening) {
			n.Set("default", "true")
		}
		bodyStart := openEnd
		if tag == "details" {
			rest := s[openEnd:closeStart]
			if i := strings.Index(rest, "<summary>"); i >= 0 {
				if j := strings.Index(rest[i:], "</summary>"); j >= 0 {
					n.Set("summary", rest[i+9:i+j])
					bodyStart = openEnd + i + j + 10
				}
			}
		}
		children, err := parseDocusaurus(s[bodyStart:closeStart], false)
		if err != nil {
			return nil, start
		}
		n.Children = children.Children
		n.Set("opening", s[start:bodyStart])
		n.Set("closing", s[closeStart:closeEnd])
		return n, closeEnd
	}
	return nil, start
}

// tagEnd respects quoted attributes and JSX property braces without evaluating JS.
func tagEnd(s string, start int) int {
	var quote byte
	braces := 0
	for i := start; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == '\\' {
				i++
			} else if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '{':
			braces++
		case '}':
			braces--
		case '>':
			if braces == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func matchingTag(s string, start int, tag string) (int, int) {
	depth := 1
	var fence string
	for i := start; i < len(s); i++ {
		if i == start || s[i-1] == '\n' {
			end := lineEnd(s, i)
			line := strings.TrimSpace(s[i:end])
			if fence != "" {
				if strings.HasPrefix(line, fence) && strings.Trim(line, string(fence[0])) == "" {
					fence = ""
				}
				i = end - 1
				continue
			}
			if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
				fence = line[:len(line)-len(strings.TrimLeft(line, string(line[0])))]
				i = end - 1
				continue
			}
		}
		if s[i] != '<' {
			continue
		}
		closing := strings.HasPrefix(s[i:], "</"+tag+">")
		opening := strings.HasPrefix(s[i:], "<"+tag+">") || strings.HasPrefix(s[i:], "<"+tag+" ") || strings.HasPrefix(s[i:], "<"+tag+"\n")
		if !closing && !opening {
			continue
		}
		e := tagEnd(s, i)
		if e < 0 {
			break
		}
		if closing {
			depth--
		} else {
			depth++
		}
		if depth == 0 {
			return i, e
		}
		i = e - 1
	}
	return -1, -1
}

func extensionMarkdown(n *Node) (string, bool) {
	if n.Kind == "mdx-source" {
		return n.Plain(), true
	}
	if n.Attr("opening") != "" && (n.Kind == "details" || n.Kind == "tabs" || n.Kind == "tab-item" || n.Kind == "callout" && n.Attr("directive") != "") {
		root := New("root", n.Children...)
		body := root.Markdown()
		if body != "" && !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return n.Attr("opening") + body + n.Attr("closing"), true
	}
	return "", false
}
