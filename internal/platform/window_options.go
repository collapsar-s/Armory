// 主窗口配置（全平台共享）。
//
// 视觉规范与已验收的 9-27 行为一致：
//   - 无边框玻璃面板，原生 Liquid Glass 材质（macOS 26+ NSGlassEffectView）
//   - 880×560（min 640×420）、44px 隐形标题栏、透明背景
//   - 文件拖放开启（main.go 监听 WindowFilesDropped 转发前端）
//
// darwin && cgo 版 CreateMainWindow 见 window.go（本文件由三个构建变体共享，
// 避免 nocgo/stub 回退重复维护同一份配置）。
package platform

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// createMainWindowOptions 以统一的 9-27 视觉配置创建主窗口。
func createMainWindowOptions(app *application.App) *application.WebviewWindow {
	return app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:     "main",
		Title:    "Armory",
		Width:    880,
		Height:   560,
		MinWidth: 640,

		MinHeight: 420,
		URL:       "/",

		Frameless:      true,
		EnableFileDrop: true, // 允许把 .app/.jar/.py/.sh 拖入面板收录（对应 files-dropped 事件）
		Mac: application.MacWindow{
			// 原生 Liquid Glass：macOS 26 走 NSGlassEffectView，玻璃透过 WebView
			// 可见需要 -tags private_mac_apis 构建。
			Backdrop:                application.MacBackdropLiquidGlass,
			InvisibleTitleBarHeight: 44,
			LiquidGlass: application.MacLiquidGlass{
				Style:        application.LiquidGlassStyleAutomatic, // 跟随系统明暗
				CornerRadius: 18,
			},
		},
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	})
}
