package service

import (
	"os"
	"path/filepath"
	"testing"

	"armory/internal/model"
)

// TestJavaBinVersionSelection 验证全局 Java 版本配置对启动路径的真实影响。
func TestJavaBinVersionSelection(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-java")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = reg.UpdateSettings(func(s *model.Settings) {
		s.JavaVersions = []model.RuntimeVersion{{Name: "Java 21", Path: fake}}
	})
	lau := NewLauncher(reg)

	if got := lau.javaBin("Java 21"); got != fake {
		t.Errorf("命中自定义版本应使用配置路径，得到 %q", got)
	}
	if got := lau.javaBin("java21"); got != fake { // 名称匹配忽略大小写
		t.Errorf("版本名忽略大小写，得到 %q", got)
	}
	if got := lau.javaBin("Java 8"); got != "java" {
		t.Errorf("未命中的版本应回退系统默认，得到 %q", got)
	}
	if got := lau.javaBin(""); got != "java" {
		t.Errorf("未指定版本应回退系统默认，得到 %q", got)
	}
	// 配置路径失效 → 回退系统默认
	_ = reg.UpdateSettings(func(s *model.Settings) {
		s.JavaVersions = []model.RuntimeVersion{{Name: "broken", Path: "/nonexistent/java"}}
	})
	if got := lau.javaBin("broken"); got != "java" {
		t.Errorf("配置路径失效应回退系统默认，得到 %q", got)
	}
}

// TestPythonBinFallback 验证 Python 版本选择（语义与 Java 统一：空=系统默认）。
func TestPythonBinFallback(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-python")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	lau := NewLauncher(reg)
	if got := lau.pythonBin(""); got != "python3" {
		t.Errorf("未配置应回退 python3，得到 %q", got)
	}
	_ = reg.UpdateSettings(func(s *model.Settings) {
		s.PythonVersions = []model.RuntimeVersion{
			{Name: "Python 3.12", Path: fake},
			{Name: "Python 3.9", Path: "/nonexistent/py"},
		}
	})
	if got := lau.pythonBin(""); got != "python3" {
		t.Errorf("未指定版本应用系统默认（工具未勾选全局设置），得到 %q", got)
	}
	if got := lau.pythonBin("Python 3.12"); got != fake {
		t.Errorf("命中指定版本应用配置路径，得到 %q", got)
	}
	if got := lau.pythonBin("Python 3.9"); got != "python3" {
		t.Errorf("命中但路径失效应回退，得到 %q", got)
	}
}

// TestSaveVersionsValidation 验证保存校验：空行过滤、缺名报错、路径校验。
func TestSaveVersionsValidation(t *testing.T) {
	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &API{Reg: reg}

	// 空行被过滤
	clean, err := a.saveVersions([]model.RuntimeVersion{{Name: "", Path: ""}}, "Java")
	if err != nil || len(clean) != 0 {
		t.Errorf("全空行应被过滤, clean=%v err=%v", clean, err)
	}
	// 有路径没名称 → 报错
	if _, err := a.saveVersions([]model.RuntimeVersion{{Name: "", Path: "/usr/bin/java"}}, "Java"); err == nil {
		t.Error("缺版本名应报错")
	}
	// 路径不存在 → 报错
	if _, err := a.saveVersions([]model.RuntimeVersion{{Name: "X", Path: "/nonexistent"}}, "Java"); err == nil {
		t.Error("路径不存在应报错")
	}
}
