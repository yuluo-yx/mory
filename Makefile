.DEFAULT_GOAL := help

NPM ?= npm

.PHONY: help install web check test-unit test-e2e verify test-macos package-macos package-windows-x64 package-windows-arm64 package-windows ci-macos ci-windows

help:
	@echo "Mory 本地构建目标"
	@echo "  make install                安装 package-lock.json 锁定的依赖"
	@echo "  make verify                 执行语法检查、单元测试和 Electron 端到端测试"
	@echo "  make package-macos          验证并生成当前架构的 macOS Mory.app"
	@echo "  make package-windows-x64    验证并生成 Windows x64 安装版和便携版"
	@echo "  make package-windows-arm64  验证并生成 Windows ARM64 安装版和便携版"
	@echo "  make package-windows        验证并生成两种 Windows 架构的全部制品"

install:
	$(NPM) ci

web:
	$(NPM) run build:web

check:
	$(NPM) run check

test-unit:
	$(NPM) test

test-e2e:
	$(NPM) run test:e2e

verify: check test-unit test-e2e

test-macos:
	$(NPM) run test:mac-web
	$(NPM) run test:mac-typing
	$(NPM) run test:mac-ime
	$(NPM) run test:mac-drag

package-macos: verify
	$(NPM) run build:mac
	$(NPM) run test:mac-web
	$(NPM) run test:mac-typing
	$(NPM) run test:mac-ime
	$(NPM) run test:mac-drag

package-windows-x64: verify
	$(NPM) run pack:win

package-windows-arm64: verify
	$(NPM) run pack:win:arm64

package-windows: verify
	$(NPM) run pack:win
	$(NPM) run pack:win:arm64

ci-macos: package-macos

ci-windows: package-windows
