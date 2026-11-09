package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"armory/internal/platform"
	"armory/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// 注册跨端事件
	// toast: 服务层向前端推送提示
	// files-dropped: 文件拖入面板（Go 转发，见 main）
	// panel-shown: 面板真正出现（⌥Space 呼出），前端播入场动画
	application.RegisterEvent[string]("toast")
	application.RegisterEvent[map[string]any]("files-dropped")
	application.RegisterEvent[bool]("panel-shown")
}

// main 应用入口（M1 里程碑）。
// 装配顺序：数据服务 → 玻璃主面板 → 全局呼出热键 → 文件拖放 → 运行。
func main() {
	// 数据目录 ~/.armory
	home, _ := os.UserHomeDir()
	api, err := service.NewAPI(filepath.Join(home, ".armory"))
	if err != nil {
		log.Fatalf("[armory] 初始化工具库失败: %v", err)
	}

	app := application.New(application.Options{
		Name:        "Armory",
		Description: "Armory - 渗透作业的狐狸洞",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(api),
		},
		Mac: application.MacOptions{
			// 启动器语义：关窗不退出，常驻待呼出
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	win := platform.CreateMainWindow(app)

	// Esc / 启动后收起 → Launchpad 式淡出（service 层经钩子调用，不依赖 platform）
	api.SetHideHook(func() { platform.HideWindowAnimated(win) })

	// 深色主题 → 原生玻璃层加深色 tint（CSS 主题与系统外观脱节的修复）
	api.SetGlassHook(func(dark bool) bool { return platform.SetGlassDark(win, dark) })

	// 开机自启（M3-1）→ SMAppService 登录项注册
	api.SetLoginHook(platform.SetLaunchAtLogin)

	// 备份导出/导入（M4-4）→ 原生文件对话框
	api.SetDialogHooks(
		func(title, defaultName string) (string, error) {
			return app.Dialog.SaveFile().SetMessage(title).SetFilename(defaultName).PromptForSingleSelection()
		},
		func(title string) (string, error) {
			return app.Dialog.OpenFile().SetTitle(title).CanChooseFiles(true).PromptForSingleSelection()
		},
	)

	// 红绿灯与窗口边缘的间距加大（系统默认过贴边）
	platform.OffsetTrafficLights(win, 6, 4)

	// 注：按用户要求（2026-09-26），已取消"失焦自动隐藏"效果。
	// 面板收起途径：⌥Space / Esc / 启动工具后自动收起。

	// 文件拖放：Finder 拖入 → 转发前端收录（M1-3）
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		files := e.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		app.Event.Emit("files-dropped", map[string]any{
			"files":   files,
			"details": e.Context().DropTargetDetails(),
		})
	})

	// 全局快捷键 ⌥Space：呼出 / 隐藏（M0-3）
	// 面板真正出现时发 panel-shown，前端只在该时机播入场动画（非每次焦点切回）
	stopHotkey, err := platform.InstallToggleHotkey(func() {
		platform.ToggleWindow(win, func() {
			app.Event.Emit("panel-shown", true)
		})
	})
	if err != nil {
		// 不中断启动：热键失败时窗口仍可用，问题在验收时暴露
		log.Printf("[armory] 全局快捷键安装失败: %v", err)
	} else {
		defer stopHotkey()
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
