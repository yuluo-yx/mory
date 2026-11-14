package main

import (
	"strings"
	"testing"
)

func TestVeryLongLineUsesSourceMode(t *testing.T) {
	a := testApp(t)
	n, err := a.lib.Create("Long line", strings.Repeat("x", maxWriteLineBytes+1), "markdown")
	if err != nil {
		t.Fatal(err)
	}
	a.openNote(n)
	if a.mode != "Source" {
		t.Fatalf("very long line opened in %s mode", a.mode)
	}
	if a.blockEditor != nil || a.doc != nil {
		t.Fatal("very long line initialized a rich or block editor")
	}
	a.changeMode("Write")
	if a.mode != "Source" || !strings.Contains(a.status, "Source mode") {
		t.Fatalf("Write mode was not guarded: mode=%s status=%q", a.mode, a.status)
	}
	a.changeMode("Read")
	if a.mode != "Source" {
		t.Fatalf("Read mode was not guarded: mode=%s", a.mode)
	}
}
