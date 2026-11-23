package main

import "github.com/ebitengine/purego/objc"

func init() {
	nativePaste = func() {
		// Send directly through the responder chain, not through the menu item,
		// so text paste preserves native selection, formatting and undo behavior.
		app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		app.Send(objc.RegisterName("sendAction:to:from:"), objc.RegisterName("paste:"), objc.ID(0), objc.ID(0))
	}
}
