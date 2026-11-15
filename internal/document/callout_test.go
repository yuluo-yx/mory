package document

import (
	"strings"
	"testing"
)

func TestXMLCalloutTypesSurviveMarkdownConversion(t *testing.T) {
	for _, tc := range []struct{ attrs, want string }{
		{`alert="WARNING"`, "> [!WARNING]"},
		{`alert="warn"`, "> [!WARNING]"},
		{`alert="TIP"`, "> [!TIP]"},
		{`alert="IMPORTANT"`, "> [!IMPORTANT]"},
		{`alert="CAUTION"`, "> [!CAUTION]"},
		{`alert="INFO"`, ":::info"},
		{`alert="DANGER"`, ":::danger"},
		{`directive="danger" title="Keep title"`, ":::danger Keep title"},
		{`directive="unknown"`, ":::note"},
	} {
		_, markdown, err := FromXML("<callout " + tc.attrs + "><p>Keep <b>formatting</b></p></callout>")
		if err != nil || !strings.Contains(markdown, tc.want) || !strings.Contains(markdown, "**formatting**") {
			t.Fatalf("conversion lost type or content: %s: %q (%v)", tc.attrs, markdown, err)
		}
	}
}
