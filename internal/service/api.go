package service

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"armory/internal/model"
	"armory/internal/store"
)

// api.go 定义暴露给前端（Wails bindings）的 API 对象。
//
// API 聚合 Registry（数据）与 Launcher（启动），并通过一组 hook 与平台层解耦：
// 玻璃效果、窗口显示/隐藏、置顶、开机自启、文件对话框等能力由 main 包在启动时注入。

// DialogHooks 提供原生打开/保存文件对话框能力。
type DialogHooks struct {
	OpenFile func(title string, filters []string) (string, error)
	SaveFile func(title, defaultName string, filters []string) (string, error)
}

// API 是 Wails 绑定的服务对象。方法名即前端调用的名称。
type API struct {
	reg    *Registry
	launch *Launcher

	dialogs DialogHooks

	glassHook func(dark bool)
	hideHook  func()
	showHook  func()
	topHook   func(alwaysOnTop bool)
	loginHook func(enabled bool) error
}

// NewAPI 构造 API。
func NewAPI(reg *Registry) *API {
	return &API{reg: reg, launch: NewLauncher(reg)}
}

// ---------- hook 注入 ----------

// SetDialogHooks 注入文件对话框回调。
func (a *API) SetDialogHooks(h DialogHooks) { a.dialogs = h }

// SetGlassHook 注入玻璃主题回调。
func (a *API) SetGlassHook(fn func(dark bool)) { a.glassHook = fn }

// SetHideHook 注入隐藏面板回调。
func (a *API) SetHideHook(fn func()) { a.hideHook = fn }

// SetShowHook 注入显示面板回调（供热键调用）。
func (a *API) SetShowHook(fn func()) { a.showHook = fn }

// SetTopHook 注入窗口置顶回调。
func (a *API) SetTopHook(fn func(bool)) { a.topHook = fn }

// SetLoginHook 注入开机自启回调。
func (a *API) SetLoginHook(fn func(bool) error) { a.loginHook = fn }

// ---------- 分类与工具 ----------

// ListCategories 返回分类列表。
func (a *API) ListCategories() []model.Category { return a.reg.ListCategories() }

// ListTools 返回指定分类下的工具列表（categoryID 为空或 "all" 返回全部）。
func (a *API) ListTools(categoryID string) []model.Tool { return a.reg.ListTools(categoryID) }

// AddToolByPath 从路径收录工具：
//   - 剥掉拖拽传来的 file:// 前缀；
//   - os.Stat 校验（目录合法——.app 包本身是目录）；
//   - 默认名 = baseName（.app 用 bundleName），手动名称优先；
//   - 空分类归入第一个分类；
//   - .app 收录后尽力提取真实图标（失败不阻断，前端走类型 emoji 兜底）；
//   - jvmArgs 仅 jar 工具使用（JVM 选项，启动时拼在 -jar 之前）。
func (a *API) AddToolByPath(path, categoryID, name, args, jvmArgs, envVersion string) (model.Tool, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return model.Tool{}, fmt.Errorf("路径为空")
	}
	p = strings.TrimPrefix(p, "file://")
	np := ExpandPath(p)
	if _, err := os.Stat(np); err != nil {
		return model.Tool{}, fmt.Errorf("路径不存在: %s", p)
	}
	// 防呆：同一路径不允许重复收录（批量拖拽/重复导入最常见的脏数据来源）。
	for _, ex := range a.reg.ListTools("") {
		if ExpandPath(ex.Path) == np {
			return model.Tool{}, fmt.Errorf("该路径已收录为「%s」，请直接编辑原工具或拖到卡片上改分组", ex.Name)
		}
	}

	t := model.Tool{
		Name:       baseName(p),
		Path:       p,
		Type:       DetectType(p),
		CategoryID: categoryID,
	}
	if t.Type == "app" {
		if n := bundleName(ExpandPath(p)); n != "" {
			t.Name = n
		}
	}
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		t.Name = trimmed // 手动指定的名称优先
	}
	t.Args = strings.TrimSpace(args)
	t.JvmArgs = strings.TrimSpace(jvmArgs)
	t.EnvVersion = strings.TrimSpace(envVersion) // 空=系统默认；非空=采用全局设置的指定版本
	if t.CategoryID == "" {
		// 拖拽收录时前端在"全部"视图下传空分类，归入第一个**顶层目录**（默认分类）。
		// 注意不能取 ListCategories()[0] 就完事——树序里也可能是顶层 title，需显式挑 dir。
		t.CategoryID = firstTopDirCategoryID(a.reg.ListCategories())
	}
	created, err := a.reg.AddTool(t)
	if err != nil {
		return model.Tool{}, err
	}
	// .app 尝试提取真实图标（失败不阻断收录，前端走类型图标兜底）
	if created.Type == "app" {
		_ = ExtractAppIcon(ExpandPath(created.Path), store.IconPathFor(created.ID))
	}
	return created, nil
}

// BackfillIcons 为已收录但缺失图标的 .app 工具补齐图标（启动时调用，尽力而为）。
// 用于升级后让历史数据也显示真实应用图标。
func (a *API) BackfillIcons() {
	for _, t := range a.reg.ListTools("all") {
		if t.Type == "app" {
			if _, err := os.Stat(store.IconPathFor(t.ID)); err != nil {
				_ = ExtractAppIcon(ExpandPath(t.Path), store.IconPathFor(t.ID))
			}
		}
	}
}

// UpdateTool 更新工具可编辑字段（前端弹层总是传全量字段，直接覆盖；
// 空路径/空分类视为不修改该字段；args/jvmArgs 允许显式清空；envVersion 允许
// 显式清空回 "自动"；workingDir 空=按类型取默认工作目录；tags/term 整体覆盖）。
// 前端拖拽改归属时 args/jvmArgs/tags 传空占位，store 会保留原值。
func (a *API) UpdateTool(id, name, path, args, jvmArgs string, tags []string, term bool, categoryID, envVersion, workingDir string) (model.Tool, error) {
	out, err := a.reg.UpdateTool(id, name, path, args, jvmArgs, tags, term, categoryID, envVersion, workingDir)
	if err != nil {
		return model.Tool{}, err
	}
	// 路径变了则刷新图标
	if path != "" && out.Type == "app" {
		_ = ExtractAppIcon(ExpandPath(out.Path), store.IconPathFor(out.ID))
	}
	return out, nil
}

// DeleteTool 删除工具（含图标缓存）。
func (a *API) DeleteTool(id string) error {
	_ = os.Remove(store.IconPathFor(id))
	return a.reg.DeleteTool(id)
}

// AddCategory 新增顶层目录（kind=dir）。保持单参数签名，前端调用名 addCategory。
func (a *API) AddCategory(name string) (model.Category, error) {
	return a.reg.AddCategory(name)
}

// AddSubCategory 在 parentID 下新建分组（页内分节）。前端调用名 addSubCategory。
func (a *API) AddSubCategory(parentID, name string) (model.Category, error) {
	return a.reg.AddSubCategory(parentID, name)
}

// ReorderCategories 按给定 id 顺序重排分类（侧栏主分组 / 页内分组分节头的拖拽排序）。
// 前端调用名 reorderCategories。
func (a *API) ReorderCategories(ids []string) error { return a.reg.ReorderCategories(ids) }

// ReorderTools 按给定 id 顺序重排工具（同分组内卡片拖拽排序）。前端调用名 reorderTools。
func (a *API) ReorderTools(ids []string) error { return a.reg.ReorderTools(ids) }

// RenameTag 跨工具重命名标签（标签条 chip 右键入口）。
func (a *API) RenameTag(oldName, newName string) error { return a.reg.RenameTag(oldName, newName) }

// DeleteTag 从所有工具上移除标签。前端调用名 deleteTag。
func (a *API) DeleteTag(name string) error { return a.reg.DeleteTag(name) }

// RenameCategory 重命名分类。
func (a *API) RenameCategory(id, name string) error { return a.reg.RenameCategory(id, name) }

// DeleteCategory 删除分类。
func (a *API) DeleteCategory(id string) error { return a.reg.DeleteCategory(id) }

// Launch 启动工具。
func (a *API) Launch(id string) error { return a.launch.Launch(id) }

// ---------- 图标与设置 ----------

// GetIcon 返回工具图标的裸 base64 编码（不含 data: 前缀；由前端拼接
// "data:image/png;base64,"，与原版运行时管线一致）；不存在时返回空串。
func (a *API) GetIcon(id string) (string, error) {
	b, err := os.ReadFile(store.IconPathFor(id))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("读取图标失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// GetSettings 返回设置。
func (a *API) GetSettings() model.Settings { return a.reg.GetSettings() }

// SetTheme 设置主题。
func (a *API) SetTheme(theme string) error {
	return a.reg.UpdateSettings(func(s *model.Settings) { s.Theme = theme })
}

// SetReduceMotion 设置是否减弱动态效果。
func (a *API) SetReduceMotion(v bool) error {
	return a.reg.UpdateSettings(func(s *model.Settings) { s.ReduceMotion = v })
}

// SetFontScale 设置字体缩放。与原版一致：非法档位直接拒绝。
func (a *API) SetFontScale(v string) error {
	switch v {
	case "standard", "large", "xlarge":
	default:
		return fmt.Errorf("无效的字体缩放档位: %s", v)
	}
	return a.reg.UpdateSettings(func(s *model.Settings) { s.FontScale = v })
}

// SetAlwaysOnTop 设置窗口置顶，并触发平台回调。
func (a *API) SetAlwaysOnTop(v bool) error {
	if err := a.reg.UpdateSettings(func(s *model.Settings) { s.AlwaysOnTop = v }); err != nil {
		return err
	}
	if a.topHook != nil {
		a.topHook(v)
	}
	return nil
}

// SetLaunchAtLogin 设置开机自启（先调用平台实现，成功后再落盘）。
func (a *API) SetLaunchAtLogin(v bool) error {
	if a.loginHook != nil {
		if err := a.loginHook(v); err != nil {
			return err
		}
	}
	return a.reg.UpdateSettings(func(s *model.Settings) { s.LaunchOnLogin = v })
}

// SetGlassTheme 设置玻璃明暗，并触发平台回调（无返回值）。
func (a *API) SetGlassTheme(dark bool) {
	if a.glassHook != nil {
		a.glassHook(dark)
	}
}

// GetAccentColor 返回系统强调色（十六进制字符串）。
// 读取 macOS 全局偏好 AppleHighlightColor（用户改过强调色才存在该键），
// 未设置或读取失败回退系统蓝 #0a84ff。
func (a *API) GetAccentColor() string {
	return SystemAccentColor()
}

// ---------- 运行时版本 ----------

// SaveJavaVersions 保存 Java 版本配置（原版不在此处做版本名校验，由前端把关）。
func (a *API) SaveJavaVersions(list []model.RuntimeVersion) error {
	return a.saveVersions(list, true)
}

// SavePythonVersions 保存 Python 版本配置。
func (a *API) SavePythonVersions(list []model.RuntimeVersion) error {
	return a.saveVersions(list, false)
}

// saveVersions 保存版本配置到设置。
func (a *API) saveVersions(list []model.RuntimeVersion, java bool) error {
	return a.reg.UpdateSettings(func(s *model.Settings) {
		if java {
			s.JavaVersions = list
		} else {
			s.PythonVersions = list
		}
	})
}

// DetectJavaVersions 扫描本机 Java 版本。
func (a *API) DetectJavaVersions() []model.RuntimeVersion { return DetectJavaVersions() }

// DetectPythonVersions 扫描本机 Python 版本。
func (a *API) DetectPythonVersions() []model.RuntimeVersion { return DetectPythonVersions() }

// ---------- 最近使用 / 说明 / 备份 ----------

// ListRecentTools 返回最近使用的工具。
func (a *API) ListRecentTools(n int) []model.Tool { return a.reg.ListRecent(n) }

// GetToolReadme 返回工具说明文档。
func (a *API) GetToolReadme(id string) (string, error) { return a.reg.GetToolReadme(id) }

// ---------- 窗口 ----------

// HidePanel 隐藏面板窗口（前端可见）。
func (a *API) HidePanel() {
	if a.hideHook != nil {
		a.hideHook()
	}
}

// ShowPanel 显示面板窗口（供全局热键内部调用，不直接暴露给前端）。
func (a *API) ShowPanel() {
	if a.showHook != nil {
		a.showHook()
	}
}
