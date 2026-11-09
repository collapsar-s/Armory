// Package model 定义 Armory 的全部数据结构。
// 规则：新字段必须 omitempty 且可缺省，保证 tools.json 向后兼容（开发计划 §9-5）。
package model

// Tool 一个被收录的工具。
type Tool struct {
	ID          string `json:"id"`                    // UUID
	Name        string `json:"name"`                  // 显示名
	Path        string `json:"path"`                  // 可执行文件/应用路径（~ 可展开）
	Args        string `json:"args,omitempty"`        // 启动参数
	Type        string `json:"type"`                  // app | jar | py | sh | bin
	CategoryID  string `json:"categoryId"`            // 所属分类 ID
	IconPath    string `json:"iconPath,omitempty"`    // 图标缓存路径（.app 提取后）
	EnvVersion  string `json:"envVersion,omitempty"`  // (M4) 指定的 JDK/Python 版本，空=系统默认
	CreatedAt   string `json:"createdAt"`             // RFC3339
	LastRunAt   string `json:"lastRunAt,omitempty"`   // RFC3339
	RunCount    int    `json:"runCount,omitempty"`    // 启动次数
}

// Category 分类。ID "all" 为内置虚拟分类（全部工具），不持久化。
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"` // SF Symbols 名，空则用默认
	Order int    `json:"order"`
}

// RuntimeVersion 一个自定义运行时版本（Java / Python 通用）。
// Name 为用户自定义版本号（如 "Java 8"、"GraalVM 21"、"Python 3.12"），
// Path 为解释器/虚拟机可执行文件路径（~ 自动展开）。
type RuntimeVersion struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Settings 应用设置。
type Settings struct {
	Hotkey        string `json:"hotkey"`                  // 呼出键，默认 "alt+space"
	LaunchOnLogin bool   `json:"launchOnLogin,omitempty"` // 开机自启（M3-1）
	ReduceMotion  bool   `json:"reduceMotion,omitempty"`  // 减弱动态效果
	Shell         string `json:"shell,omitempty"`         // sh/py 工具使用的 shell，空=/bin/zsh
	Theme         string `json:"theme,omitempty"`         // auto | light | dark

	// 全局运行环境（对标 Spear JavaPaths，升级为自定义版本列表）：
	// Java 与 Python 分开管理；版本名称用户自定义，不写死。
	JavaVersions   []RuntimeVersion `json:"javaVersions,omitempty"`
	PythonVersions []RuntimeVersion `json:"pythonVersions,omitempty"`

	// 界面字体缩放：standard(1.0) | large(1.1，默认) | xlarge(1.2)。
	// 空值 = large（比初版默认字号大一号，2026-09-26 用户设定）。
	FontScale string `json:"fontScale,omitempty"`
}

// HistoryEntry 单次启动记录（M4-2，SQLite）。
type HistoryEntry struct {
	ToolID    string `json:"toolId"`
	ToolName  string `json:"toolName"`
	RunAt     string `json:"runAt"`
	ExitOK    bool   `json:"exitOk"`
}
