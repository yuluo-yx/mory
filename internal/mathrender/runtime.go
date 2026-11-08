// Package mathrender runs the official KaTeX engine inside moejs and paints its
// layout with MyGo glyphs, rules and SVG paths. No browser is used for editing.
package mathrender

import (
	stdctx "context"
	"embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/egoist/mygo/ui"
)

// KaTeX 0.16.22 is MIT licensed; the bundled notice is shipped with the app.
//
//go:embed assets/*
var assets embed.FS

var engine struct {
	once    sync.Once
	mu      sync.Mutex
	runtime *moejs.Runtime
	hook    moejs.Hook
	err     error
	cache   map[string]string
}

func initialize() {
	engine.cache = map[string]string{}
	files, _ := assets.ReadDir("assets")
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".ttf") {
			data, _ := assets.ReadFile("assets/" + file.Name())
			if err := ui.RegisterFont(data, ""); err != nil {
				engine.err = err
				return
			}
		}
	}
	js, _ := assets.ReadFile("assets/katex.min.js")
	source := "const module = {exports:{}}; const exports = module.exports;\n" + string(js) + "\nexport function render(x) { return module.exports.renderToString('\\\\displaystyle '+x.source, {displayMode:x.display, output:x.output, throwOnError:true, strict:'ignore', trust:false, maxExpand:1000, maxSize:100}); }"
	mod, err := moejs.Compile("katex-0.16.22.js", source)
	if err != nil {
		engine.err = err
		return
	}
	engine.hook, err = mod.Hook("render")
	if err != nil {
		engine.err = err
		return
	}
	engine.runtime = moejs.NewRuntime(moejs.Options{MemoryLimit: 128 << 20})
	engine.err = engine.runtime.Load(mod)
}

func HTML(source string, display bool, output string) (string, error) {
	engine.once.Do(initialize)
	if engine.err != nil {
		return "", engine.err
	}
	if len(source) > 64<<10 {
		return "", fmt.Errorf("equation exceeds 64 KiB")
	}
	key := fmt.Sprintf("%t:%s:%s", display, output, source)
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if hit, ok := engine.cache[key]; ok {
		return hit, nil
	}
	rt := engine.runtime
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), time.Second)
	defer cancel()
	done := make(chan struct{})
	stop := stdctx.AfterFunc(ctx, func() { rt.Interrupt(ctx.Err()); close(done) })
	defer func() {
		if !stop() {
			<-done
		}
		rt.ClearInterrupt()
		rt.ReleaseCallData()
	}()
	arg, err := rt.FromGo(map[string]any{"source": source, "display": display, "output": output})
	if err != nil {
		return "", err
	}
	value, err := rt.Call(engine.hook, arg)
	if err != nil {
		return "", fmt.Errorf("invalid equation: %w", err)
	}
	var html string
	if err = rt.Unmarshal(value, &html); err != nil {
		return "", err
	}
	if len(engine.cache) >= 512 {
		clear(engine.cache)
	}
	engine.cache[key] = html
	return html, nil
}
