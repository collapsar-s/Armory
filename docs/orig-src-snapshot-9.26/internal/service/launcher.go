package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"armory/internal/model"
)

// Launcher 工具启动服务（M1-4）。
// 进程语义：启动即脱离（Setpgid + 独立进程组），关闭 Armory 不影响已启动工具。
type Launcher struct {
	reg *Registry
}

func NewLauncher(reg *Registry) *Launcher {
	return &Launcher{reg: reg}
}

// DetectType 依据扩展名识别工具类型（任务书 §1.3 F1）。
func DetectType(path string) string {
	p := strings.ToLower(path)
	switch {
	case strings.HasSuffix(p, ".app"):
		return "app"
	case strings.HasSuffix(p, ".jar"):
		return "jar"
	case strings.HasSuffix(p, ".py"):
		return "py"
	case strings.HasSuffix(p, ".sh"), strings.HasSuffix(p, ".command"), strings.HasSuffix(p, ".zsh"):
		return "sh"
	default:
		return "bin"
	}
}

// Launch 启动指定工具。返回错误时工具未成功拉起。
func (l *Launcher) Launch(toolID string) error {
	var t model.Tool
	found := false
	for _, tool := range l.reg.ListTools("") {
		if tool.ID == toolID {
			t = tool
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	if _, err := os.Stat(ExpandPath(t.Path)); err != nil {
		return fmt.Errorf("路径不存在: %s", t.Path)
	}

	args := SplitArgs(t.Args)
	var cmd *exec.Cmd
	p := ExpandPath(t.Path)

	switch t.Type {
	case "app":
		// .app 经 LaunchServices 打开；参数需走 --args
		base := []string{p}
		if len(args) > 0 {
			cmd = exec.Command("open", append(base, append([]string{"--args"}, args...)...)...)
		} else {
			cmd = exec.Command("open", base...)
		}
	case "jar":
		bin := l.javaBin(t.EnvVersion)
		cmd = exec.Command(bin, append([]string{"-jar", p}, args...)...)
		cmd.Env = javaEnv(bin) // JAVA_HOME + PATH 前置（部分工具认环境变量才认版本）
	case "py":
		bin := l.pythonBin(t.EnvVersion)
		cmd = exec.Command(bin, append([]string{p}, args...)...)
		cmd.Env = prependPathEnv(bin)
	case "sh":
		shell := l.reg.GetSettings().Shell
		if shell == "" {
			shell = "/bin/zsh"
		}
		cmd = exec.Command(shell, append([]string{p}, args...)...)
	default: // bin
		cmd = exec.Command(p, args...)
	}

	// 脱离父进程：独立进程组 + 不继承 stdio，Armory 退出后工具继续存活
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}
	go func() { _ = cmd.Wait() }() // 回收子进程防僵尸，不阻塞主流程

	return l.reg.MarkRun(toolID)
}

// normVerName 版本名归一化：去全部空格 + 转小写（"Java 21" / "java21" 互通）。
func normVerName(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "")
}

// findVersion 在版本列表里按名称查找（归一化后比较）。
func findVersion(versions []model.RuntimeVersion, name string) *model.RuntimeVersion {
	key := normVerName(name)
	if key == "" {
		return nil
	}
	for i := range versions {
		if normVerName(versions[i].Name) == key {
			return &versions[i]
		}
	}
	return nil
}

// resolveVersionPath 版本命中且路径有效 → 返回展开后的绝对路径；否则空串。
func resolveVersionPath(v *model.RuntimeVersion) string {
	if v == nil || v.Path == "" {
		return ""
	}
	if _, err := os.Stat(ExpandPath(v.Path)); err != nil {
		return "" // 配置失效回退系统默认（可用性优先）
	}
	return ExpandPath(v.Path)
}

// javaBin 依据工具指定的版本名（EnvVersion，用户自定义，如 "Java 8"）
// 查找全局 Java 版本列表（设置 → Java 环境）。未指定/未命中/路径失效 → 系统 java。
func (l *Launcher) javaBin(envVersion string) string {
	if p := resolveVersionPath(findVersion(l.reg.GetSettings().JavaVersions, envVersion)); p != "" {
		return p
	}
	return "java"
}

// pythonBin 依据工具指定的版本名查全局 Python 版本列表（设置 → Python 环境）。
// 与 javaBin 语义统一：未指定/未命中/路径失效 → 系统 python3。
func (l *Launcher) pythonBin(envVersion string) string {
	if p := resolveVersionPath(findVersion(l.reg.GetSettings().PythonVersions, envVersion)); p != "" {
		return p
	}
	return "python3"
}

// SplitArgs 拆分启动参数。M1 采用空格分词（不支持引号转义，见任务书范围）。
func SplitArgs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Fields(s)
}

// javaEnv Java 启动环境（M4-1）：bin 形如 ".../Home/bin/java" 时
// 注入 JAVA_HOME=.../Home，并把 bin 目录前置到 PATH；系统 java（无绝对路径）不注入。
func javaEnv(bin string) []string {
	env := os.Environ()
	if !filepath.IsAbs(bin) {
		return env
	}
	binDir := filepath.Dir(bin)
	home := filepath.Dir(binDir) // Home 目录（bin 的上一级）
	env = append(env, "JAVA_HOME="+home)
	return prependPath(env, binDir)
}

// prependPathEnv Python 启动环境（M4-1）：解释器所在目录前置到 PATH。
func prependPathEnv(bin string) []string {
	env := os.Environ()
	if !filepath.IsAbs(bin) {
		return env
	}
	return prependPath(env, filepath.Dir(bin))
}

func prependPath(env []string, dir string) []string {
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + dir + ":" + strings.TrimPrefix(kv, "PATH=")
			return env
		}
	}
	return append(env, "PATH="+dir)
}
