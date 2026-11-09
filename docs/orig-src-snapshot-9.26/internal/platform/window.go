//go:build darwin

package platform

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// CreateMainWindow 创建主面板窗口——原生 Liquid Glass（任务书 M0-2）。
//
// 视觉规范（任务书 §2.3）：
//   - 无边框玻璃面板，原生 NSGlassEffectView 材质（macOS 26+）
//   - 圆角 24px（同心圆角体系最外层）
//   - 浮动层级（Spotlight 式，浮于普通窗口之上）
//   - 失焦隐藏已按用户要求取消（2026-09-26）
// 文件拖放：EnableFileDrop 开启，main.go 监听 WindowFilesDropped 转发前端。
func CreateMainWindow(app *application.App) *application.WebviewWindow {
	return app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Armory",
		// 默认 760×520（≈1.46:1）：6 列卡片网格 + 4 行首屏不多不少；
		// 最小 600×420：保住 4 列网格与侧栏可用的下限
		Width:    760,
		Height:   520,
		MinWidth: 600,
		MinHeight: 420,

		EnableFileDrop: true,

		Mac: application.MacWindow{
			// 原生 Liquid Glass：macOS 26 走 NSGlassEffectView，
			// 旧系统回退 NSVisualEffectView（本项目硬性要求 26+）。
			// 玻璃透过 WebView 可见需要 -tags private_mac_apis 构建。
			Backdrop: application.MacBackdropLiquidGlass,
			LiquidGlass: application.MacLiquidGlass{
				Style:        application.LiquidGlassStyleAutomatic, // 跟随系统明暗
				CornerRadius: 24,
			},

			// 透明标题栏 + 保留红绿灯三按钮（对标 SpearX 左上角样式）：
			// AppearsTransparent 让标题栏融入内容，FullSizeContent 让内容满窗，
			// HideTitle 去掉窗口标题文字（前端品牌区已显示 Armory，避免重复）；
			// 红绿灯由系统绘制在左上角；顶部 40px 为系统拖拽区
			TitleBar: application.MacTitleBar{
				Hide:               false,
				HideTitle:          true,
				AppearsTransparent: true,
				FullSizeContent:    true,
			},
			InvisibleTitleBarHeight: 40,

			// Spotlight 式浮层：高于普通窗口
			WindowLevel: application.MacWindowLevelFloating,

			CornerType:   application.MacWindowCornerTypeRounded,
			CornerRadius: 24,
		},

		URL: "/",
	})
}
