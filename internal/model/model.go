// Package model 定义 Armory 的核心数据模型。
//
// 数据持久化到 ~/.armory/tools.json，字段名与 JSON tag 与历史版本保持完全兼容，
// 保证升级后能直接读取旧数据。
package model

// Tool 表示一个被收录的可执行工具（.app / .jar / .py / .sh / 二进制）。
type Tool struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Args       string `json:"args,omitempty"`
	// JvmArgs 仅 jar 工具使用：JVM 选项（-noverify / -javaagent:… 等），
	// 拼在 -jar 之前；Args 是 -jar 之后的程序参数。
	JvmArgs string `json:"jvmArgs,omitempty"`
	// Order 同分组内拖拽排序用的序号（0=未排序，保持收录顺序）。
	Order      float64 `json:"order,omitempty"`
	Tags       []string `json:"tags,omitempty"` // 标签：多对多归类（跨分组检索用）
	WorkingDir string `json:"workingDir,omitempty"`
	// Term 为真时，CLI 类工具（bin/sh/py）在系统终端新窗口中运行（Spear openterm 式）。
	Term       bool   `json:"term,omitempty"`
	CategoryID string `json:"categoryId"`
	IconPath   string `json:"iconPath,omitempty"`
	HasIcon    bool   `json:"hasIcon"`
	Type       string `json:"type"` // app | jar | py | sh | bin
	EnvVersion string `json:"envVersion,omitempty"`
	Notes      string `json:"notes,omitempty"`
	RunCount   int    `json:"runCount,omitempty"`
	LastRunAt  string `json:"lastRunAt,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// Category 表示侧边栏中的一个工具分类。
// 层级模型（2026-11-08 起统一）：主分组（parentId 空，侧栏一层）→ 分组（页内分节）。
// Kind 字段为历史遗留（"title"=分组标题），加载时已归一为 dir，不再产生新值。
type Category struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Order    float64 `json:"order"`
	ParentID string  `json:"parentId,omitempty"`
	Kind     string  `json:"kind,omitempty"` // 已废弃：仅兼容旧数据，读入时归一
}

// RuntimeVersion 表示一个可用的运行时（Java / Python）版本配置。
type RuntimeVersion struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Settings 表示全局设置。
type Settings struct {
	Theme          string           `json:"theme,omitempty"` // auto | light | dark
	ReduceMotion   bool             `json:"reduceMotion,omitempty"`
	AlwaysOnTop    bool             `json:"alwaysOnTop,omitempty"`
	FontScale      string           `json:"fontScale,omitempty"` // standard | large | xlarge
	LaunchOnLogin  bool             `json:"launchOnLogin,omitempty"`
	Hotkey         string           `json:"hotkey,omitempty"`
	JavaVersions   []RuntimeVersion `json:"javaVersions,omitempty"`
	PythonVersions []RuntimeVersion `json:"pythonVersions,omitempty"`
}

// RegistryData 是持久化到 ~/.armory/tools.json 的顶层结构。
type RegistryData struct {
	Tools      []Tool     `json:"Tools"`
	Categories []Category `json:"Categories"`
	Settings   Settings   `json:"Settings"`
}
