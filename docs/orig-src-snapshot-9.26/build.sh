#!/bin/bash
# Armory 一键构建脚本
# 背景：~/.zshrc 全局 CGO_ENABLED=0（用户安全默认），本项目（Wails/Objective-C）必须 CGO=1。
# 用法:
#   ./build.sh          → 开发版编译（private_mac_apis + dev 标签，快速）
#   ./build.sh prod     → 生产版编译（production，裁剪调试信息）
#   ./build.sh app      → 生产版 + 打包成 .app
#   ./build.sh devapp   → 开发版 + 组装 .dev.app + 启动（日常开发推荐）
set -e
cd "$(dirname "$0")"

export CGO_ENABLED=1
export CGO_CFLAGS="-mmacosx-version-min=26.0"
export CGO_LDFLAGS="-mmacosx-version-min=26.0"

MODE="${1:-dev}"

case "$MODE" in
  dev)
    echo "→ 编译开发版..."
    go build -tags "private_mac_apis dev" -o bin/armory .
    ;;
  prod)
    echo "→ 编译生产版..."
    go build -tags "private_mac_apis production" -trimpath -ldflags="-w -s" -o bin/armory .
    ;;
  app)
    echo "→ 编译生产版并打包 .app..."
    # EXTRA_TAGS 必须带 private_mac_apis，否则 Liquid Glass 在正式包中失效
    "$HOME/go/bin/wails3" task package EXTRA_TAGS=private_mac_apis
    ;;
  devapp)
    echo "→ 编译开发版并启动..."
    go build -tags "private_mac_apis dev" -o bin/armory .
    mkdir -p bin/Armory.dev.app/Contents/MacOS bin/Armory.dev.app/Contents/Resources
    cp build/darwin/icons.icns bin/Armory.dev.app/Contents/Resources/ 2>/dev/null || true
    cp bin/armory bin/Armory.dev.app/Contents/MacOS/armory
    cp build/darwin/Info.dev.plist bin/Armory.dev.app/Contents/Info.plist
    codesign --force --deep --sign - bin/Armory.dev.app 2>/dev/null
    open bin/Armory.dev.app
    ;;
  *)
    echo "用法: ./build.sh [dev|prod|app]"; exit 1
    ;;
esac

echo "✓ 完成: bin/"
