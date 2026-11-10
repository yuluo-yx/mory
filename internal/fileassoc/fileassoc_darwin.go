// Package fileassoc manages user-requested Markdown file associations on macOS.
package fileassoc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

type services struct {
	createString func(uintptr, string, uint32) uintptr
	release      func(uintptr)
	resolveType  func(uintptr, uintptr, uintptr) uintptr
	setHandler   func(uintptr, uint32, uintptr) int32
	register     func(uintptr, bool) int32
}

var load = sync.OnceValues(func() (*services, error) {
	s := &services{}
	for _, lib := range []struct {
		path  string
		funcs map[string]any
	}{
		{"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", map[string]any{"CFStringCreateWithCString": &s.createString, "CFRelease": &s.release}},
		{"/System/Library/Frameworks/CoreServices.framework/CoreServices", map[string]any{"UTTypeCreatePreferredIdentifierForTag": &s.resolveType, "LSSetDefaultRoleHandlerForContentType": &s.setHandler, "LSRegisterURL": &s.register}},
	} {
		h, err := purego.Dlopen(lib.path, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			return nil, err
		}
		for name, target := range lib.funcs {
			symbol, err := purego.Dlsym(h, name)
			if err != nil {
				return nil, err
			}
			purego.RegisterFunc(target, symbol)
		}
	}
	return s, nil
})

// SetMarkdownDefault uses the running bundle, so release and development builds
// never accidentally register each other's identity. It must run on the UI thread.
func SetMarkdownDefault() error {
	bundle := objc.ID(objc.GetClass("NSBundle")).Send(objc.RegisterName("mainBundle"))
	url := bundle.Send(objc.RegisterName("bundleURL"))
	id := bundle.Send(objc.RegisterName("bundleIdentifier"))
	ext := url.Send(objc.RegisterName("pathExtension"))
	// An unpackaged go run binary cannot be a persistent Finder application.
	appBytes := []byte{'a', 'p', 'p', 0}
	appString := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), &appBytes[0])
	if id == 0 || !objc.Send[bool](ext, objc.RegisterName("isEqualToString:"), appString) {
		return fmt.Errorf("open the installed Mory.app before changing default applications")
	}
	s, err := load()
	if err != nil {
		return err
	}
	return s.set(uintptr(url), uintptr(id))
}

func (s *services) set(url, id uintptr) error {
	if status := s.register(url, true); status != 0 {
		return fmt.Errorf("register Mory: macOS error %d", status)
	}
	tag := s.createString(0, "public.filename-extension", 0x08000100)
	defer s.release(tag)
	var failed []string
	// Resolve each extension through Launch Services instead of assuming every
	// Markdown editor declares the same UTI. Never claim the parent plain-text UTI.
	for _, ext := range []string{"md", "markdown", "mdx"} {
		e := s.createString(0, ext, 0x08000100)
		t := s.resolveType(tag, e, 0)
		s.release(e)
		if t == 0 {
			failed = append(failed, ext)
			continue
		}
		status := s.setHandler(t, 0xffffffff, id)
		s.release(t)
		if status != 0 {
			failed = append(failed, fmt.Sprintf("%s (%d)", ext, status))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not change default application for: %s", strings.Join(failed, ", "))
	}
	return nil
}
