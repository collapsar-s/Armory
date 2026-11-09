package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"armory/internal/model"
)

// envprobe.go 本机运行时扫描。
//
// Java：首选 /usr/libexec/java_home -V（macOS 官方入口），失败退化为扫
// /Library/Java/JavaVirtualMachines 与 ~/Library/Java/JavaVirtualMachines 两个目录。
// 绝不把 PATH 里的 java（含 /usr/bin/java 系统存根）当作已安装版本——
// 存根只是转发到默认 JDK，扫出来会得到重复的假版本项。
//
// Python：按「用户管理的优先于系统自带的」顺序收集候选（pyenv > Homebrew >
// /usr/local > python.org 框架 > conda 基环境 > /usr/bin > PATH），实际执行
// --version 取准确版本号。跳过 pyenv shims（转发脚本，不是真实解释器）。
//
// 去重规则（2026-11-08 定）：只有完全相同的路径才算重复；symlink 别名（同一
// 真实文件的多个名字）按解析后的真实文件合并，展示路径保留候选原样。不同路径
// 的安装一律保留，同名时用全版本号命名区分（如三个 JDK 8 → "Java 1.8.0_371 /
// 1.8.0_291 / 1.8.0_65"）；版本名必须唯一——工具配置与启动器按名字引用版本。

// probeEntry 扫描到的单个解释器/JDK（path 保证互异）。
type probeEntry struct {
	name string // 两位短名（"Python 3.11" / "Java 1.8"）
	full string // 全版本号（"3.11.15" / "1.8.0_371"），用于同名组区分；可为空
	path string
}

// dedupeProbeEntries 按路径去重：完全相同的路径先到先得，不同路径一律保留。
func dedupeProbeEntries(es []probeEntry) []probeEntry {
	seen := map[string]bool{}
	out := es[:0]
	for _, e := range es {
		if seen[e.path] {
			continue
		}
		seen[e.path] = true
		out = append(out, e)
	}
	return out
}

// resolveNameCollisions 落最终版本名：同名（两位短名）组内只有一条时用短名；
// 多条是不同的具体安装，全部改用全版本号命名以便逐个选择；全版本号仍撞名
// （同版本装了多份）追加 " #N"。
func resolveNameCollisions(es []probeEntry) []model.RuntimeVersion {
	counts := map[string]int{}
	for _, e := range es {
		counts[e.name]++
	}
	used := map[string]int{}
	vs := make([]model.RuntimeVersion, 0, len(es))
	for _, e := range es {
		name := e.name
		if counts[name] > 1 && e.full != "" {
			prefix := name
			if i := strings.IndexByte(name, ' '); i > 0 {
				prefix = name[:i]
			}
			name = prefix + " " + e.full
		}
		used[name]++
		if n := used[name]; n > 1 {
			name = fmt.Sprintf("%s #%d", name, n)
		}
		vs = append(vs, model.RuntimeVersion{Name: name, Path: e.path})
	}
	return vs
}

// DetectJavaVersions 扫描本机 JDK。
func DetectJavaVersions() []model.RuntimeVersion {
	if out, err := exec.Command("/usr/libexec/java_home", "-V").CombinedOutput(); err == nil {
		var es []probeEntry
		for _, line := range strings.Split(string(out), "\n") {
			home, ver, ok := javaHomeLine(line)
			if !ok {
				continue
			}
			bin := filepath.Join(home, "bin", "java")
			if !fileExists(bin) {
				continue
			}
			es = append(es, probeEntry{name: "Java " + majorVersion(ver), full: ver, path: bin})
		}
		if len(es) > 0 {
			vs := resolveNameCollisions(dedupeProbeEntries(es))
			sortRuntimeVersionsDesc(vs)
			return vs
		}
	}
	// 兜底：直接扫目录（名称 = "Java " + 完整目录名，保留 .jdk 后缀；无全版本号）
	var es []probeEntry
	roots := []string{"/Library/Java/JavaVirtualMachines", filepath.Join(homeDir(), "Library/Java/JavaVirtualMachines")}
	for _, root := range roots {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			bin := filepath.Join(root, e.Name(), "Contents", "Home", "bin", "java")
			if fileExists(bin) {
				es = append(es, probeEntry{name: "Java " + e.Name(), path: bin})
			}
		}
	}
	vs := resolveNameCollisions(dedupeProbeEntries(es))
	sortRuntimeVersionsDesc(vs)
	return vs
}

// javaHomeLine 解析 java_home -V 输出的一行，返回 (home, 版本号, ok)。
// 兼容两种行格式——旧版路径带引号；macOS 26 起路径裸在行尾、厂商名带引号：
//
//	  21.0.5 (arm64) "/Library/Java/JavaVirtualMachines/jdk-21/Contents/Home"
//	  26.0.2 (arm64) "Oracle Corporation" - "Java SE 26.0.2" /Library/Java/JavaVirtualMachines/jdk-26.jdk/Contents/Home
//
// 统一策略：取行尾最后一个字段剥引号、校验 /Contents/Home 后缀，版本号取行首
// 第一个字段（java_home 末尾那行裸的默认 JVM 路径只有一个字段，自然被排除）。
func javaHomeLine(line string) (home, ver string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	home = strings.Trim(fields[len(fields)-1], "\"")
	if !strings.HasSuffix(home, "/Contents/Home") {
		return "", "", false
	}
	ver = fields[0]
	if _, err := strconv.Atoi(strings.Split(ver, ".")[0]); err != nil {
		return "", "", false
	}
	return home, ver, true
}

// pythonBinNameRE 可执行文件名白名单：python / python2 / python2.7 / python3 /
// python3.11 之类，排除 python3.11-config、python2.7-gdb.py 等同前缀干扰项。
var pythonBinNameRE = regexp.MustCompile(`^python(?:\d+(?:\.\d+)*)?$`)

// isShimPath pyenv shims 里的 python3 是转发脚本，不是真实解释器，
// 扫出来只会和真实项重名。
func isShimPath(p string) bool {
	return strings.Contains(p, "/shims/")
}

// pythonPathRank 同一解释器的多个别名（symlink 链）里优先展示哪个路径：
// python3 最规范，其次 python（pyenv 2.x 目录里只有它），再按 3.x/2.x 明细。
func pythonPathRank(base string) int {
	switch base {
	case "python3":
		return 0
	case "python":
		return 1
	}
	if strings.HasPrefix(base, "python3") {
		return 2
	}
	return 3
}

// DetectPythonVersions 扫描本机 Python 解释器，实际执行 --version 取准确版本号。
// symlink 别名按 realpath 合并，保留原始候选路径作为展示路径；不同路径的安装
// 全部保留，同名时由 resolveNameCollisions 用全版本号命名。
func DetectPythonVersions() []model.RuntimeVersion {
	h := homeDir()

	// 1) 收集候选：固定位置 + glob，按优先级顺序 append，组内按别名规范度排序。
	var candidates []string
	seenPath := map[string]bool{}
	add := func(p string) {
		if p == "" || seenPath[p] || !fileExists(p) || isShimPath(p) {
			return
		}
		seenPath[p] = true
		candidates = append(candidates, p)
	}
	glob := func(patterns ...string) {
		for _, g := range patterns {
			matches, _ := filepath.Glob(g)
			sort.SliceStable(matches, func(i, j int) bool {
				return pythonPathRank(filepath.Base(matches[i])) < pythonPathRank(filepath.Base(matches[j]))
			})
			for _, m := range matches {
				if pythonBinNameRE.MatchString(filepath.Base(m)) {
					add(m)
				}
			}
		}
	}
	glob(
		filepath.Join(h, ".pyenv", "versions", "*", "bin", "python*"), // 全版本，含 2.x
		"/opt/homebrew/bin/python*",
		"/usr/local/bin/python*",
		"/Library/Frameworks/Python.framework/Versions/*/bin/python3*",
	)
	for _, root := range []string{"miniconda3", "anaconda3", "miniforge3", "mambaforge"} {
		add(filepath.Join(h, root, "bin", "python3")) // 仅基环境，envs/* 不扫
	}
	add("/usr/bin/python3")
	if p, _ := exec.LookPath("python3"); p != "" {
		add(p)
	}

	// 2) symlink 别名合并：同一真实文件的多个名字只算一个候选。
	type cand struct{ shown, real string }
	var merged []cand
	seenReal := map[string]bool{}
	for _, p := range candidates {
		real := p
		if r, err := filepath.EvalSymlinks(p); err == nil && r != "" {
			real = r
		}
		if seenReal[real] {
			continue
		}
		seenReal[real] = true
		merged = append(merged, cand{shown: p, real: real})
	}

	// 3) 实际执行 --version，逐行找 "Python X.Y.Z"（容忍退出码非零与前置噪音行）。
	var es []probeEntry
	for _, c := range merged {
		out, _ := exec.Command(c.shown, "--version").CombinedOutput()
		full := pythonVersionFromOutput(string(out))
		if full == "" {
			continue
		}
		es = append(es, probeEntry{name: "Python " + majorVersion(full), full: full, path: c.shown})
	}
	vs := resolveNameCollisions(dedupeProbeEntries(es))
	sortRuntimeVersionsDesc(vs)
	return vs
}

// pythonVersionFromOutput 从 --version 输出中解析版本号段（"Python 3.11.15 (main)"
// → "3.11.15"）。逐行扫描以跳过前置噪音（conda 警告等）；版本号必须为纯数字段，
// 否则视为非版本输出返回空——名字直接取原始输出会让配置里出现垃圾行。
func pythonVersionFromOutput(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Python ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v := fields[1]
		for _, seg := range strings.Split(v, ".") {
			if _, err := strconv.Atoi(seg); err != nil {
				v = ""
				break
			}
		}
		if v != "" {
			return v
		}
	}
	return ""
}

// sortRuntimeVersionsDesc 按名字里的版本号数值降序（"Python 3.11" 必须排在
// "Python 3.9" 前，字符串比较会排反）；解析不出版本号的排最后、按名字兜底。
func sortRuntimeVersionsDesc(vs []model.RuntimeVersion) {
	sort.SliceStable(vs, func(i, j int) bool {
		a, aok := parseVersionKey(vs[i].Name)
		b, bok := parseVersionKey(vs[j].Name)
		switch {
		case !aok && !bok:
			return vs[i].Name > vs[j].Name
		case !aok:
			return false
		case !bok:
			return true
		}
		if a[0] != b[0] {
			return a[0] > b[0]
		}
		if a[1] != b[1] {
			return a[1] > b[1]
		}
		// 同 major.minor（如三个 JDK 8）：按全版本号数值比较，新的在前。
		if c := versionCompare(versionOf(vs[i].Name), versionOf(vs[j].Name)); c != 0 {
			return c > 0
		}
		return vs[i].Name > vs[j].Name
	})
}

// versionOf 取名字里的版本号段（"Java 1.8.0_371" → "1.8.0_371"）。
func versionOf(name string) string {
	fields := strings.Fields(name)
	if len(fields) < 2 {
		return ""
	}
	return fields[1]
}

// versionCompare 把版本号按 . 与 _ 切段逐段数值比较，返回 -1/0/1；
// 非数字段按 0 处理（仅用于同 major.minor 的 tie-break，段型一致）。
func versionCompare(a, b string) int {
	as := strings.FieldsFunc(a, func(r rune) bool { return r == '.' || r == '_' })
	bs := strings.FieldsFunc(b, func(r rune) bool { return r == '.' || r == '_' })
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

// parseVersionKey 解析 "前缀 X.Y" 形式名字的 (major, minor)，如 "Python 3.11" →
// [3, 11]；全版本号名（"Java 1.8.0_371"）取前两位；解析失败返回 ok=false。
func parseVersionKey(name string) ([2]int, bool) {
	fields := strings.Fields(name)
	if len(fields) < 2 {
		return [2]int{}, false
	}
	segs := strings.Split(fields[1], ".")
	n0, err := strconv.Atoi(segs[0])
	if err != nil {
		return [2]int{}, false
	}
	if len(segs) == 1 {
		return [2]int{n0, 0}, true
	}
	n1, err := strconv.Atoi(segs[1])
	if err != nil {
		return [2]int{}, false
	}
	return [2]int{n0, n1}, true
}

// majorVersion 取主版本号："21.0.5" → "21.0"；"3.12.7" → "3.12"（两位更易读）。
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

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}
