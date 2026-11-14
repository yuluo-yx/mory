package main

import "testing"

func TestOpenWelcomeSelectsGuide(t *testing.T) {
	a := testApp(t)
	a.openWelcome()
	if a.current == nil || a.current.Title != "Welcome to Mory" {
		t.Fatalf("welcome guide was not opened: current=%+v problem=%s", a.current, a.problem)
	}
}
