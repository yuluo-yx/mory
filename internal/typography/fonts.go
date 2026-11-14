// Package typography embeds the Chinese typeface used by Mory's native UI.
package typography

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Keep the Latin face first so its spaces and code columns retain their width.
// Luo then supplies Chinese glyphs before the platform's automatic fallback.
const (
	Text    = "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', Luo, sans-serif"
	Display = "-apple-system, BlinkMacSystemFont, 'SF Pro Display', Luo, sans-serif"
	Mono    = "monospace, Luo"
)

// The unmodified upstream font and its OFL notice are pinned in THIRD_PARTY.md.
//
//go:embed assets/Luo-Regular.ttf
var luo []byte

var register = sync.OnceValue(func() error { return ui.RegisterFont(luo, "Luo") })

// Register makes Luo available before any window measures or renders text.
// Registration is process-local and shared by all Mory windows.
func Register() error { return register() }
