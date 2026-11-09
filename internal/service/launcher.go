package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"armory/internal/model"
)

// Launcher 负责根据工具类型与运行时版本拼装命令并启动进程。
type Launcher struct {
	reg *Registry
}

// NewLauncher 创建启动器。
func NewLauncher(reg *Registry) *Launcher { return &Launcher{reg: reg} }

// Launch 启动指定工具。启动成功后由 Registry 记录一次运行。
func (l *Launcher) Launch(id string) error {
	t, err := l.reg.GetTool(id)
	if err != nil {
		return err
	}
	// URL 类型是远程/网页地址，不做本地路径校验。
	if t.Type != "url" {
		if _, err := os.Stat(ExpandPath(t.Path)); err != nil {
			return fmt.Errorf("路径不存在: %s", t.Path)
		}
	}
	var name string
	var args []string
	// 勾选「在终端中运行」的 CLI 工具：新开终端窗口执行（输出可见）。
	if t.Term && (t.Type == "bin" || t.Type == "sh" || t.Type == "py") {
		name, args, err = l.buildTermCommand(t)
	} else {
		name, args, err = l.buildCommand(t)
	}
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	if t.Type != "url" {
		cmd.Dir = l.launchDir(t)
	}
	// 运行环境注入（M4-1）：jar/py 使用全局配置的解释器时，
	// 同步注入 JAVA_HOME / PATH 前置——部分工具认环境变量才认版本。
	if env := l.commandEnv(t); env != nil {
		cmd.Env = env
	}
	if err := cmd.Start(); err != nil {
		return friendlyStartError(err)
	}
	// 后台等待回收，避免僵尸进程。
	go func() { _ = cmd.Wait() }()
	// 运行记录落盘失败不应影响"已成功启动"的事实，但要向上返回。
	return l.reg.MarkRun(id)
}

// launchDir 进程工作目录：显式配置的 WorkingDir 优先；否则按类型取默认——
// jar/bin/sh/py 都用工具所在目录：-javaagent 相对路径、脚本自带的资源/配置文件
// 都能按相对路径找到（与「终端运行」模式 cd 到工具目录的行为一致）。
func (l *Launcher) launchDir(t model.Tool) string {
	if t.WorkingDir != "" {
		return ExpandPath(t.WorkingDir)
	}
	switch t.Type {
	case "jar", "bin", "sh", "py":
		return filepath.Dir(ExpandPath(t.Path))
	}
	return ""
}

// buildTermCommand 构造「在终端中运行」命令（CLI 工具，Spear openterm 式）：
// 新开 Terminal.app 窗口，cd 到工具目录并执行（命令结束后窗口保留，交互工具可用）。
func (l *Launcher) buildTermCommand(t model.Tool) (string, []string, error) {
	bin := ExpandPath(t.Path)
	dir := filepath.Dir(bin)
	parts := []string{}
	switch t.Type {
	case "sh":
		parts = append(parts, "/bin/sh", bin)
	case "py":
		parts = append(parts, l.pythonBin(t.EnvVersion), bin)
	default:
		parts = append(parts, bin)
	}
	parts = append(parts, SplitArgs(t.Args)...)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, shellQuote(p))
	}
	script := "cd " + shellQuote(dir) + " && " + strings.Join(quoted, " ")
	apple := fmt.Sprintf("tell application \"Terminal\"\nactivate\ndo script %q\nend tell", script)
	return "osascript", []string{"-e", apple}, nil
}

// shellQuote POSIX 单引号包裹；单引号本身按 '\'' 转义。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// friendlyStartError 把底层 exec 错误翻译成可操作的提示：
// 可执行文件缺失（如 PATH 中无 java/python3）时，报错应告诉用户去设置里配置，
// 而不是抛出 exec: "java": executable file not found in $PATH 这类原始信息。
func friendlyStartError(err error) error {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return fmt.Errorf("启动失败: 未找到 %s，请确认已安装；jar/py 工具可在「设置 → 运行环境」中配置版本", execErr.Name)
	}
	return fmt.Errorf("启动失败: %w", err)
}

// buildCommand 根据工具类型返回可执行程序名与参数数组。
func (l *Launcher) buildCommand(t model.Tool) (string, []string, error) {
	path := ExpandPath(t.Path)
	userArgs := SplitArgs(t.Args)

	switch t.Type {
	case "app":
		// .app 经 LaunchServices 打开；参数需走 --args（与原版一致，不用 -a）。
		if len(userArgs) > 0 {
			return "open", append([]string{path, "--args"}, userArgs...), nil
		}
		return "open", []string{path}, nil
	case "url":
		// 在线服务/网页工具：系统默认浏览器打开（支持 http/https）。
		return "open", []string{path}, nil
	case "jar":
		// JVM 选项（-noverify / -javaagent:… 等）必须拼在 -jar 之前；
		// Args 是 -jar 之后传给程序主类的参数，两者不可混。
		argv := []string{"-jar", path}
		if jvm := SplitArgs(t.JvmArgs); len(jvm) > 0 {
			argv = append(jvm, argv...)
		}
		return l.javaBin(t.EnvVersion), append(argv, userArgs...), nil
	case "py":
		return l.pythonBin(t.EnvVersion), append([]string{path}, userArgs...), nil
	case "sh":
		return "/bin/sh", append([]string{path}, userArgs...), nil
	default: // bin 或其它可执行文件
		return path, userArgs, nil
	}
}

// DetectType 根据扩展名判断工具类型（.command/.zsh 也算 shell 脚本；URL 前缀算网页工具）。
func DetectType(path string) string {
	lower := strings.ToLower(strings.TrimSpace(path))
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return "url"
	}
	switch {
	case strings.HasSuffix(lower, ".app"):
		return "app"
	case strings.HasSuffix(lower, ".jar"):
		return "jar"
	case strings.HasSuffix(lower, ".py"):
		return "py"
	case strings.HasSuffix(lower, ".sh"), strings.HasSuffix(lower, ".command"), strings.HasSuffix(lower, ".zsh"):
		return "sh"
	default:
		return "bin"
	}
}

// SplitArgs 将参数字符串按 shell 风格切分为参数数组（支持单双引号）。
func SplitArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	var quote rune
	has := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			has = true
		case r == ' ' || r == '\t':
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// javaBin 返回指定版本名对应的 java 可执行文件；未指定或未匹配则用系统默认。
// 兜底链（与原版一致）：配置版本 → PATH 中的 java → /usr/libexec/java_home 定位的 JDK。
func (l *Launcher) javaBin(version string) string {
	if version != "" {
		for _, v := range l.reg.GetSettings().JavaVersions {
			if v.Name == version && fileExists(v.Path) {
				return v.Path
			}
		}
	}
	if p := resolveVersionPath("java"); p != "" {
		return p
	}
	// macOS 无 PATH java 时，用系统 java_home 工具定位 JDK。
	if out, err := exec.Command("/usr/libexec/java_home").Output(); err == nil {
		home := strings.TrimSpace(string(out))
		if home != "" {
			if bin := filepath.Join(home, "bin", "java"); fileExists(bin) {
				return bin
			}
		}
	}
	return "java"
}

// pythonBin 返回指定版本名对应的 python 可执行文件；未指定或未匹配则用系统默认。
// 兜底链（与原版一致）：配置版本 → PATH 中的 python3/python → /usr/local/bin/python3。
func (l *Launcher) pythonBin(version string) string {
	if version != "" {
		for _, v := range l.reg.GetSettings().PythonVersions {
			if v.Name == version && fileExists(v.Path) {
				return v.Path
			}
		}
	}
	for _, cand := range []string{"python3", "python"} {
		if p := resolveVersionPath(cand); p != "" {
			return p
		}
	}
	if fileExists("/usr/local/bin/python3") {
		return "/usr/local/bin/python3"
	}
	return "python3"
}

// resolveVersionPath 在 PATH 中查找可执行文件绝对路径，找不到返回空串。
func resolveVersionPath(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// commandEnv 运行环境注入（M4-1）：仅当 jar/py 命中了全局配置的版本
// （且路径存在）时注入 JAVA_HOME / PATH 前置；系统默认解释器不注入，
// 保持与不设置任何环境变量时行为一致。
func (l *Launcher) commandEnv(t model.Tool) []string {
	var versions []model.RuntimeVersion
	switch t.Type {
	case "jar":
		versions = l.reg.GetSettings().JavaVersions
	case "py":
		versions = l.reg.GetSettings().PythonVersions
	default:
		return nil
	}
	name := t.EnvVersion
	if name == "" {
		return nil
	}
	var bin string
	for _, v := range versions {
		if v.Name == name && fileExists(v.Path) {
			bin = v.Path
			break
		}
	}
	if bin == "" {
		return nil
	}
	binDir := filepath.Dir(bin)
	env := append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	if t.Type == "jar" {
		// .../Home/bin/java → JAVA_HOME=.../Home
		env = append(env, "JAVA_HOME="+filepath.Dir(binDir))
	}
	return env
}
