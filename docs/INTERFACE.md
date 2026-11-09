# 接口契约（main.go ↔ service ↔ platform）

> 由 Lead 依据 main.go 的实际调用固定。**后端 Agent 与平台 Agent 必须严格实现以下导出符号**，
> 否则 `go build` 失败。签名一旦需要变更，先与 Lead 沟通。

## A. package service（后端 Agent 实现）

### A.1 构造与 API 对象
```go
func NewRegistry() (*Registry, error)          // 打开/初始化 ~/.armory/tools.json
func NewAPI(reg *Registry) *API                 // 构造绑定给前端的 API（含 Launcher）
```

### A.2 API hook 注入（main.go 调用，签名固定）
```go
type DialogHooks struct {
    OpenFile func(title string, filters []string) (string, error)
    SaveFile func(title, defaultName string, filters []string) (string, error)
}
func (a *API) SetDialogHooks(h DialogHooks)
func (a *API) SetGlassHook(fn func(dark bool))
func (a *API) SetHideHook(fn func())
func (a *API) SetShowHook(fn func())
func (a *API) SetTopHook(fn func(alwaysOnTop bool))
func (a *API) SetLoginHook(fn func(enabled bool) error)
func (a *API) HidePanel()   // 调 hideHook
func (a *API) ShowPanel()   // 调 showHook（供热键显示；不暴露给前端绑定也可）
```

### A.3 绑定给前端的 27 个方法（Go 导出名 → 前端名）
> 返回值用 `model.Tool`（不再有重复 HasIcon 的 ToolView）。列表类返回 `[]model.Tool`。
> 前端通过官方 bindings 调用，Go 方法名即导出名。

```go
func (a *API) ListCategories() []model.Category
func (a *API) ListTools(categoryID string) []model.Tool
func (a *API) AddToolByPath(path, name, args, categoryID, envVersion string) (model.Tool, error)
func (a *API) UpdateTool(id, name, path, args, categoryID, envVersion string) (model.Tool, error)
func (a *API) DeleteTool(id string) error
func (a *API) AddCategory(name string) (model.Category, error)
func (a *API) RenameCategory(id, name string) error
func (a *API) DeleteCategory(id string) error
func (a *API) Launch(id string) error
func (a *API) GetIcon(id string) (string, error)      // dataURL 或 ""
func (a *API) GetSettings() model.Settings
func (a *API) SetTheme(theme string) error
func (a *API) SetReduceMotion(v bool) error
func (a *API) SetLaunchAtLogin(v bool) error
func (a *API) SetAlwaysOnTop(v bool) error
func (a *API) SetFontScale(v string) error
func (a *API) GetAccentColor() string
func (a *API) SetGlassTheme(dark bool)                 // 触发 glassHook（无返回值）
func (a *API) SaveJavaVersions(list []model.RuntimeVersion) error
func (a *API) SavePythonVersions(list []model.RuntimeVersion) error
func (a *API) DetectJavaVersions() []model.RuntimeVersion
func (a *API) DetectPythonVersions() []model.RuntimeVersion
func (a *API) ListRecentTools(n int) []model.Tool
func (a *API) GetToolReadme(id string) (string, error)
func (a *API) ExportBackup() (string, error)
func (a *API) ImportBackup(path string) error
func (a *API) HidePanel()                              // 同 A.2
```
> 注：`ShowPanel` 与 `HidePanel` 同名冲突需注意——`HidePanel` 前端可见，`ShowPanel` 内部用。

## B. package platform（平台 Agent 实现）

> 目标平台 darwin；提供 cgo 真实现（`//go:build darwin && cgo`）与纯 Go 回退（`//go:build darwin && !cgo` 及非 darwin）。
> 所有函数都要在两个构建下都存在（回退为无操作）。

```go
func ShowWindowAnimated()                  // 窗口淡入/缩放动画显示
func HideWindowAnimated()                  // 窗口淡出/缩放动画隐藏
func OffsetTrafficLights()                 // 调整 macOS 红绿灯位置对齐自定义标题栏
func SetGlassDark(dark bool)               // 设置毛玻璃深/浅色
func SetLaunchAtLogin(enabled bool) error  // 开机自启（SMAppService / LaunchAgent）
```
- cgo 实现可用 Objective-C `.m` 文件 + cgo 桥接（`#cgo LDFLAGS: -framework Cocoa -framework AppKit ...`）。
- 需要访问 NSWindow 时，可通过 Wails 窗口句柄或遍历 NSApp windows；若难以拿到句柄，允许合理降级（记录 TODO 但不崩）。
