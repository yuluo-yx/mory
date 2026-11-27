package library

import (
	"fmt"
	"os"
	"path/filepath"
)

// DocumentPath returns the absolute path of the authored file. External notes
// refer to their originals; library notes use their saved format and location.
// A missing file still has a copyable path, so existence is checked by callers
// that need to open it. Resolving a path never saves pending edits.
func (l *Library) DocumentPath(id string) (string, error) {
	n := l.Get(id)
	if n == nil || n.FolderTrash {
		return "", fmt.Errorf("document no longer exists")
	}
	if n.External != "" && !n.Deleted {
		return filepath.Abs(n.External)
	}
	path := l.notePath(n)
	if saved := l.savedNotePath(n); saved != path {
		// savedNotePath also supplies a conflict-check path when both formats
		// are missing. Only expose the alternate format if it actually exists.
		if _, err := os.Stat(saved); err == nil {
			path = saved
		}
	}
	return filepath.Abs(path)
}
