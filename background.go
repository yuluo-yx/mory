package main

import "time"

// Saving and reminder checks continue when AppKit suspends painting an occluded
// window. All state is still confined to the application's main thread.
func (a *app) backgroundTick() {
	a.capture()
	if a.dirty && time.Since(a.dirtySince) >= 700*time.Millisecond {
		a.save()
	}
	a.refreshDisk()
}
