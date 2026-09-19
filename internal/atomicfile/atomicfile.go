// Package atomicfile replaces completed files without truncating their previous contents.
package atomicfile

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Replace writes and closes a sibling temporary file before replacing name.
// All path operations remain relative to root. Existing relative file links are followed.
func Replace(root *os.Root, name string, mode os.FileMode, write func(*os.File) error) (err error) {
	existing := false
	for links := 0; ; links++ {
		info, statErr := root.Lstat(name)
		if errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("replacement target is not a regular file: %s", name)
			}
			mode = info.Mode().Perm()
			existing = true
			// Preserve the existing file's write permission contract before replacement.
			check, openErr := root.OpenFile(name, os.O_WRONLY, 0)
			if openErr != nil {
				return openErr
			}
			if closeErr := check.Close(); closeErr != nil {
				return closeErr
			}
			break
		}
		if links >= 40 {
			return fmt.Errorf("too many symbolic links: %s", name)
		}
		link, readErr := root.Readlink(name)
		if readErr != nil {
			return readErr
		}
		if filepath.IsAbs(link) {
			return fmt.Errorf("absolute symbolic link is outside the root contract: %s", name)
		}
		name = filepath.Clean(filepath.Join(filepath.Dir(name), link))
		if !filepath.IsLocal(name) {
			return fmt.Errorf("symbolic link escapes the root: %s", name)
		}
	}
	// Hold the resolved parent directory for both creation and replacement.
	parent, err := root.OpenRoot(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer parent.Close()
	temporary := ".mory-write-" + rand.Text() + ".tmp"
	file, err := parent.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	defer func() {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		if removeErr := parent.Remove(temporary); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	if !existing {
		info, statErr := file.Stat()
		if statErr != nil {
			return statErr
		}
		mode = info.Mode().Perm() // Preserve the creation umask for a new document.
	}
	if err = write(file); err != nil {
		return err
	}
	if err = file.Chmod(mode.Perm()); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	closing := file
	file = nil
	if err = closing.Close(); err != nil {
		return err
	}
	return parent.Rename(temporary, filepath.Base(name))
}

// WriteFile follows an existing document link and preserves its target's permissions.
func WriteFile(name string, data []byte, mode os.FileMode) error {
	resolved, err := filepath.EvalSymlinks(name)
	if err == nil {
		name = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	} else if _, linkErr := os.Lstat(name); linkErr == nil {
		return err // Do not replace a dangling link with a new document.
	}
	root, err := os.OpenRoot(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer root.Close()
	return Replace(root, filepath.Base(name), mode, func(file *os.File) error {
		_, err := bytes.NewReader(data).WriteTo(file)
		return err
	})
}
