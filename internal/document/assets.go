package document

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
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
			return "", fmt.Errorf("read attachment %q: %w", path, err)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return "", err
		}
		if info.IsDir() || info.Size() > 32<<20 {
			return "", fmt.Errorf("invalid or oversized attachment: %s", path)
		}
		b, err := root.ReadFile(filepath.FromSlash(path))
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(b)
		name := fmt.Sprintf("%x", sum[:12]) + strings.ToLower(filepath.Ext(path))
		dir := filepath.Join(to, "assets")
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		dest := filepath.Join(dir, name)
		if existing, err := os.ReadFile(dest); err == nil {
			if sha256.Sum256(existing) != sum {
				return "", fmt.Errorf("attachment collision: %s", dest)
			}
		} else if !os.IsNotExist(err) {
			return "", err
		} else if err = library.AtomicWrite(dest, b); err != nil {
			return "", err
		}
		return "assets/" + name, nil
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
