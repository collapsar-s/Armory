//go:build darwin && !cgo

// Package platform 封装 macOS 原生能力（纯 Go 回退实现）。
//
// 本文件覆盖 darwin 但 CGO 关闭（darwin && !cgo）的构建：窗口动画/玻璃/红绿灯/
// Carbon 热键降级为无操作或返回错误（由 Wails 自身窗口 API 兜底显示/隐藏）。
// SetLaunchAtLogin 经 Wails AutostartManager（SMAppService / LaunchAgent 双路径），
// 不依赖 cgo，功能完整可用。
//
// cgo 真实现见本目录各 *_darwin 文件（darwin && cgo）；
// 非 darwin 平台回退见 platform_stub_other.go（!darwin）。
package platform

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// CreateMainWindow 纯 Go 回退：与 cgo 版相同的窗口配置（配置本身不依赖 cgo）。
func CreateMainWindow(app *application.App) *application.WebviewWindow {
	return createMainWindowOptions(app)
}

// ShowWindowAnimated 显示窗口。纯 Go 回退：无操作，由调用方的 window.Show() 完成。
// TODO: CGO 关闭时无法操作 NSWindow，如需要请以 CGO_ENABLED=1 构建。
func ShowWindowAnimated(w *application.WebviewWindow) {
	_ = w
	w.Show()
}

// HideWindowAnimated 隐藏窗口。纯 Go 回退：无动画直接隐藏。
func HideWindowAnimated(w *application.WebviewWindow) {
	w.Hide()
}

// OffsetTrafficLights 调整红绿灯位置。纯 Go 回退：无操作（无 NSWindow 访问能力）。
func OffsetTrafficLights(w *application.WebviewWindow) {
	_ = w
}

// SetGlassDark 设置毛玻璃深浅。纯 Go 回退：无操作（玻璃外观由 Wails MacBackdrop 控制）。
func SetGlassDark(w *application.WebviewWindow, dark bool) {
	_, _ = w, dark
}

// InstallToggleHotkey 纯 Go 回退：无法注册 Carbon 热键。
func InstallToggleHotkey(fn func()) (func(), error) {
	_ = fn
	return func() {}, errors.New("global hotkey requires CGO_ENABLED=1 build")
}

// ToggleWindow 切换主面板显隐（无动画版）。
func ToggleWindow(w *application.WebviewWindow, onShown func()) {
	if w.IsVisible() {
		w.Hide()
	} else {
		w.Show()
		w.Focus()
		if onShown != nil {
			onShown()
		}
	}
}

// SetLaunchAtLogin 设置开机自启（经 Wails AutostartManager，不依赖 cgo）。
func SetLaunchAtLogin(app *application.App, enabled bool) error {
	if enabled {
		return app.Autostart.Enable()
	}
	return app.Autostart.Disable()
}
