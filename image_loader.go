package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const remoteImageLimit = 32 << 20

var remoteImageClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if !isRemoteImageURL(req.URL.String()) {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func isRemoteImageURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

func fetchRemoteImage(raw string) ([]byte, string, error) {
	if !isRemoteImageURL(raw) {
		return nil, "", fmt.Errorf("unsupported image URL")
	}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mory image loader")
	resp, err := remoteImageClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("image server returned %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, remoteImageLimit+1))
	if err != nil {
		return nil, "", err
	}
	if len(b) > remoteImageLimit {
		return nil, "", fmt.Errorf("remote image exceeds 32 MiB")
	}
	return b, resp.Header.Get("Content-Type"), nil
}

func decodeImage(data []byte, raw, contentType string) (ui.ImageSource, error) {
	ext := ""
	if parsed, err := url.Parse(raw); err == nil {
		ext = filepath.Ext(parsed.Path)
	}
	if strings.Contains(strings.ToLower(contentType), "image/svg+xml") || strings.EqualFold(ext, ".svg") {
		return ui.ParseSVG(data)
	}
	return ui.DecodeBitmap(data)
}

func (a *app) loadPreviewImage(raw string) (ui.ImageSource, error) {
	if isRemoteImageURL(raw) {
		data, contentType, err := fetchRemoteImage(raw)
		if err != nil {
			if raw == welcomeImageURL {
				return markdown.FileImages(a.assetRoot())("assets/mory-welcome.png")
			}
			return nil, err
		}
		return decodeImage(data, raw, contentType)
	}
	return markdown.FileImages(a.assetRoot())(raw)
}
