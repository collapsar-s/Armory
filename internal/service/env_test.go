package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"armory/internal/model"
	"armory/internal/store"
)

// TestRuntimeVersionSelection M4-1：按工具指定的版本名选择全局配置的解释器，
// 未指定/未命中/路径失效时回退系统默认。
func TestRuntimeVersionSelection(t *testing.T) {
	dir := t.TempDir()
	store.SetDataDirForTest(dir)
	defer store.SetDataDirForTest("")

	fakeJava := filepath.Join(dir, "fake-java")
	if err := os.WriteFile(fakeJava, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	lau := NewLauncher(reg)

	// 系统默认 = PATH 中的 java（LookPath 解析为绝对路径；找不到时为字面 "java"）
	sysJava, err := exec.LookPath("java")
	if err != nil {
		sysJava = "java"
	}

	// 未配置任何版本 → 系统 java
	if got := lau.javaBin("Java 21"); got != sysJava {
		t.Errorf("未配置时应回退系统 java，得到 %q", got)
	}
	// 配置后命中
	_ = reg.UpdateSettings(func(s *model.Settings) {
		s.JavaVersions = []model.RuntimeVersion{
			{Name: "Java 21", Path: fakeJava},
			{Name: "Java 8", Path: "/nonexistent/java8"},
		}
	})
	if got := lau.javaBin("Java 21"); got != fakeJava {
		t.Errorf("命中版本应用配置路径，得到 %q", got)
	}
	// 命中但路径失效 → 回退
	if got := lau.javaBin("Java 8"); got != sysJava {
		t.Errorf("路径失效应回退系统 java，得到 %q", got)
	}
	// 未指定版本 → 系统默认
	if got := lau.javaBin(""); got != sysJava {
		t.Errorf("未指定版本应用系统 java，得到 %q", got)
	}
}

// TestCommandEnv M4-1：命中全局配置的绝对路径解释器时注入 JAVA_HOME 与 PATH 前置；
// 系统默认解释器不注入（保持与无环境变量时一致）。
func TestCommandEnv(t *testing.T) {
	dir := t.TempDir()
	store.SetDataDirForTest(dir)
	defer store.SetDataDirForTest("")

	fakeJava := filepath.Join(dir, "jdk", "bin", "java")
	if err := os.MkdirAll(filepath.Dir(fakeJava), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeJava, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	lau := NewLauncher(reg)
	_ = reg.UpdateSettings(func(s *model.Settings) {
		s.JavaVersions = []model.RuntimeVersion{{Name: "Java 21", Path: fakeJava}}
	})

	// jar + 命中版本 → JAVA_HOME=.../jdk 且 PATH 前置
	env := lau.commandEnv(model.Tool{Type: "jar", EnvVersion: "Java 21"})
	if env == nil {
		t.Fatal("命中绝对路径解释器时应注入环境变量")
	}
	var javaHome, pathVal string
	foundPathPrefix := false
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "JAVA_HOME="):
			javaHome = strings.TrimPrefix(kv, "JAVA_HOME=")
		case strings.HasPrefix(kv, "PATH="):
			pathVal = kv
			if strings.HasPrefix(strings.TrimPrefix(kv, "PATH="), filepath.Dir(fakeJava)+":") {
				foundPathPrefix = true
			}
		}
	}
	if javaHome != filepath.Dir(filepath.Dir(fakeJava)) {
		t.Errorf("JAVA_HOME 应为 jdk 根目录，得到 %q", javaHome)
	}
	if !foundPathPrefix {
		t.Errorf("PATH 应前置解释器目录，得到 %q", pathVal)
	}

	// bin 类型 → 不注入
	if env := lau.commandEnv(model.Tool{Type: "bin"}); env != nil {
		t.Errorf("bin 类型不应注入环境变量")
	}
	// py + 系统默认 → 不注入
	if env := lau.commandEnv(model.Tool{Type: "py"}); env != nil {
		t.Errorf("系统默认解释器不应注入环境变量")
	}
}

// TestJvmArgsAndLaunchDir jar 工具：JVM 参数（-noverify/-javaagent 等）必须拼在
// -jar 之前；未配置 WorkingDir 时默认以 jar 所在目录为工作目录（相对路径资源可用）。
func TestJvmArgsAndLaunchDir(t *testing.T) {
	dir := t.TempDir()
	store.SetDataDirForTest(dir)
	defer store.SetDataDirForTest("")

	reg, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	lau := NewLauncher(reg)

	jarPath := filepath.Join(dir, "tool.jar")
	tool, err := reg.AddTool(model.Tool{
		Name:    "t",
		Path:    jarPath,
		Type:    "jar",
		Args:    "-a b",
		JvmArgs: `-noverify -javaagent:agent.jar -javaagent:"/path with space/x.jar"`,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, args, err := lau.buildCommand(tool)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-noverify", "-javaagent:agent.jar", "-javaagent:/path with space/x.jar",
		"-jar", jarPath, "-a", "b",
	}
	if len(args) != len(want) {
		t.Fatalf("参数数量不符: got %v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("参数[%d] = %q, want %q（JVM 选项必须在 -jar 之前）", i, args[i], want[i])
		}
	}

	// 工作目录：jar/bin 等默认 = 工具所在目录（与「终端运行」cd 行为一致）；显式 WorkingDir 优先。
	if got := lau.launchDir(tool); got != dir {
		t.Errorf("jar 默认工作目录应为所在目录 %q，得到 %q", dir, got)
	}
	tool.WorkingDir = "/custom/wd"
	if got := lau.launchDir(tool); got != "/custom/wd" {
		t.Errorf("显式 WorkingDir 应优先，得到 %q", got)
	}
	tool.Type = "bin"
	tool.WorkingDir = ""
	if got := lau.launchDir(tool); got != dir {
		t.Errorf("bin 类型默认工作目录应为工具所在目录 %q，得到 %q", dir, got)
	}
}

// TestDetectTypeURL URL 前缀识别为网页工具。
func TestDetectTypeURL(t *testing.T) {
	for _, p := range []string{"https://github.com", "HTTP://x.com", "https://a.com/b?c=1"} {
		if got := DetectType(p); got != "url" {
			t.Errorf("DetectType(%q) = %q, want url", p, got)
		}
	}
	if got := DetectType("/usr/bin/nmap"); got != "bin" {
		t.Errorf("本地路径不应识别为 url: %q", got)
	}
}

// TestBuildTermCommand 终端运行命令：osascript + cd 到工具目录 + 引号转义。
func TestBuildTermCommand(t *testing.T) {
	dir := t.TempDir()
	store.SetDataDirForTest(dir)
	defer store.SetDataDirForTest("")
	reg, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	lau := NewLauncher(reg)
	bin := filepath.Join(dir, "fscan")
	tool := model.Tool{Path: bin, Type: "bin", Args: "-h 127.0.0.1", Term: true}
	name, args, err := lau.buildTermCommand(tool)
	if err != nil {
		t.Fatal(err)
	}
	if name != "osascript" || len(args) != 2 || args[0] != "-e" {
		t.Fatalf("应为 osascript -e，得到 %s %v", name, args)
	}
	script := args[1]
	if !strings.Contains(script, "tell application") || !strings.Contains(script, `cd '`+dir+`'`) {
		t.Errorf("脚本应包含 tell application 与 cd: %s", script)
	}
	// 参数逐个单引号包裹（SplitArgs 切分后 shellQuote）
	if !strings.Contains(script, `'`+bin+`' '-h' '127.0.0.1'`) {
		t.Errorf("脚本应包含引号包裹的工具路径与参数: %s", script)
	}
	// 单引号路径转义：AppleScript 字符串内反斜杠翻倍（%q 语义），osascript
	// 解析后 shell 收到 POSIX 的 '\'' 转义。
	tool2 := model.Tool{Path: filepath.Join(dir, "it's", "tool"), Type: "bin", Term: true}
	if _, args2, _ := lau.buildTermCommand(tool2); !strings.Contains(args2[1], `'\\''`) {
		t.Errorf("路径中的单引号应被转义: %s", args2[1])
	}
}
