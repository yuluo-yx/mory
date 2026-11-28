package document

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuluo-yx/mory/internal/library"
)

var markdownAsset = regexp.MustCompile(`(!?\[[^\]\n]*\]\()(<[^>]*>|[^)\s]+)(\))`)
var xmlAsset = regexp.MustCompile(`((?:href|src|path|cover)=")([^"]+)(")`)

// RewriteAssets handles relative file destinations only. URLs and anchors are
// left alone; an import never downloads remote content.
func RewriteAssets(source, format string, resolve func(string) (string, error)) (string, error) {
	pattern := markdownAsset
	if format == "xml" {
		pattern = xmlAsset
	}
	var first error
	out := pattern.ReplaceAllStringFunc(source, func(match string) string {
		parts := pattern.FindStringSubmatch(match)
		raw := strings.Trim(parts[2], "<>")
		if format == "xml" {
			raw = html.UnescapeString(raw)
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" {
			return match
		}
		path, err := resolve(u.Path)
		if err != nil {
			if first == nil {
				first = err
			}
			return match
		}
		u.Path = path
		replacement := u.String()
		if format == "xml" {
			replacement = html.EscapeString(replacement)
		}
		return parts[1] + replacement + parts[3]
	})
	return out, first
}

// CopyAssets uses a rooted filesystem so document references cannot read files
// outside their containing folder. Content-addressed names avoid collisions.
func CopyAssets(source, format, from, to string) (string, error) {
	root, err := os.OpenRoot(from)
	if err != nil {
		return "", err
	}
	defer root.Close()
	return RewriteAssets(source, format, func(path string) (string, error) {
		f, err := root.Open(filepath.FromSlash(path))
		if err != nil {
			return "", &AssetError{Name: filepath.Base(path), Reason: "read", Err: err}
		}
		defer f.Close()
		b, err := readAsset(f, filepath.Base(path))
		if err != nil {
			return "", err
		}
		return StoreAsset(b, filepath.Base(path), to)
	})
}

// EmbedImages makes HTML and PDF exports self-contained, including local images.
// Other relative links remain links; remote images are never fetched.
func EmbedImages(page, dir string) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	var first error
	pattern := regexp.MustCompile(`(<img\b[^>]*\bsrc=")([^"]+)(")`)
	result := pattern.ReplaceAllStringFunc(page, func(match string) string {
		parts := pattern.FindStringSubmatch(match)
		u, err := url.Parse(html.UnescapeString(parts[2]))
		if err != nil || u.Scheme != "" || u.Host != "" {
			return match
		}
		b, err := root.ReadFile(filepath.FromSlash(u.Path))
		if err != nil {
			first = err
			return match
		}
		mime := http.DetectContentType(b)
		if !strings.HasPrefix(mime, "image/") {
			first = fmt.Errorf("not an image: %s", u.Path)
			return match
		}
		return parts[1] + "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b) + parts[3]
	})
	return result, first
}

// MaxAssetBytes applies to every local attachment entry point, including covers,
// imported document resources, file drops and clipboard representations.
const MaxAssetBytes = 128 << 20

// AssetError keeps actionable attachment failures separate from document-save
// failures. The UI translates Reason without parsing filesystem error strings.
type AssetError struct {
	Name   string
	Reason string
	Size   int64
	Err    error
}

func (e *AssetError) Error() string {
	switch e.Reason {
	case "too-large":
		return fmt.Sprintf("attachment %q is %.1f MiB; the limit is %d MiB", e.Name, float64(e.Size)/(1<<20), MaxAssetBytes>>20)
	case "not-file":
		return fmt.Sprintf("attachment %q is not a regular file", e.Name)
	default:
		return fmt.Sprintf("cannot %s attachment %q: %v", e.Reason, e.Name, e.Err)
	}
}

func (e *AssetError) Unwrap() error { return e.Err }

// ReadAsset checks the open file and bounds the actual read as well as its stat
// size, so a file growing during import cannot bypass the limit.
func ReadAsset(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, &AssetError{Name: filepath.Base(path), Reason: "read", Err: err}
	}
	defer f.Close()
	return readAsset(f, filepath.Base(path))
}

func readAsset(f *os.File, name string) ([]byte, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, &AssetError{Name: name, Reason: "read", Err: err}
	}
	if !info.Mode().IsRegular() {
		return nil, &AssetError{Name: name, Reason: "not-file"}
	}
	if info.Size() > MaxAssetBytes {
		return nil, &AssetError{Name: name, Reason: "too-large", Size: info.Size()}
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxAssetBytes+1))
	if err != nil {
		return nil, &AssetError{Name: name, Reason: "read", Err: err}
	}
	if len(b) > MaxAssetBytes {
		return nil, &AssetError{Name: name, Reason: "too-large", Size: int64(len(b))}
	}
	return b, nil
}

// StoreAsset preserves the original bytes and reuses content-addressed files.
// The returned relative path is committed to the document only after success.
func StoreAsset(data []byte, name, to string) (string, error) {
	if len(data) > MaxAssetBytes {
		return "", &AssetError{Name: name, Reason: "too-large", Size: int64(len(data))}
	}
	sum := sha256.Sum256(data)
	filename := fmt.Sprintf("%x", sum[:12]) + strings.ToLower(filepath.Ext(name))
	dir := filepath.Join(to, "assets")
	failure := func(err error) (string, error) {
		return "", &AssetError{Name: name, Reason: "write", Err: err}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return failure(err)
	}
	dest := filepath.Join(dir, filename)
	if existing, err := os.ReadFile(dest); err == nil {
		if sha256.Sum256(existing) != sum {
			return failure(fmt.Errorf("attachment collision: %s", dest))
		}
	} else if !os.IsNotExist(err) {
		return failure(err)
	} else if err = library.AtomicWrite(dest, data); err != nil {
		return failure(err)
	}
	return "assets/" + filename, nil
}
