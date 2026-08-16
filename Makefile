.DEFAULT_GOAL := help

NPM ?= npm

.PHONY: help install web storage storage-macos windows-build-x64 windows-build-arm64 check test-unit test-go test-e2e verify test-macos package-macos package-windows-x64 package-windows-arm64 package-windows ci-macos ci-windows

help:
	@echo "Mory 本地构建目标"
	@echo "  make install                安装 package-lock.json 锁定的依赖"
	@echo "  make verify                 执行语法检查、单元测试和 Electron 端到端测试"
	@echo "  make storage                生成当前平台的存储插件侧车"
	@echo "  make package-macos          验证并生成当前架构的 macOS DMG、ZIP 与校验文件"
	@echo "  make windows-build-x64      交叉编译 WebView2 Windows x64 GUI（不打包）"
	@echo "  make windows-build-arm64    交叉编译 WebView2 Windows ARM64 GUI（不打包）"
	@echo "  make package-windows-x64    在 Windows 上生成 WebView2 x64 安装版和便携版"
	@echo "  make package-windows-arm64  在 Windows 上生成 WebView2 ARM64 安装版和便携版"
	@echo "  make package-windows        验证并生成两种 Windows 架构的全部制品"

install:
	$(NPM) ci

web:
	$(NPM) run build:web

storage:
	mkdir -p .build/storage
	env GOCACHE=$(CURDIR)/.cache/go-build go build -trimpath -o .build/storage/mory-storage ./cmd/mory-storage

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

package-macos: verify
	$(NPM) run build:mac
	$(NPM) run test:mac-web
	$(NPM) run test:mac-typing
	$(NPM) run test:mac-ime
	$(NPM) run test:mac-graph
	$(NPM) run test:mac-drag
	$(NPM) run test:mac-workspace-watcher
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
