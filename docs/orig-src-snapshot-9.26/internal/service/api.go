package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"armory/internal/model"
)

// API 前端绑定门面（M1-2a）。
// 仅此结构体会暴露给前端（main.go Bind）；内部分工：Reg 管数据、Lau 管启动。
type API struct {
	Reg       *Registry `json:"-"`
	lau       *Launcher
	dataDir   string
	hideHook    func()              // 面板淡出隐藏钩子（main.go 注入 platform.HideWindowAnimated）
	glassHook   func(dark bool) bool // 玻璃层深浅切换钩子（main.go 注入 platform.SetGlassDark）
	loginHook   func(bool) error    // 开机自启注册钩子（main.go 注入 platform.SetLaunchAtLogin）
	saveFileHook func(title, defaultName string) (string, error) // 备份导出保存对话框
	openFileHook func(title string) (string, error)              // 备份导入文件选择对话框
}

// NewAPI 装配门面。
func NewAPI(dataDir string) (*API, error) {
	reg, err := NewRegistry(dataDir)
	if err != nil {
		return nil, err
	}
	return &API{Reg: reg, lau: NewLauncher(reg), dataDir: dataDir}, nil
}

/* ---------------- 查询 ---------------- */

// ListCategories 分类列表（含内置"全部"语义由前端处理）。
func (a *API) ListCategories() []model.Category {
	return a.Reg.ListCategories()
}

// ToolView 工具视图：数据 + 图标可用性（图标内容经 GetIcon 惰性获取）。
type ToolView struct {
	model.Tool
	HasIcon bool `json:"hasIcon"`
}

// ListTools 分类下工具列表。
func (a *API) ListTools(categoryID string) []ToolView {
	tools := a.Reg.ListTools(categoryID)
	out := make([]ToolView, 0, len(tools))
	for _, t := range tools {
		// 防线：修复历史版本 defaults read 造成的 \uXXXX 乱码名
		t.Name = decodeUniEscapes(t.Name)
		out = append(out, ToolView{Tool: t, HasIcon: a.iconExists(t.ID)})
	}
	return out
}

/* ---------------- 收录（M1-3） ---------------- */

// AddToolByPath 按路径收录工具：自动识别类型/提取图标。
// name/args 可选：name 留空则自动识别（.app 读 CFBundleName，其余取文件名）。
// envVersion 可选：非空表示"采用全局设置"并指定全局版本名；空=系统默认。
// 拖拽收录与手动添加共用此入口。
func (a *API) AddToolByPath(path, categoryID, name, args, envVersion string) (model.Tool, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return model.Tool{}, errors.New("路径为空")
	}
	p = strings.TrimPrefix(p, "file://")
	if _, err := os.Stat(ExpandPath(p)); err != nil {
		return model.Tool{}, errors.New("路径不存在: " + p)
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
	t.EnvVersion = strings.TrimSpace(envVersion) // 空=系统默认；非空=采用全局设置的指定版本
	// 边界修复：拖拽收录时前端在"全部"视图下传空分类，空 categoryId 的工具
	// 在任何具体分类下都不可见——归入第一个分类（默认分类）
	if t.CategoryID == "" {
		if cats := a.Reg.ListCategories(); len(cats) > 0 {
			t.CategoryID = cats[0].ID
		}
	}
	created, err := a.Reg.AddTool(t)
	if err != nil {
		return model.Tool{}, err
	}
	// .app 尝试提取真实图标（失败不阻断收录，前端走类型图标兜底）
	if created.Type == "app" {
		_ = ExtractAppIcon(ExpandPath(created.Path), IconPathFor(a.dataDir, created.ID))
	}
	return created, nil
}

/* ---------------- 编辑 / 删除 ---------------- */

// UpdateTool 更新工具可编辑字段（前端弹层总是传全量字段，直接覆盖；
// 空路径/空分类视为不修改该字段；envVersion 允许显式清空回 "自动"）。
func (a *API) UpdateTool(id, name, path, args, categoryID, envVersion string) (model.Tool, error) {
	out, err := a.Reg.UpdateTool(id, func(t *model.Tool) {
		if name != "" {
			t.Name = name
		}
		if path != "" {
			t.Path = path
		}
		t.Args = args // 允许显式清空
		if categoryID != "" {
			t.CategoryID = categoryID
		}
		t.EnvVersion = strings.TrimSpace(envVersion) // 空=系统默认
	})
	if err != nil {
		return model.Tool{}, err
	}
	// 路径变了则刷新图标
	if path != "" && out.Type == "app" {
		_ = ExtractAppIcon(ExpandPath(out.Path), IconPathFor(a.dataDir, out.ID))
	}
	return out, nil
}

// DeleteTool 删除工具（含图标缓存）。
func (a *API) DeleteTool(id string) error {
	_ = os.Remove(IconPathFor(a.dataDir, id))
	return a.Reg.DeleteTool(id)
}

/* ---------------- 分类 ---------------- */

func (a *API) AddCategory(name string) (model.Category, error)  { return a.Reg.AddCategory(name) }
func (a *API) RenameCategory(id, name string) error             { return a.Reg.RenameCategory(id, name) }
func (a *API) DeleteCategory(id string) error                   { return a.Reg.DeleteCategory(id) }

/* ---------------- 启动 ---------------- */

// Launch 启动工具（进程脱离）。
func (a *API) Launch(toolID string) error { return a.lau.Launch(toolID) }

/* ---------------- 面板显隐 ---------------- */

// SetHideHook 注入面板隐藏回调（main.go 绑定 platform.HideWindowAnimated，
// service 层不直接依赖 platform——与 SetPinHook 同模式）。
func (a *API) SetHideHook(hook func()) { a.hideHook = hook }

// HidePanel 收起面板（Launchpad 式淡出，Esc/启动后收起共用入口）。
func (a *API) HidePanel() {
	if a.hideHook != nil {
		a.hideHook()
	}
}

// SetGlassHook 注入玻璃层深浅切换回调（main.go 绑定 platform.SetGlassDark）。
func (a *API) SetGlassHook(hook func(dark bool) bool) { a.glassHook = hook }

// SetGlassTheme 同步原生玻璃层与前端主题（深色 tint / 清除 tint）。
// 自动模式由前端跟随系统外观后调用；重试逻辑在前端处理。
func (a *API) SetGlassTheme(dark bool) {
	if a.glassHook != nil {
		a.glassHook(dark)
	}
}

/* ---------------- 开机自启（M3-1） ---------------- */

// SetLoginHook 注入登录项注册回调（main.go 绑定 platform.SetLaunchAtLogin）。
func (a *API) SetLoginHook(hook func(bool) error) { a.loginHook = hook }

// SetLaunchAtLogin 开机自启开关：先走系统注册（SMAppService），成功才持久化。
// 失败（如 dev 裸二进制运行）返回错误，前端提示且状态不落库。
func (a *API) SetLaunchAtLogin(enable bool) error {
	if a.loginHook != nil {
		if err := a.loginHook(enable); err != nil {
			return err
		}
	}
	return a.Reg.UpdateSettings(func(s *model.Settings) { s.LaunchOnLogin = enable })
}

/* ---------------- M4 体验增强 ---------------- */

// SetDialogHooks 注入备份导出/导入所需的系统文件对话框（main.go 绑定 Wails DialogManager）。
func (a *API) SetDialogHooks(save func(title, defaultName string) (string, error), open func(title string) (string, error)) {
	a.saveFileHook = save
	a.openFileHook = open
}

// ListRecentTools 最近使用列表（M4-2），按最后启动时间倒序。
func (a *API) ListRecentTools(n int) []ToolView {
	recent := a.Reg.ListRecent(n)
	out := make([]ToolView, 0, len(recent))
	for _, t := range recent {
		out = append(out, ToolView{Tool: t, HasIcon: a.iconExists(t.ID)})
	}
	return out
}

/* ---------------- 图标 ---------------- */

// GetIcon 返回工具图标的 PNG base64（无缓存图标返回空串，前端走类型图标兜底）。
func (a *API) GetIcon(toolID string) string {
	data, err := os.ReadFile(IconPathFor(a.dataDir, toolID))
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func (a *API) iconExists(toolID string) bool {
	_, err := os.Stat(IconPathFor(a.dataDir, toolID))
	return err == nil
}

/* ---------------- 设置 / 主题 ---------------- */

// GetSettings 当前设置。
func (a *API) GetSettings() model.Settings { return a.Reg.GetSettings() }

// SetTheme 主题模式：auto | light | dark（持久化）。
func (a *API) SetTheme(theme string) error {
	return a.Reg.UpdateSettings(func(s *model.Settings) { s.Theme = theme })
}

// SetReduceMotion 减弱动态效果（持久化）。
func (a *API) SetReduceMotion(v bool) error {
	return a.Reg.UpdateSettings(func(s *model.Settings) { s.ReduceMotion = v })
}

// saveVersions 校验并保存一组运行时版本（Java / Python 共用内部逻辑）。
// 过滤名称与路径全空的行；名称必填；路径非空时校验文件存在（~ 自动展开）。
func (a *API) saveVersions(vs []model.RuntimeVersion, label string) ([]model.RuntimeVersion, error) {
	out := make([]model.RuntimeVersion, 0, len(vs))
	for _, v := range vs {
		v.Name = strings.TrimSpace(v.Name)
		v.Path = strings.TrimSpace(v.Path)
		if v.Name == "" && v.Path == "" {
			continue // 空行忽略
		}
		if v.Name == "" {
			return nil, errors.New(label + "存在未填版本名的配置行")
		}
		if v.Path != "" {
			if _, err := os.Stat(ExpandPath(v.Path)); err != nil {
				return nil, errors.New(label + "「" + v.Name + "」路径不存在: " + v.Path)
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// SaveJavaVersions 保存全局 Java 版本列表（设置 → Java 环境，持久化）。
func (a *API) SaveJavaVersions(vs []model.RuntimeVersion) error {
	clean, err := a.saveVersions(vs, "Java")
	if err != nil {
		return err
	}
	return a.Reg.UpdateSettings(func(s *model.Settings) { s.JavaVersions = clean })
}

// SavePythonVersions 保存全局 Python 版本列表（设置 → Python 环境，持久化）。
func (a *API) SavePythonVersions(vs []model.RuntimeVersion) error {
	clean, err := a.saveVersions(vs, "Python")
	if err != nil {
		return err
	}
	return a.Reg.UpdateSettings(func(s *model.Settings) { s.PythonVersions = clean })
}

// SetFontScale 界面字体缩放：standard | large | xlarge（持久化）。
func (a *API) SetFontScale(scale string) error {
	switch scale {
	case "standard", "large", "xlarge":
		return a.Reg.UpdateSettings(func(s *model.Settings) { s.FontScale = scale })
	}
	return errors.New("无效的字体缩放档位: " + scale)
}

// GetAccentColor 系统强调色（任务书 §2.4：跟随系统，失败回退 #0A84FF）。
// 解析 macOS 全局 AppleHighlightColor（如 "0.698039 0.843137 1.000000 Blue"）。
// 注意：高亮色是浅色调（白字对比度不足），亮度超限时压暗到可用区间。
func (a *API) GetAccentColor() string {
	if out, err := exec.Command("defaults", "read", "-g", "AppleHighlightColor").Output(); err == nil {
		f := strings.Fields(strings.TrimSpace(string(out)))
		if len(f) >= 3 {
			r, e1 := strconv.ParseFloat(f[0], 64)
			g, e2 := strconv.ParseFloat(f[1], 64)
			b, e3 := strconv.ParseFloat(f[2], 64)
			if e1 == nil && e2 == nil && e3 == nil {
				// 亮度（HSL lightness）> 0.62 时等比压暗，保持色相
				maxC := math.Max(r, math.Max(g, b))
				minC := math.Min(r, math.Min(g, b))
				if l := (maxC + minC) / 2; l > 0.62 {
					k := 0.62 / l
					r, g, b = r*k, g*k, b*k
				}
				clamp := func(v float64) int { return int(math.Min(v, 1)*255 + 0.5) }
				return fmt.Sprintf("#%02X%02X%02X", clamp(r), clamp(g), clamp(b))
			}
		}
	}
	return "#0A84FF"
}

/* ---------------- helpers ---------------- */

func baseName(p string) string {
	b := filepath.Base(strings.TrimRight(p, "/"))
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// bundleName 读 .app 的 CFBundleName（失败回退目录名去 .app）。
// 用 plutil 而非 defaults read：后者对非 ASCII 输出 \uXXXX 转义（中文乱码根因）。
func bundleName(appPath string) string {
	plist := filepath.Join(appPath, "Contents", "Info")
	if out, err := exec.Command("plutil", "-extract", "CFBundleName", "raw",
		"-o", "-", plist).Output(); err == nil {
		if n := decodeUniEscapes(strings.TrimSpace(string(out))); n != "" {
			return n
		}
	}
	// 兜底：defaults read（含转义解码）
	if out, err := exec.Command("defaults", "read",
		plist, "CFBundleName").Output(); err == nil {
		if n := decodeUniEscapes(strings.TrimSpace(string(out))); n != "" {
			return n
		}
	}
	return decodeUniEscapes(strings.TrimSuffix(filepath.Base(appPath), ".app"))
}

var uniEscapeRe = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

// decodeUniEscapes 还原 defaults 类命令输出的 \uXXXX 转义（中文等 BMP 字符）。
func decodeUniEscapes(s string) string {
	if !strings.Contains(s, `\u`) {
		return s
	}
	return uniEscapeRe.ReplaceAllStringFunc(s, func(m string) string {
		v, err := strconv.ParseUint(m[2:], 16, 32)
		if err != nil {
			return m
		}
		return string(rune(v))
	})
}
