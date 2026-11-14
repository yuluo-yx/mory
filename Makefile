GO ?= go
ARCH ?= $(shell $(GO) env GOARCH)
VERSION := $(shell python3 -c 'import json; print(json.load(open("mygo.json"))["version"])')

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
	$(GO) tool mygo build -platform darwin/$(ARCH) -skip-notarize
	cd build/darwin-$(ARCH) && shasum -a 256 mory-$(VERSION)-$(ARCH).dmg > SHA256SUMS
