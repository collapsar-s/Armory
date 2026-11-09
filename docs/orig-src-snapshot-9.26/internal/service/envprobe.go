package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"armory/internal/model"
)

// 本机运行时扫描（M4-1 增强）：自动发现已安装的 JDK / Python，
// 结果直接作为版本列表候选填入设置（用户确认后保存）。

// DetectJavaVersions 扫描本机 JDK。
// 首选 /usr/libexec/java_home -V（macOS 官方入口，覆盖 JDK/JRE 全部安装）；
// 失败则退化为扫 /Library/Java/JavaVirtualMachines 目录。
func (a *API) DetectJavaVersions() []model.RuntimeVersion {
	if out, err := exec.Command("/usr/libexec/java_home", "-V").CombinedOutput(); err == nil {
		var vs []model.RuntimeVersion
		for _, line := range strings.Split(string(out), "\n") {
			// 行样例：  21.0.5 (arm64) "/Library/Java/JavaVirtualMachines/jdk-21/Contents/Home"
			i := strings.Index(line, "\"")
			j := strings.LastIndex(line, "\"")
			if i < 0 || j <= i {
				continue
			}
			home := strings.TrimSpace(line[i+1 : j])
			bin := filepath.Join(home, "bin", "java")
			if !fileExists(bin) {
				continue
			}
			ver := strings.Fields(strings.TrimSpace(line[:i]))
			name := "Java"
			if len(ver) > 0 {
				name = "Java " + majorVersion(ver[0])
			}
			vs = append(vs, model.RuntimeVersion{Name: name, Path: bin})
		}
		if len(vs) > 0 {
			return vs
		}
	}
	// 兜底：直接扫目录
	var vs []model.RuntimeVersion
	roots := []string{"/Library/Java/JavaVirtualMachines", filepath.Join(homeDir(), "Library/Java/JavaVirtualMachines")}
	for _, root := range roots {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			bin := filepath.Join(root, e.Name(), "Contents", "Home", "bin", "java")
			if fileExists(bin) {
				vs = append(vs, model.RuntimeVersion{Name: "Java " + e.Name(), Path: bin})
			}
		}
	}
	return vs
}

// DetectPythonVersions 扫描常见位置的 Python 解释器，并实际执行 --version 取准确版本号。
func (a *API) DetectPythonVersions() []model.RuntimeVersion {
	seen := map[string]bool{}
	var candidates []string
	add := func(p string) {
		if p != "" && fileExists(p) && !seen[p] {
			seen[p] = true
			candidates = append(candidates, p)
		}
	}
	add("/usr/bin/python3")
	add("/usr/local/bin/python3")
	if hb, _ := exec.LookPath("python3"); hb != "" {
		add(hb)
	}
	globs := []string{
		"/opt/homebrew/bin/python3*",
		"/usr/local/bin/python3*",
		filepath.Join(homeDir(), ".pyenv", "versions", "*", "bin", "python3"),
	}
	for _, g := range globs {
		matches, _ := filepath.Glob(g)
		for _, m := range matches {
			add(m)
		}
	}

	var vs []model.RuntimeVersion
	for _, bin := range candidates {
		out, err := exec.Command(bin, "--version").CombinedOutput()
		ver := strings.TrimSpace(string(out)) // "Python 3.12.7"
		if err != nil && ver == "" {
			continue
		}
		name := ver
		if strings.HasPrefix(name, "Python ") {
			name = "Python " + majorVersion(strings.TrimPrefix(name, "Python "))
		}
		if name == "" || name == "Python" {
			continue
		}
		vs = append(vs, model.RuntimeVersion{Name: name, Path: bin})
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i].Name > vs[j].Name })
	return vs
}

// majorVersion 取主版本号："21.0.5" → "21"；"3.12.7" → "3.12"（两位更易读）。
func majorVersion(v string) string {
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) == 0 {
		return v
	}
	if len(parts) >= 2 {
		if _, err := strconv.Atoi(parts[0]); err == nil {
			if _, err2 := strconv.Atoi(parts[1]); err2 == nil {
				return parts[0] + "." + parts[1]
			}
		}
	}
	if _, err := strconv.Atoi(parts[0]); err == nil {
		return parts[0]
	}
	return v
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}
