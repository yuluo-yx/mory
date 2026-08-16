package windowshost

import "testing"

func TestMIMETypeForPathUsesPortableMappings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "Web 字体", path: "font.woff2", want: "font/woff2"},
		{name: "PNG 图片", path: "image.png", want: "image/png"},
		{name: "未知资源", path: "asset.mory-unknown", want: "application/octet-stream"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mimeTypeForPath(test.path); got != test.want {
				t.Fatalf("MIME = %q，期望 %q", got, test.want)
			}
		})
	}
}
