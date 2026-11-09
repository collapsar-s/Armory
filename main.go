// Command armory 是 Armory 渗透工具军械库的桌面应用入口。
//
// 技术栈：Wails v3（Go 后端）+ Vue 3 前端。前端构建产物（frontend/dist）通过
// go:embed 内嵌进可执行文件，运行时由 Wails 资产服务器提供。
//
// 本文件负责「装配」：初始化数据层与 API、经 platform.CreateMainWindow 创建
// Spotlight 式无边框毛玻璃窗口、安装 Carbon 全局热键、把平台原生能力（窗口动画/
// 玻璃/置顶/登录项/文件对话框）以 hook 的形式注入到 service 层，实现业务逻辑与
// 平台层的解耦。装配结构与原版一致（见 docs/orig-src-snapshot-9.26/main.go）。
package main

import (
	"embed"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"armory/internal/platform"
	"armory/internal/service"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 1) 数据层与 API
	reg, err := service.NewRegistry()
	if err != nil {
		log.Fatalf("初始化工具库失败: %v", err)
	}
	api := service.NewAPI(reg)

	// 2) 组装 Wails 应用
	app := application.New(application.Options{
		Name:        "Armory",
		Description: "Armory - 渗透作业的军械库",
		Services: []application.Service{
			application.NewService(api),
		},
		Assets: application.AssetOptions{
			Handler:        application.AssetFileServerFS(assets),
			DisableLogging: true,
		},
		LogLevel: slog.LevelInfo,
		Mac: application.MacOptions{
			// 关键：Armory 是可呼出/隐藏的常驻面板，启动工具后（以及按 Esc/⌥Space）会调用
			// Hide() 收起面板。若该项为 true，macOS 会把“隐藏最后一个窗口”当成关闭事件并
			// 直接终止进程——表现为“启动工具后 Armory 闪退”。故必须为 false；
			// 退出应用请使用 ⌘Q 或菜单“退出”。
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	// 3) 主窗口：Spotlight 式无边框玻璃面板（配置见 platform/window_options.go）
	win := platform.CreateMainWindow(app)

	// 4) 平台能力 + hook 注入（业务层通过接口调用，不直接依赖 Wails/原生细节）
	// 首帧布局后恢复一次红绿灯；窗口显示后再校正一次（按钮 frame 在显示后才稳定）。
	platform.OffsetTrafficLights(win)
	go func() {
		// 布局稳定后再次对齐（短延迟），覆盖首帧时按钮尚未定位的情况。
		time.Sleep(350 * time.Millisecond)
		platform.OffsetTrafficLights(win)
	}()
	// 启动时后台为缺失图标的 .app 工具补齐图标（升级后让历史条目也显示真实图标）。
	go api.BackfillIcons()

	// 文件拖放：Finder 拖入 → 转发前端收录。后端监听 Wails 原生 WindowFilesDropped，
	// 再以自定义事件 files-dropped 转发（payload 形如 {files:[...]}），与原版管线一致。
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		files := e.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		app.Event.Emit("files-dropped", map[string]any{"files": files})
	})

	// Esc / 启动后收起 → Launchpad 式淡出。
	api.SetHideHook(func() { platform.HideWindowAnimated(win) })
	api.SetShowHook(func() {
		platform.ShowWindowAnimated(win)
		// 显示后重新对齐红绿灯（保持与自定义标题栏一致的位置），并通知前端重播入场动画。
		platform.OffsetTrafficLights(win)
		app.Event.Emit("panel-shown")
	})
	// 深色主题 → 原生玻璃层叠深色 tint（CSS 主题与系统外观脱节的修复）。
	api.SetGlassHook(func(dark bool) { platform.SetGlassDark(win, dark) })
	api.SetTopHook(func(v bool) {
		win.SetAlwaysOnTop(v)
	})
	// 开机自启 → Wails AutostartManager（.app + macOS 13+ 走 SMAppService，
	// 「系统设置 → 登录项」原生条目；否则自动回退 LaunchAgent plist）。
	api.SetLoginHook(func(enabled bool) error {
		return platform.SetLaunchAtLogin(app, enabled)
	})
	// 备份导出/导入 → 原生文件对话框。
	api.SetDialogHooks(service.DialogHooks{
		OpenFile: func(title string, filters []string) (string, error) {
			return app.Dialog.OpenFile().SetTitle(title).PromptForSingleSelection()
		},
		SaveFile: func(title, defaultName string, filters []string) (string, error) {
			d := app.Dialog.SaveFile()
			d.SetOptions(&application.SaveFileDialogOptions{Title: title, Filename: defaultName})
			return d.PromptForSingleSelection()
		},
	})

	// 5) 全局热键 ⌥Space：呼出 / 隐藏（Carbon RegisterEventHotKey，与原版同机制）。
	// 面板真正出现时发 panel-shown，前端只在该时机播入场动画 + 聚焦搜索框。
	stopHotkey, err := platform.InstallToggleHotkey(func() {
		platform.ToggleWindow(win, func() {
			app.Event.Emit("panel-shown")
		})
	})
	if err != nil {
		// 不中断启动：热键失败时窗口仍可用，问题在验收时暴露。
		log.Printf("[armory] 全局快捷键安装失败: %v", err)
	} else {
		defer stopHotkey()
	}

	// 6) 启动
	if err := app.Run(); err != nil {
		log.Printf("运行失败: %v", err)
		os.Exit(1)
	}
}
