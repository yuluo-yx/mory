package windowshost

import "testing"

func TestMIMETypeForPathUsesPortableMappings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "web font", path: "font.woff2", want: "font/woff2"},
		{name: "PNG image", path: "image.png", want: "image/png"},
		{name: "unknown asset", path: "asset.mory-unknown", want: "application/octet-stream"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mimeTypeForPath(test.path); got != test.want {
				t.Fatalf("MIME = %q, want %q", got, test.want)
			}
		})
	}
}
