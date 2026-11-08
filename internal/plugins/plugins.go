// Package plugins runs document transforms in isolated moejs runtimes.
package plugins

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
)

//go:embed builtin/*.js
var builtins embed.FS

type Plugin struct {
	Name, Path string
	module     *moejs.Module
	hook       moejs.Hook
}
type Input struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}
type Output struct {
	Markdown *string `json:"markdown"`
	Message  string  `json:"message"`
}

// Load discovers local scripts without executing them. A malformed script is
// reported independently; it does not disable other plugins or the editor.
func Load(dir string) ([]*Plugin, []error) {
	var result []*Plugin
	var problems []error
	entries, _ := builtins.ReadDir("builtin")
	for _, e := range entries {
		b, _ := builtins.ReadFile("builtin/" + e.Name())
		p, err := Compile(e.Name(), string(b))
		if err != nil {
			problems = append(problems, err)
		} else {
			p.Path = "Built in"
			result = append(result, p)
		}
	}
	entries2, err := os.ReadDir(dir)
	if err != nil {
		return result, append(problems, err)
	}
	for _, e := range entries2 {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		p, err := Compile("User: "+e.Name(), string(b))
		if err != nil {
			problems = append(problems, err)
		} else {
			p.Path = path
			result = append(result, p)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, problems
}

func Compile(name, source string) (*Plugin, error) {
	if len(source) > 1<<20 {
		return nil, fmt.Errorf("%s: plugin exceeds 1 MiB", name)
	}
	m, err := moejs.Compile(name, source)
	if err != nil {
		return nil, err
	}
	h, err := m.Hook("transform")
	if err != nil {
		return nil, err
	}
	return &Plugin{Name: strings.TrimSuffix(name, ".js"), module: m, hook: h}, nil
}

func (p *Plugin) Run(ctx context.Context, in Input) (Output, error) {
	ctx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	rt := moejs.NewRuntime(moejs.Options{MemoryLimit: 32 << 20})
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { rt.Interrupt(context.Cause(ctx)); close(done) })
	defer func() {
		if !stop() {
			<-done
		}
	}()
	if err := rt.Load(p.module); err != nil {
		return Output{}, fmt.Errorf("%s: %w", p.Name, err)
	}
	arg, err := rt.FromGo(in)
	if err != nil {
		return Output{}, err
	}
	value, err := rt.Call(p.hook, arg)
	if err != nil {
		return Output{}, fmt.Errorf("%s: %w", p.Name, err)
	}
	var out Output
	if err = rt.Unmarshal(value, &out); err != nil {
		return out, err
	}
	if out.Markdown == nil && out.Message == "" {
		return out, fmt.Errorf("%s: return markdown or a message", p.Name)
	}
	return out, nil
}
