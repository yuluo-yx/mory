package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRemoteImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(iconPNG)
	}))
	defer server.Close()

	data, contentType, err := fetchRemoteImage(server.URL + "/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/png" || !bytes.Equal(data, iconPNG) {
		t.Fatalf("unexpected remote image response: type=%q bytes=%d", contentType, len(data))
	}
	if _, _, err := fetchRemoteImage(server.URL + "/missing"); err == nil {
		t.Fatal("missing remote image was accepted")
	}
	if isRemoteImageURL("assets/icon.png") || !isRemoteImageURL(server.URL+"/icon.png") {
		t.Fatal("remote image URL detection failed")
	}
}
