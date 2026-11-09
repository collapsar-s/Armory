//go:build !darwin

// Package platform 封装 macOS 原生能力（非 darwin 平台回退实现）。
//
// 本文件覆盖非 darwin 构建（如 Linux/Windows CI 上 `go build ./...`）：
// 所有平台相关操作均为无操作 / 返回明确的不支持错误，保证跨平台可编译。
//
// macOS 实现见 window.go / window_anim.go / glass_tint.go / trafficlights.go /
// hotkey.go / loginitem.go（darwin && cgo）与 platform_darwin_nocgo.go（darwin && !cgo）。
package platform

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// CreateMainWindow 非 darwin 平台：以相同配置创建窗口（平台选项在 darwin 分支内）。
func CreateMainWindow(app *application.App) *application.WebviewWindow {
	return createMainWindowOptions(app)
}

// ShowWindowAnimated 非 darwin 平台：无动画直接显示。
func ShowWindowAnimated(w *application.WebviewWindow) { w.Show() }

// HideWindowAnimated 非 darwin 平台：直接隐藏。
func HideWindowAnimated(w *application.WebviewWindow) { w.Hide() }

// OffsetTrafficLights 非 darwin 平台：无操作（无 macOS 红绿灯）。
func OffsetTrafficLights(w *application.WebviewWindow) { _ = w }

// SetGlassDark 非 darwin 平台：无操作（无 macOS 毛玻璃）。
func SetGlassDark(w *application.WebviewWindow, dark bool) { _, _ = w, dark }

// InstallToggleHotkey 非 darwin 平台：不支持，返回明确错误。
func InstallToggleHotkey(fn func()) (func(), error) {
	_ = fn
	return func() {}, errors.New("platform: InstallToggleHotkey 仅在 macOS 上受支持")
}

// ToggleWindow 非 darwin 平台：直接切换显隐。
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

// SetLaunchAtLogin 非 darwin 平台：不支持，返回明确错误。
func SetLaunchAtLogin(app *application.App, enabled bool) error {
	_, _ = app, enabled
	return errors.New("platform: SetLaunchAtLogin 仅在 macOS 上受支持")
}
