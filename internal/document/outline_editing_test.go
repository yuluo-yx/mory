package document

import (
	"slices"
	"strings"
	"testing"
)

func TestHeadingsWithDraftList(t *testing.T) {
	for _, tc := range []struct {
		name, source, marker string
		want                 []string
	}{
		{"plain", "## Before\n\nParagraph\n-\n\n## After", "-", []string{"Before", "After"}},
		{"multiline", "First line\nSecond line\n-", "-", nil},
		{"CRLF and front matter", "---\r\ntitle: Header\r\n---\r\n\r\n## 正式标题\r\n\r\n普通段落\r\n - \r\n\r\n## 后续", " - ", []string{"正式标题", "后续"}},
		{"ATX", "## Keep\n-", "-", []string{"Keep"}},
		{"empty ATX", "##\n-", "-", []string{""}},
		{"tab ATX", "##\tKeep\n-", "-", []string{"Keep"}},
		{"code", "```md\nText\n-\n```\n\n## Keep", "-", []string{"Keep"}},
		{"list", "- Existing\n-", "-", nil},
		{"two dashes", "Keep\n--", "--", []string{"Keep"}},
		{"equals", "Keep\n=", "=", []string{"Keep"}},
		{"list content", "Body\n- Item", "- Item", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := strings.LastIndex(tc.source, "\n"+tc.marker) + 1
			got := HeadingsWithDraftList(tc.source, line)
			var titles []string
			for _, h := range got {
				titles = append(titles, h.Text)
				if h.Offset < 0 || h.Offset >= len(tc.source) {
					t.Fatal("invalid heading offset", h)
				}
			}
			if !slices.Equal(titles, tc.want) {
				t.Fatalf("got %+v, want %v", got, tc.want)
			}
		})
	}
	const source = "Title\n-"
	for _, offset := range []int{-1, 0, 4, 100} {
		got := HeadingsWithDraftList(source, offset)
		if len(got) != 1 || got[0].Text != "Title" {
			t.Fatalf("invalid draft offset %d suppressed an existing heading: %+v", offset, got)
		}
	}
}
