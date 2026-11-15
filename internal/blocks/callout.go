package blocks

import "strings"

// CalloutType resolves both GitHub alerts and Docusaurus admonitions without
// rewriting their source spelling. Older untyped XML callouts behave as notes.
func CalloutType(n *Node) string {
	kind := n.Attr("directive")
	if kind == "" {
		kind = n.Attr("alert")
	}
	return normalizeCalloutType(kind)
}

func normalizeCalloutType(kind string) string {
	switch kind = strings.ToLower(strings.TrimSpace(kind)); kind {
	case "warn":
		return "warning"
	case "note", "info", "tip", "important", "warning", "caution", "danger":
		return kind
	default:
		return "note"
	}
}

// CalloutColor supplies the same semantic accent to the native view and export.
func CalloutColor(n *Node) string {
	switch CalloutType(n) {
	case "info":
		return "#3088bd"
	case "tip":
		return "#2e8555"
	case "important":
		return "#8250c4"
	case "warning", "caution":
		return "#b78000"
	case "danger":
		return "#d12d3f"
	default:
		return "#708090"
	}
}

// SetCalloutType keeps the existing syntax family whenever it can represent
// the chosen type. GitHub has no INFO or DANGER alert, so those use directives.
func SetCalloutType(n *Node, kind string) {
	kind = normalizeCalloutType(kind)
	if n.Kind != "callout" || CalloutType(n) == kind {
		return
	}
	if old := n.Attr("directive"); old != "" {
		n.Set("directive", kind)
		if opening := n.Attr("opening"); opening != "" {
			n.Set("opening", strings.Replace(opening, old, kind, 1))
		}
		n.Set("alert", "")
	} else if kind == "info" || kind == "danger" {
		n.Set("directive", kind)
		n.Set("alert", "")
	} else {
		n.Set("alert", strings.ToUpper(kind))
	}
}
