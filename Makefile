.DEFAULT_GOAL := help

NPM ?= npm

.PHONY: help install web cli storage storage-macos windows-build-x64 windows-build-arm64 check test-unit test-go test-e2e verify test-macos package-macos package-windows-x64 package-windows-arm64 package-windows ci-macos ci-windows

help:
	@echo "Mory local build targets"
	@echo "  make install                Install dependencies from package-lock.json"
	@echo "  make verify                 Run syntax, unit, Go, and Electron E2E checks"
	@echo "  make storage                Build the storage sidecar for the current platform"
	@echo "  make cli                    Build the Mory command-line client for the current platform"
	@echo "  make package-macos          Build macOS DMG, ZIP, and checksums for this architecture"
	@echo "  make windows-build-x64      Cross-compile the WebView2 Windows x64 GUI"
	@echo "  make windows-build-arm64    Cross-compile the WebView2 Windows ARM64 GUI"
	@echo "  make package-windows-x64    Build Windows WebView2 x64 setup and portable artifacts"
	@echo "  make package-windows-arm64  Build Windows WebView2 ARM64 setup and portable artifacts"
	@echo "  make package-windows        Build all Windows setup and portable artifacts"

install:
	$(NPM) ci

web:
	$(NPM) run build:web

storage:
	mkdir -p .build/storage
	env GOCACHE=$(CURDIR)/.cache/go-build go build -trimpath -o .build/storage/mory-storage ./cmd/mory-storage

cli:
	mkdir -p .build/cli
	env GOCACHE=$(CURDIR)/.cache/go-build go build -trimpath -o .build/cli/mory ./cmd/mory

storage-macos: storage

windows-build-x64: web
	mkdir -p .build/windows
	env GOCACHE=$(CURDIR)/.cache/go-build GOOS=windows GOARCH=amd64 go build -tags desktop,production,wv2runtime.embed -trimpath -ldflags="-s -w -H windowsgui" -o .build/windows/Mory-x64.exe ./cmd/mory-windows

windows-build-arm64: web
	mkdir -p .build/windows
	env GOCACHE=$(CURDIR)/.cache/go-build GOOS=windows GOARCH=arm64 go build -tags desktop,production,wv2runtime.embed -trimpath -ldflags="-s -w -H windowsgui" -o .build/windows/Mory-arm64.exe ./cmd/mory-windows

check:
	$(NPM) run check

test-unit:
	$(NPM) test

test-go:
	env GOCACHE=$(CURDIR)/.cache/go-build go test ./...

test-e2e:
	$(NPM) run test:e2e

verify: check test-unit test-go test-e2e

test-macos:
	$(NPM) run test:mac-web
	$(NPM) run test:mac-typing
	$(NPM) run test:mac-ime
	$(NPM) run test:mac-graph
	$(NPM) run test:mac-drag
	$(NPM) run test:mac-workspace-watcher
	$(NPM) run test:mac-menu-localization
	$(NPM) run test:mac-launch-request

package-macos: verify
	$(NPM) run build:mac
	$(NPM) run test:mac-web
	$(NPM) run test:mac-typing
	$(NPM) run test:mac-ime
	$(NPM) run test:mac-graph
	$(NPM) run test:mac-drag
	$(NPM) run test:mac-workspace-watcher
	$(NPM) run test:mac-launch-request
	$(NPM) run dist:mac

package-windows-x64: verify
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows-wails.ps1 -Architecture amd64

package-windows-arm64: verify
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows-wails.ps1 -Architecture arm64

package-windows: verify
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows-wails.ps1 -Architecture amd64
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows-wails.ps1 -Architecture arm64

ci-macos: package-macos

ci-windows: package-windows
