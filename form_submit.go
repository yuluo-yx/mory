package main

import "github.com/egoist/mygo/ui"

// Use the native single-line submission event after bound text is synchronized.
// IME confirmation stays with the input method; modified Enter is not submit.
// Call before reading the submit button so either route runs one shared action.
func formSubmitted(input ui.Element) bool {
	input.OnTextKey(func(key ui.TextKey) bool {
		return key.Key == ui.KeyEnter && key.Mods != 0
	})
	return input.Submitted()
}
