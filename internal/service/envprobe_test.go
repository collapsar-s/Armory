package service

import (
	"strings"
	"testing"

	"armory/internal/model"
)

func TestIsPythonBinName(t *testing.T) {
	yes := []string{"python", "python2", "python2.7", "python3", "python3.11", "python3.13"}
	no := []string{"python3.11-config", "python2.7-gdb.py", "python-config", "python-ws", "idle3", "pydoc3", ""}
	for _, s := range yes {
		if !pythonBinNameRE.MatchString(s) {
			t.Errorf("%q 应命中白名单", s)
		}
	}
	for _, s := range no {
		if pythonBinNameRE.MatchString(s) {
			t.Errorf("%q 不应命中白名单", s)
		}
	}
}

func TestIsShimPath(t *testing.T) {
	if !isShimPath("/Users/x/.pyenv/shims/python3") {
		t.Error("pyenv shims 路径应被跳过")
	}
	if isShimPath("/Users/x/.pyenv/versions/3.11.15/bin/python3") {
		t.Error("pyenv versions 路径不应被跳过")
	}
}

func TestPythonPathRank(t *testing.T) {
	// python3 最规范 > python（2.x 目录常只有它）> python3.x > python2.x
	if pythonPathRank("python3") >= pythonPathRank("python") {
		t.Error("python3 应优先于 python")
	}
	if pythonPathRank("python") >= pythonPathRank("python3.8") {
		t.Error("python 应优先于 python3.8")
	}
	if pythonPathRank("python3.8") >= pythonPathRank("python2.7") {
		t.Error("python3.8 应优先于 python2.7")
	}
}

func TestPythonVersionFromOutput(t *testing.T) {
	cases := []struct {
		out, want string
	}{
		{"Python 3.11.15\n", "3.11.15"},
		{"Python 2.7.18\n", "2.7.18"}, // python2 打到 stderr，CombinedOutput 同样捕获
		{"Python 3.9.6 (main, Nov 20 2024, 12:00:00)\n[Clang 16.0.0]\n", "3.9.6"},
		{"Warning: something noisy\nPython 3.12.7 :: Conda base\n", "3.12.7"}, // 前置噪音行
		{"OpenJDK Runtime Environment (build 21.0.5)\n", ""},                  // 非 Python 输出
		{"Python 3.x.y\n", ""},   // 版本号段非纯数字
		{"Python\n", ""},         // 无版本号段
		{"", ""},                 // 空
	}
	for _, c := range cases {
		if got := pythonVersionFromOutput(c.out); got != c.want {
			t.Errorf("pythonVersionFromOutput(%q) = %q, want %q", c.out, got, c.want)
		}
	}
}

func TestDedupeProbeEntries(t *testing.T) {
	got := dedupeProbeEntries([]probeEntry{
		{name: "Java 1.8", full: "1.8.0_371", path: "/a"},
		{name: "Java 1.8", full: "1.8.0_291", path: "/a"}, // 同路径，去重
		{name: "Java 1.8", full: "1.8.0_65", path: "/b"},  // 不同路径，必须保留
	})
	if len(got) != 2 || got[0].path != "/a" || got[1].path != "/b" {
		t.Errorf("路径去重结果错误: %+v", got)
	}
}

func TestResolveNameCollisions(t *testing.T) {
	vs := resolveNameCollisions([]probeEntry{
		{name: "Java 26.0", full: "26.0.2", path: "/26"},
		{name: "Java 1.8", full: "1.8.0_371", path: "/jdk-1.8"},
		{name: "Java 1.8", full: "1.8.0_291", path: "/jdk1.8.0_291"},
		{name: "Java 1.8", full: "1.8.0_65", path: "/jdk1.8.0_65"},
	})
	want := []string{"Java 26.0", "Java 1.8.0_371", "Java 1.8.0_291", "Java 1.8.0_65"}
	for i, w := range want {
		if vs[i].Name != w {
			t.Errorf("命名[%d] = %q, want %q", i, vs[i].Name, w)
		}
	}
	// 同版本装多份（全版本号仍撞名）→ " #N"
	vs = resolveNameCollisions([]probeEntry{
		{name: "Java 1.8", full: "1.8.0_65", path: "/x"},
		{name: "Java 1.8", full: "1.8.0_65", path: "/y"},
	})
	if vs[0].Name != "Java 1.8.0_65" || vs[1].Name != "Java 1.8.0_65 #2" {
		t.Errorf("同版本多份命名错误: %q, %q", vs[0].Name, vs[1].Name)
	}
	// 无全版本号（目录兜底）且同名 → " #N"
	vs = resolveNameCollisions([]probeEntry{
		{name: "Java jdk-21.jdk", path: "/Library/J/jdk-21.jdk"},
		{name: "Java jdk-21.jdk", path: "/Users/x/J/jdk-21.jdk"},
	})
	if vs[0].Name != "Java jdk-21.jdk" || vs[1].Name != "Java jdk-21.jdk #2" {
		t.Errorf("兜底同名命名错误: %q, %q", vs[0].Name, vs[1].Name)
	}
}

func TestSortRuntimeVersionsDesc(t *testing.T) {
	vs := []model.RuntimeVersion{
		{Name: "Python 3.8", Path: "a"},
		{Name: "Python 3.11", Path: "b"}, // 字符串排序会排到 3.9 后，数值排序必须在前
		{Name: "Python 2.7", Path: "c"},
		{Name: "Python 3.9", Path: "d"},
		{Name: "Java 1.8.0_291", Path: "e"}, // 全版本号名：按 1.8 归组
		{Name: "Java 26", Path: "f"},
		{Name: "Java 1.8.0_65", Path: "g"},
		{Name: "Java 1.8.0_371", Path: "i"},
		{Name: "Java temurin-21.jdk", Path: "h"}, // 解析失败排最后
	}
	sortRuntimeVersionsDesc(vs)
	want := []string{
		"Java 26", "Python 3.11", "Python 3.9", "Python 3.8", "Python 2.7",
		"Java 1.8.0_371", "Java 1.8.0_291", "Java 1.8.0_65", "Java temurin-21.jdk",
	}
	for i, w := range want {
		if vs[i].Name != w {
			t.Errorf("排序[%d] = %q, want %q", i, vs[i].Name, w)
		}
	}
}

func TestParseVersionKey(t *testing.T) {
	if k, ok := parseVersionKey("Python 3.11"); !ok || k[0] != 3 || k[1] != 11 {
		t.Errorf("parseVersionKey(Python 3.11) = %v, %v", k, ok)
	}
	if k, ok := parseVersionKey("Java 26"); !ok || k[0] != 26 || k[1] != 0 {
		t.Errorf("parseVersionKey(Java 26) = %v, %v", k, ok)
	}
	if k, ok := parseVersionKey("Java 1.8.0_371"); !ok || k[0] != 1 || k[1] != 8 {
		t.Errorf("parseVersionKey(Java 1.8.0_371) = %v, %v", k, ok)
	}
	if _, ok := parseVersionKey("Java temurin-21.jdk"); ok {
		t.Error("非版本名应解析失败")
	}
	if _, ok := parseVersionKey("Python"); ok {
		t.Error("缺版本段应解析失败")
	}
}

func TestJavaHomeLine(t *testing.T) {
	// 旧格式：路径带引号
	home, ver, ok := javaHomeLine(`  21.0.5 (arm64) "/Library/Java/JavaVirtualMachines/jdk-21/Contents/Home"`)
	if !ok || home != "/Library/Java/JavaVirtualMachines/jdk-21/Contents/Home" || ver != "21.0.5" {
		t.Errorf("旧格式解析错误: %q %q %v", home, ver, ok)
	}
	// macOS 26 新格式：路径裸在行尾
	home, ver, ok = javaHomeLine(`    26.0.2 (arm64) "Oracle Corporation" - "Java SE 26.0.2" /Library/Java/JavaVirtualMachines/jdk-26.jdk/Contents/Home`)
	if !ok || home != "/Library/Java/JavaVirtualMachines/jdk-26.jdk/Contents/Home" || ver != "26.0.2" {
		t.Errorf("新格式解析错误: %q %q %v", home, ver, ok)
	}
	// 末尾裸的默认 JVM 路径行（单字段）与非 JVM 行应排除
	for _, line := range []string{
		"/Library/Java/JavaVirtualMachines/jdk-26.jdk/Contents/Home",
		"Matching Java Virtual Machines (7):",
		"",
	} {
		if _, _, ok := javaHomeLine(line); ok {
			t.Errorf("应排除行: %q", line)
		}
	}
}

// TestDetectPythonScanInvariants 真机扫描的通用不变量（不绑定具体机器装了哪些版本）。
func TestDetectPythonScanInvariants(t *testing.T) {
	vs := DetectPythonVersions()
	seen := map[string]bool{}
	for _, v := range vs {
		fields := strings.Fields(v.Name)
		if len(fields) != 2 || fields[0] != "Python" {
			t.Errorf("名字格式异常: %q", v.Name)
			continue
		}
		if _, ok := parseVersionKey(v.Name); !ok {
			t.Errorf("名字版本段无法解析: %q", v.Name)
		}
		if seen[v.Name] {
			t.Errorf("重名条目: %q", v.Name)
		}
		seen[v.Name] = true
		if strings.Contains(v.Path, "/shims/") {
			t.Errorf("shim 混入扫描结果: %q → %q", v.Name, v.Path)
		}
		if !fileExists(v.Path) {
			t.Errorf("路径不存在: %q → %q", v.Name, v.Path)
		}
	}
	for i := 1; i < len(vs); i++ {
		a, aok := parseVersionKey(vs[i-1].Name)
		b, _ := parseVersionKey(vs[i].Name)
		if aok && a[0] < b[0] || (aok && a[0] == b[0] && a[1] < b[1]) {
			t.Errorf("未按版本数值降序: %q 排在 %q 前", vs[i-1].Name, vs[i].Name)
		}
	}
}

// TestDetectJavaScanUniqueAndSorted 真机 Java 扫描：无重名、可解析名字按版本降序。
func TestDetectJavaScanUniqueAndSorted(t *testing.T) {
	vs := DetectJavaVersions()
	seen := map[string]bool{}
	for _, v := range vs {
		if seen[v.Name] {
			t.Errorf("重名条目: %q", v.Name)
		}
		seen[v.Name] = true
		if !fileExists(v.Path) {
			t.Errorf("路径不存在: %q → %q", v.Name, v.Path)
		}
	}
	for i := 1; i < len(vs); i++ {
		a, aok := parseVersionKey(vs[i-1].Name)
		b, bok := parseVersionKey(vs[i].Name)
		if aok && bok && (a[0] < b[0] || (a[0] == b[0] && a[1] < b[1])) {
			t.Errorf("未按版本数值降序: %q 排在 %q 前", vs[i-1].Name, vs[i].Name)
		}
	}
}
