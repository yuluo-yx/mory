GO ?= go

.PHONY: run test check race bench perf build

run:
	$(GO) tool mygo dev

test:
	$(GO) test ./...

# Native E2E needs a logged-in macOS desktop session.
check:
	python3 scripts/check.py

race:
	CGO_ENABLED=1 $(GO) test -race ./...

bench:
	$(GO) test -run '^$$' -bench . -benchtime=10x -benchmem ./...

# Real-window measurements require a logged-in macOS desktop session.
perf:
	python3 scripts/performance.py

build:
	$(GO) tool mygo build -platform darwin/universal -skip-notarize
	cd build/darwin-universal && shasum -a 256 mory-0.1.0.dmg > SHA256SUMS
