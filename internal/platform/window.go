//go:build darwin && cgo

// Package platform 是 Wails/AppKit 平台隔离层。
// 铁律（开发计划 §9-4）：所有 Wails 窗口配置与平台私有 API 只允许出现在本包内。
package platform

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// CreateMainWindow 创建主面板窗口（Spotlight 式毛玻璃悬浮面板）。
// 窗口配置统一维护在 window_options.go（三个构建变体共享）。
func CreateMainWindow(app *application.App) *application.WebviewWindow {
	return createMainWindowOptions(app)
}
