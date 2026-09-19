package mory

import "testing"

func TestHostMessages(t *testing.T) {
	for chinese, english := range hostMessages {
		if got := HostMessage(chinese, "en"); got != english {
			t.Errorf("English translation = %q, want %q", got, english)
		}
		if got := HostMessage(chinese, "zh-CN"); got != chinese {
			t.Error("Chinese label changed")
		}
	}
	if got := HostMessage("\u5bfc\u51fa\u5931\u8d25\uff1aENOENT /notes/file.md", "en"); got != "Export failed: ENOENT /notes/file.md" {
		t.Fatalf("error details changed: %q", got)
	}
	if got := HostMessage("External error", "en"); got != "External error" {
		t.Fatal("external error changed")
	}
}
