.DEFAULT_GOAL := help

NPM ?= npm

.PHONY: help install web storage storage-macos storage-windows-x64 storage-windows-arm64 check test-unit test-go test-e2e verify test-macos package-macos package-windows-x64 package-windows-arm64 package-windows ci-macos ci-windows

help:
	@echo "Mory 本地构建目标"
	@echo "  make install                安装 package-lock.json 锁定的依赖"
	@echo "  make verify                 执行语法检查、单元测试和 Electron 端到端测试"
	@echo "  make storage                生成当前平台的存储插件侧车"
	@echo "  make package-macos          验证并生成当前架构的 macOS DMG、ZIP 与校验文件"
	@echo "  make package-windows-x64    验证并生成 Windows x64 安装版和便携版"
	@echo "  make package-windows-arm64  验证并生成 Windows ARM64 安装版和便携版"
	@echo "  make package-windows        验证并生成两种 Windows 架构的全部制品"

install:
	$(NPM) ci

web:
	$(NPM) run build:web

storage:
	mkdir -p .build/storage
	env GOCACHE=$(CURDIR)/.cache/go-build go build -trimpath -o .build/storage/mory-storage ./cmd/mory-storage

storage-macos: storage

storage-windows-x64:
	mkdir -p .build/storage
	env GOCACHE=$(CURDIR)/.cache/go-build GOOS=windows GOARCH=amd64 go build -trimpath -o .build/storage/mory-storage.exe ./cmd/mory-storage

storage-windows-arm64:
	mkdir -p .build/storage
	env GOCACHE=$(CURDIR)/.cache/go-build GOOS=windows GOARCH=arm64 go build -trimpath -o .build/storage/mory-storage.exe ./cmd/mory-storage

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

package-windows-x64: verify storage-windows-x64
	$(NPM) run pack:win

package-windows-arm64: verify storage-windows-arm64
	$(NPM) run pack:win:arm64

package-windows: verify
	$(MAKE) storage-windows-x64
	$(NPM) run pack:win
	$(MAKE) storage-windows-arm64
	$(NPM) run pack:win:arm64

ci-macos: package-macos

ci-windows: package-windows
