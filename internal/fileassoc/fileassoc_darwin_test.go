package fileassoc

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// Launch Services needs declared content types to persist default handlers.
// Extension-only bundle declarations leave MDX as an unregistered dynamic UTI.
func TestPackagedMarkdownTypes(t *testing.T) {
	data, err := os.ReadFile("../../mygo.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MacOS struct {
			InfoPlist struct {
				Imported []struct {
					Identifier string   `json:"UTTypeIdentifier"`
					ConformsTo []string `json:"UTTypeConformsTo"`
					Tags       struct {
						Extensions []string `json:"public.filename-extension"`
					} `json:"UTTypeTagSpecification"`
				} `json:"UTImportedTypeDeclarations"`
				Documents []struct {
					Role  string   `json:"CFBundleTypeRole"`
					Types []string `json:"LSItemContentTypes"`
				} `json:"CFBundleDocumentTypes"`
			} `json:"infoPlist"`
		} `json:"macos"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	plist := config.MacOS.InfoPlist
	for _, ext := range []string{"md", "markdown", "mdx"} {
		t.Run(ext, func(t *testing.T) {
			for _, typ := range plist.Imported {
				if !slices.Contains(typ.Tags.Extensions, ext) {
					continue
				}
				if typ.Identifier == "" || strings.HasPrefix(typ.Identifier, "dyn.") || typ.Identifier == "public.plain-text" {
					t.Fatalf("extension %s needs its own declared content type: %q", ext, typ.Identifier)
				}
				if !slices.Contains(typ.ConformsTo, "public.plain-text") {
					t.Fatalf("%s must conform to plain text", typ.Identifier)
				}
				for _, doc := range plist.Documents {
					if doc.Role == "Editor" && slices.Contains(doc.Types, typ.Identifier) {
						return
					}
				}
				t.Fatalf("no document editor registered for %s", typ.Identifier)
			}
			t.Fatalf("no imported content type declared for .%s", ext)
		})
	}
}

func TestDefaultAssociations(t *testing.T) {
	for _, failure := range []string{"", "register", "resolve", "set"} {
		t.Run(failure, func(t *testing.T) {
			stringsByID := map[uintptr]string{}
			var next uintptr = 1
			var changed []string
			s := &services{
				createString: func(_ uintptr, v string, _ uint32) uintptr { next++; stringsByID[next] = v; return next },
				release:      func(uintptr) {},
				register: func(url uintptr, force bool) int32 {
					if url != 100 || !force {
						t.Fatal("wrong bundle")
					}
					if failure == "register" {
						return -1
					}
					return 0
				},
				resolveType: func(tag, ext, parent uintptr) uintptr {
					if stringsByID[tag] != "public.filename-extension" || parent != 0 {
						t.Fatal("wrong type scope")
					}
					if failure == "resolve" && stringsByID[ext] == "mdx" {
						return 0
					}
					return ext
				},
				setHandler: func(typ uintptr, role uint32, id uintptr) int32 {
					if role != 0xffffffff || id != 101 {
						t.Fatal("wrong association")
					}
					changed = append(changed, stringsByID[typ])
					if failure == "set" {
						return -2
					}
					return 0
				},
			}
			err := s.set(100, 101)
			if (err != nil) != (failure != "") {
				t.Fatal(err)
			}
			if failure == "" && strings.Join(changed, ",") != "md,markdown,mdx" {
				t.Fatal(changed)
			}
		})
	}
}

func TestNativeBindingsWithoutChangingDefaults(t *testing.T) {
	if _, err := load(); err != nil {
		t.Fatal(err)
	}
	// Test binaries are not app bundles; the operation must stop before mutation.
	if err := SetMarkdownDefault(); err == nil || !strings.Contains(err.Error(), "installed Mory.app") {
		t.Fatal(err)
	}
}
