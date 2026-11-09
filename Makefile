# Armory 构建入口（封装 wails3 Taskfile + 必要的沙箱环境变量）
#
# 本环境有文件沙箱：默认 GOCACHE(~Library/Caches/go-build) 与 npm 缓存(~/.npm)
# 均不可写，故统一重定向到 /tmp；node/pnpm 在 nvm 中，需 source。
# CGO_ENABLED 该环境默认 0，wails/production 必需 1。

SHELL := /bin/bash
export PATH := $(HOME)/go/bin:$(PATH)
export GOCACHE := /tmp/gocache-armory
export npm_config_cache := /tmp/npmcache-armory
export CGO_ENABLED := 1

.PHONY: help deps frontend build orig-ui restore-orig app package dmg run dev vet test fmt tidy clean

help:
	@echo "make deps      - 安装前端依赖"
	@echo "make frontend  - 构建前端 -> frontend/dist"
	@echo "make build     - 编译 bin/armory（源码版，先重建前端）"
	@echo "make orig-ui   - 构建 bin/armory.orig-ui（内嵌原版前端 docs/orig-frontend-9.27，"
	@echo "                 会临时改写 frontend/dist，之后跑 make build 即可还原源码版 dist）"
	@echo "make restore-orig - 还原原版：内嵌原版前端构建 bin/armory(.orig-ui) 并打包 bin/armory.app，"
	@echo "                 构建后自动恢复源码版 frontend/dist"
	@echo "make package   - 生成 bin/armory.app + ad-hoc 签名"
	@echo "make dmg       - 生成 bin/armory.dmg（需 hdiutil 权限）"
	@echo "make run       - 构建并运行 .dev.app"
	@echo "make vet/test  - go vet / go test"
	@echo "make fmt/tidy  - gofmt / go mod tidy"
	@echo "make clean     - 清理产物"

deps:
	@mkdir -p $(GOCACHE) $(npm_config_cache)
	@bash -lc 'source ~/.nvm/nvm.sh && cd frontend && pnpm install'

frontend:
	@bash -lc 'source ~/.nvm/nvm.sh && cd frontend && ./node_modules/.bin/vite build'

build: frontend
	go build -tags "production,private_mac_apis" -trimpath -buildvcs=false -ldflags="-w -s" -o bin/armory .

# 用提取固化的原版前端（与运行中 app 内嵌字节一致）构建"原装 UI"版。
orig-ui:
	@rm -rf frontend/dist && mkdir -p frontend/dist/assets
	@cp docs/orig-frontend-9.27/index.html frontend/dist/
	@cp docs/orig-frontend-9.27/assets/* frontend/dist/assets/
	@cp docs/orig-frontend-9.27/Inter-Medium.ttf frontend/dist/
	go build -tags "production,private_mac_apis" -trimpath -buildvcs=false -ldflags="-w -s" -o bin/armory.orig-ui .
	@echo "bin/armory.orig-ui 构建完成（内嵌原版前端字节）"

# 还原原版：内嵌原版前端（docs/orig-frontend-9.27）构建 bin/armory 与 bin/armory.orig-ui，
# 并打包 bin/armory.app。构建期间临时替换 frontend/dist，结束后自动恢复源码版 dist。
restore-orig:
	@set -e; backup=$$(mktemp -d /tmp/armory-dist.XXXXXX); \
	  cp -R frontend/dist/. "$$backup"/; \
	  trap 'rm -rf frontend/dist && mkdir -p frontend/dist && cp -R "$$backup"/. frontend/dist/ && rm -rf "$$backup"' EXIT; \
	  rm -rf frontend/dist && mkdir -p frontend/dist/assets; \
	  cp docs/orig-frontend-9.27/index.html frontend/dist/; \
	  cp docs/orig-frontend-9.27/assets/* frontend/dist/assets/; \
	  cp docs/orig-frontend-9.27/Inter-Medium.ttf frontend/dist/; \
	  go build -tags "production,private_mac_apis" -trimpath -buildvcs=false -ldflags="-w -s" -o bin/armory .; \
	  cp bin/armory bin/armory.orig-ui
	@wails3 task create:app:bundle
	@echo "bin/armory.app 已还原为原版前端（+ 后端对齐），源码版 frontend/dist 已恢复"

app: package
package: build
	wails3 task package

dmg: package
	wails3 task dmg

run: build
	wails3 task run

vet:
	go vet ./...

test:
	go test ./internal/... -count=1

fmt:
	gofmt -l -w internal main.go

tidy:
	go mod tidy

clean:
	rm -rf bin frontend/dist/assets
