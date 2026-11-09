//go:build darwin && cgo

// 平台层：开机自启登录项。
//
// 9-27 实测语义（二进制取证）：走 Wails 内置 Autostart 管理器——
// .app bundle + macOS 13+ 走 SMAppService.mainAppService（「系统设置 → 登录项」
// 原生条目）；失败/旧系统/非 bundle 自动回退 LaunchAgent plist，禁用时双路径清理。
// 本文件仅提供 cgo 构建下的薄封装（无自有 C 代码）。
package platform

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SetLaunchAtLogin 注册/注销开机自启登录项（经 Wails AutostartManager）。
func SetLaunchAtLogin(app *application.App, enabled bool) error {
	if enabled {
		return app.Autostart.Enable()
	}
	return app.Autostart.Disable()
}
