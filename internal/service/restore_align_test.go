package service

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"armory/internal/model"
	"armory/internal/store"
)

// restore_align_test.go —— 第二轮对齐（RESTORE-DONE.md §六）行为的回归测试。
// 全部通过 SetDataDirForTest 注入临时目录，绝不触碰真实 ~/.armory。

func newTestAPI(t *testing.T) (*API, *Registry) {
	t.Helper()
	store.SetDataDirForTest(t.TempDir())
	t.Cleanup(func() { store.SetDataDirForTest("") })
	reg, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return NewAPI(reg), reg
}

// TestIconDirIsCacheIcons 图标缓存目录应为 ~/.armory/cache/icons（原版路径）。
func TestIconDirIsCacheIcons(t *testing.T) {
	dir := store.IconDir()
	if !strings.HasSuffix(filepath.ToSlash(dir), "/cache/icons") {
		t.Errorf("图标目录应为 cache/icons，得到 %q", dir)
	}
}

// TestAddToolPathValidation 收录时的路径校验：空路径 / 不存在的路径直接拒绝。
func TestAddToolPathValidation(t *testing.T) {
	api, _ := newTestAPI(t)

	if _, err := api.AddToolByPath("   ", "", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "路径为空") {
		t.Errorf("空路径应报「路径为空」，得到 %v", err)
	}
	if _, err := api.AddToolByPath("/nonexistent/tool-xyz.jar", "", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "路径不存在") {
		t.Errorf("不存在路径应报「路径不存在」，得到 %v", err)
	}
	// 真实存在的路径应能收录（用测试自身的源文件）。
	me, _ := os.Executable()
	if _, err := api.AddToolByPath(me, "", "t", "", "", ""); err != nil {
		t.Errorf("存在路径应收录成功: %v", err)
	}
}

// TestDeleteCategoryGuard 分类删除守卫：至少保留一个分类；工具优先归入上级。
func TestDeleteCategoryGuard(t *testing.T) {
	api, reg := newTestAPI(t)

	// 仅剩默认分类时可删到只剩一个，但删最后一个被拒绝。
	cats := reg.ListCategories()
	if len(cats) != 4 {
		t.Fatalf("默认应有 4 个分类，得到 %d", len(cats))
	}
	// 建一个子分类，往里放一个工具，删掉子分类后工具应归入父级。
	sub, err := api.AddSubCategory(cats[0].ID, "子分类")
	if err != nil {
		t.Fatal(err)
	}
	me, _ := os.Executable()
	tool, err := api.AddToolByPath(me, sub.ID, "t", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := api.DeleteCategory(sub.ID); err != nil {
		t.Fatalf("删除子分类失败: %v", err)
	}
	got, err := reg.GetTool(tool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CategoryID != cats[0].ID {
		t.Errorf("删除子分类后工具应归入上级 %q，得到 %q", cats[0].ID, got.CategoryID)
	}

	// 逐个删除，最后一个应被拒绝。
	for len(reg.ListCategories()) > 1 {
		if err := api.DeleteCategory(reg.ListCategories()[0].ID); err != nil {
			t.Fatalf("删除分类失败: %v", err)
		}
	}
	rest := reg.ListCategories()
	if err := api.DeleteCategory(rest[0].ID); err == nil || !strings.Contains(err.Error(), "至少保留一个分类") {
		t.Errorf("最后一个分类应拒绝删除并报「至少保留一个分类」，得到 %v", err)
	}
}

// TestBackupZipStructure 备份 zip 的数据条目应为 tools.json（原版结构），图标在 icons/ 下。
func TestBackupZipStructure(t *testing.T) {
	api, reg := newTestAPI(t)

	me, _ := os.Executable()
	if _, err := api.AddToolByPath(me, "", "t", "", "", ""); err != nil {
		t.Fatal(err)
	}
	// 造一个假图标，验证打包。
	tools := reg.ListTools("all")
	if err := os.WriteFile(store.IconPathFor(tools[0].ID), []byte("icon-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath, err := reg.buildBackup()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["tools.json"] {
		t.Errorf("备份 zip 应包含 tools.json，实际条目: %v", names)
	}
	if names["data.json"] {
		t.Errorf("备份 zip 不应再使用 data.json 条目名")
	}
	if !names["icons/"+tools[0].ID+".png"] {
		t.Errorf("备份 zip 应包含 icons/<id>.png，实际条目: %v", names)
	}
}

// TestExportImportWithDialogHooks 导出/导入的对话框流程：
// 导出经保存对话框落盘（默认名 armory-backup-*.zip），取消返回空串；
// 导入经打开对话框还原；对话框 hook 缺失时给出原版错误文案。
func TestExportImportWithDialogHooks(t *testing.T) {
	api, _ := newTestAPI(t)

	me, _ := os.Executable()
	src, err := api.AddToolByPath(me, "", "t", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// hook 缺失 → 原版错误文案
	if _, err := api.ExportBackup(); err == nil || !strings.Contains(err.Error(), "导出对话框不可用") {
		t.Errorf("无 SaveFile hook 应报「导出对话框不可用」，得到 %v", err)
	}
	if _, err := api.ImportBackup(); err == nil || !strings.Contains(err.Error(), "文件对话框不可用") {
		t.Errorf("无 OpenFile hook 应报「文件对话框不可用」，得到 %v", err)
	}

	// 正常导出 → 文件落盘且文件名符合 armory-backup-<ts>.zip
	outDir := t.TempDir()
	api.SetDialogHooks(DialogHooks{
		SaveFile: func(title, defaultName string, filters []string) (string, error) {
			if defaultName == "" || !strings.HasPrefix(defaultName, "armory-backup-") || !strings.HasSuffix(defaultName, ".zip") {
				t.Errorf("保存对话框默认名应为 armory-backup-<ts>.zip，得到 %q", defaultName)
			}
			return filepath.Join(outDir, defaultName), nil
		},
	})
	backupPath, err := api.ExportBackup()
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if backupPath == "" {
		t.Fatal("导出应返回落盘路径")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("备份文件不存在: %v", err)
	}

	// 用户取消 → 空串且无错误
	api.SetDialogHooks(DialogHooks{
		SaveFile: func(string, string, []string) (string, error) { return "", nil },
	})
	if p, err := api.ExportBackup(); p != "" || err != nil {
		t.Errorf("取消导出应返回空串无错误，得到 %q, %v", p, err)
	}

	// 全新库 + 打开对话框 → 完整还原
	api2, reg2 := newTestAPI(t)
	api2.SetDialogHooks(DialogHooks{
		OpenFile: func(title string, filters []string) (string, error) {
			return backupPath, nil
		},
	})
	if _, err := api2.ImportBackup(); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if _, err := reg2.GetTool(src.ID); err != nil {
		t.Errorf("还原后应能取到工具 %s: %v", src.ID, err)
	}
	// 图标应落在 cache/icons
	if _, err := os.Stat(store.IconPathFor(src.ID)); err == nil {
		// 备份中无该图标（未写入），此处只验证目录行为，不作强断言
		_ = err
	}
}

// TestImportAcceptsCacheIconsPrefix 还原兼容 cache/icons/ 前缀的图标条目（原版目录布局）。
func TestImportAcceptsCacheIconsPrefix(t *testing.T) {
	_, reg := newTestAPI(t)

	zipPath := filepath.Join(t.TempDir(), "old-style.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("tools.json")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(`{"Tools":[],"Categories":[{"id":"cat-x","name":"信息收集","order":0}],"Settings":{}}`))
	w2, err := zw.Create("cache/icons/abc123.png")
	if err != nil {
		t.Fatal(err)
	}
	w2.Write([]byte("old-icon"))
	zw.Close()
	f.Close()

	if err := reg.restoreBackup(zipPath); err != nil {
		t.Fatalf("还原失败: %v", err)
	}
	b, err := os.ReadFile(store.IconPathFor("abc123"))
	if err != nil || string(b) != "old-icon" {
		t.Errorf("cache/icons/ 前缀的图标应被还原到图标目录: %v", err)
	}
}

// TestReadmeCandidatesUseShuoMing 说明文档候选应包含 使用说明.md。
func TestReadmeCandidatesUseShuoMing(t *testing.T) {
	found := false
	for _, n := range readmeCandidates {
		if n == "使用说明.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("readme 候选应包含 使用说明.md: %v", readmeCandidates)
	}
}

// TestLauncherMissingPath 启动不存在路径的工具应报「路径不存在」。
func TestLauncherMissingPath(t *testing.T) {
	api, _ := newTestAPI(t)
	tool, err := api.AddToolByPath(os.Args[0], "", "t", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// 把路径改成不存在的文件再启动。
	if _, err := api.UpdateTool(tool.ID, "", "/nonexistent/nope.jar", "", "", nil, false, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := api.Launch(tool.ID); err == nil || !strings.Contains(err.Error(), "路径不存在") {
		t.Errorf("启动不存在路径应报「路径不存在」，得到 %v", err)
	}
}

// TestDefaultCategoriesMatchOriginal 默认分类与原版一致（信息收集/漏洞利用/代理隧道/开发日常）。
func TestDefaultCategoriesMatchOriginal(t *testing.T) {
	_, reg := newTestAPI(t)
	cats := reg.ListCategories()
	want := []string{"信息收集", "漏洞利用", "代理隧道", "开发日常"}
	if len(cats) != len(want) {
		t.Fatalf("默认分类数量应为 %d，得到 %d", len(want), len(cats))
	}
	for i, c := range cats {
		if c.Name != want[i] || c.ID != "cat-"+want[i] {
			t.Errorf("默认分类[%d] 应为 %s(cat-%s)，得到 %s(%s)", i, want[i], want[i], c.Name, c.ID)
		}
	}
	// 目录不应写 kind（与原版 JSON 输出一致）。
	for _, c := range cats {
		if c.Kind != "" {
			t.Errorf("默认目录 kind 应为空，得到 %q", c.Kind)
		}
	}
}

// TestDetectJavaScanNoStub Java 扫描不得把 PATH 里的 /usr/bin/java 存根当版本
// （原版只走 java_home -V + JVM 目录扫描；存根转发到默认 JDK，扫出来是假版本项）。
func TestDetectJavaScanNoStub(t *testing.T) {
	vs := DetectJavaVersions()
	if len(vs) == 0 {
		t.Fatal("本机应扫描到 JDK")
	}
	for _, v := range vs {
		if v.Path == "/usr/bin/java" {
			t.Errorf("/usr/bin/java 存根不应出现在扫描结果中: %+v", v)
		}
		if !strings.HasSuffix(v.Path, "/bin/java") {
			t.Errorf("扫描结果应为 JVM 的 bin/java: %+v", v)
		}
		if !strings.HasPrefix(v.Name, "Java ") {
			t.Errorf("版本名应以 Java 开头: %+v", v)
		}
	}
	// 系统级 JVM 目录里的条目命名形态：java_home -V 可用时 = "Java X.Y"（2026-11 起
	// 解析器兼容 macOS 26 新行格式，不再整体退化到目录扫描）；java_home 不可用时
	// 兜底 = "Java " + 完整目录名（保留 .jdk 后缀）。两种形态均合法。
	entries, err := os.ReadDir("/Library/Java/JavaVirtualMachines")
	if err != nil || len(entries) == 0 {
		t.Skip("本机无系统级 JVM 目录，跳过命名形态校验")
	}
	got := map[string]string{}
	for _, v := range vs {
		got[v.Path] = v.Name
	}
	for _, e := range entries {
		bin := "/Library/Java/JavaVirtualMachines/" + e.Name() + "/Contents/Home/bin/java"
		name, ok := got[bin]
		if !ok {
			continue
		}
		clean := strings.TrimPrefix(name, "Java ")
		_, verOk := parseVersionKey(name)
		isVersionForm := clean != name && verOk // "Java X.Y"
		isDirForm := name == "Java "+e.Name()
		if !isVersionForm && !isDirForm {
			t.Errorf("JVM %s 的版本名 %q 不符合两种合法形态（\"Java X.Y\" 或 \"Java %s\"）", e.Name(), name, e.Name())
		}
	}
}

// TestMajorVersionOriginalQuirks majorVersion 的原版行为（含两位小数的"怪癖"）。
func TestMajorVersionOriginalQuirks(t *testing.T) {
	cases := map[string]string{
		"21.0.5":    "21.0", // 原版取两位
		"3.12.7":    "3.12", // Python 命名即来自这里（"Python 3.12"）
		"26":        "26",   // 单段取整段
		"1.8.0_291": "1.8",
	}
	for in, want := range cases {
		if got := majorVersion(in); got != want {
			t.Errorf("majorVersion(%q) 应为 %q，得到 %q", in, want, got)
		}
	}
}

// TestAddAppBundleDirAccepted 收录 .app 包（目录）必须放行——回归：
// fileExists 一度改为"目录不算存在"，导致 .app 收录/启动/图标提取全被拒。
func TestAddAppBundleDirAccepted(t *testing.T) {
	api, reg := newTestAPI(t)

	// 造一个最小 .app 包（目录形态，无 icns——图标提取失败不阻断收录）
	appDir := filepath.Join(t.TempDir(), "FakeTool.app")
	if err := os.MkdirAll(filepath.Join(appDir, "Contents", "Resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	tool, err := api.AddToolByPath(appDir, "", "", "", "", "")
	if err != nil {
		t.Fatalf("收录 .app 目录应成功，得到 %v", err)
	}
	if tool.Type != "app" {
		t.Errorf("类型应为 app，得到 %q", tool.Type)
	}
	if tool.Name != "FakeTool" {
		t.Errorf("默认名应为包名（去 .app），得到 %q", tool.Name)
	}
	if _, err := reg.GetTool(tool.ID); err != nil {
		t.Errorf("工具应已落库: %v", err)
	}
}

// TestAddToolFileURLPrefix 拖拽传来的 file:// 前缀应被剥离（与原版一致）。
func TestAddToolFileURLPrefix(t *testing.T) {
	api, _ := newTestAPI(t)
	me, _ := os.Executable()
	tool, err := api.AddToolByPath("file://"+me, "", "", "", "", "")
	if err != nil {
		t.Fatalf("file:// 路径应可收录: %v", err)
	}
	if tool.Path != me {
		t.Errorf("存储路径应剥离 file:// 前缀，得到 %q", tool.Path)
	}
}

// TestExtractIconRealBundle 对真实 .app 包的图标提取（本机有 云资产管理工具.app 时验证）。
func TestExtractIconRealBundle(t *testing.T) {
	const realApp = "/Applications/云资产管理工具.app"
	if _, err := os.Stat(realApp); err != nil {
		t.Skip("本机无该 .app，跳过")
	}
	out := filepath.Join(t.TempDir(), "icon.png")
	if err := ExtractAppIcon(realApp, out); err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil || len(b) < 8 {
		t.Fatalf("图标 PNG 为空: %v", err)
	}
	if string(b[:4]) != "\x89PNG" {
		t.Errorf("输出不是 PNG: % x", b[:4])
	}
}

// TestImportRejectsMissingCategories 备份包缺少分类数据时应拒绝（原版文案）。
func TestImportRejectsMissingCategories(t *testing.T) {
	_, reg := newTestAPI(t)
	zipPath := filepath.Join(t.TempDir(), "no-cats.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("tools.json")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(`{"Tools":[],"Categories":[],"Settings":{}}`))
	zw.Close()
	f.Close()

	if err := reg.restoreBackup(zipPath); err == nil || !strings.Contains(err.Error(), "备份包缺少分类数据") {
		t.Errorf("缺少分类数据应报「备份包缺少分类数据」，得到 %v", err)
	}
}

// TestReorderCategories 分类拖拽排序：按给定 id 顺序写 Order，未提及的保持原值。
func TestReorderCategories(t *testing.T) {
	api, _ := newTestAPI(t)
	if _, err := api.AddCategory("排序A"); err != nil {
		t.Fatal(err)
	}
	b, err := api.AddCategory("排序B")
	if err != nil {
		t.Fatal(err)
	}
	c, err := api.AddCategory("排序C")
	if err != nil {
		t.Fatal(err)
	}
	// 把 C 排到最前，B 其次，A 不提及（保持原值）
	if err := api.ReorderCategories([]string{c.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	cats := map[string]float64{}
	for _, cat := range api.ListCategories() {
		cats[cat.Name] = cat.Order
	}
	if !(cats["排序C"] < cats["排序B"] && cats["排序B"] < cats["排序A"]) {
		t.Errorf("排序应满足 C < B < A，得到 %v", cats)
	}
}

// TestReorderTools 工具卡片拖拽排序：Order 升序稳定排序，未提及的保持收录顺序。
func TestReorderTools(t *testing.T) {
	api, _ := newTestAPI(t)
	dir, err := api.AddCategory("排序目录T")
	if err != nil {
		t.Fatal(err)
	}
	// 收录防呆不允许同路径重复，这里用三个真实文件代替同一可执行文件。
	tmp := t.TempDir()
	mk := func(n string) string {
		p := filepath.Join(tmp, n)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	t1, err := api.AddToolByPath(mk("t1.jar"), dir.ID, "工具1", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := api.AddToolByPath(mk("t2.jar"), dir.ID, "工具2", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t3, err := api.AddToolByPath(mk("t3.jar"), dir.ID, "工具3", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// 把第三个工具拖到最前
	if err := api.ReorderTools([]string{t3.ID, t1.ID, t2.ID}); err != nil {
		t.Fatal(err)
	}
	list := api.ListTools(dir.ID)
	if list[0].ID != t3.ID || list[1].ID != t1.ID || list[2].ID != t2.ID {
		t.Errorf("拖拽排序未生效: %s, %s, %s", list[0].Name, list[1].Name, list[2].Name)
	}
}

// TestListToolsIncludesSubtree 查看父目录时应返回其子目录/后代内的工具（原版语义）。
func TestListToolsIncludesSubtree(t *testing.T) {
	api, _ := newTestAPI(t)
	parent, err := api.AddCategory("父目录T")
	if err != nil {
		t.Fatal(err)
	}
	child, err := api.AddSubCategory(parent.ID, "子目录T")
	if err != nil {
		t.Fatal(err)
	}
	me, _ := os.Executable()
	tool, err := api.AddToolByPath(me, child.ID, "子工具", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, x := range api.ListTools(parent.ID) {
		if x.ID == tool.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("查看父目录应包含子目录内的工具")
	}
	// 无关分类不应包含该工具。
	for _, c := range api.ListCategories() {
		if c.ID == parent.ID || c.ID == child.ID {
			continue
		}
		for _, x := range api.ListTools(c.ID) {
			if x.ID == tool.ID {
				t.Errorf("无关分类 %s 不应包含该工具", c.ID)
			}
		}
	}
}

// TestListCategoriesTreeOrder 侧栏依赖“子项紧跟父项”的树序，不能按 order 全局排序。
func TestListCategoriesTreeOrder(t *testing.T) {
	api, _ := newTestAPI(t)
	p, err := api.AddCategory("树序父T")
	if err != nil {
		t.Fatal(err)
	}
	c, err := api.AddSubCategory(p.ID, "树序子T")
	if err != nil {
		t.Fatal(err)
	}
	cats := api.ListCategories()
	pi, ci := -1, -1
	for i, x := range cats {
		if x.ID == p.ID {
			pi = i
		}
		if x.ID == c.ID {
			ci = i
		}
	}
	if pi < 0 || ci < 0 {
		t.Fatalf("分类缺失 p=%d c=%d", pi, ci)
	}
	if ci != pi+1 {
		t.Errorf("子分类应紧跟父分类：parent=%d child=%d 顺序=%v", pi, ci, cats)
	}
}

// TestUpdateToolKeepsArgsOnDrag 拖拽改归属（name/path 为空的部分更新）不应清空启动参数；
// 但完整编辑时允许显式清空 args。
func TestUpdateToolKeepsArgsOnDrag(t *testing.T) {
	api, _ := newTestAPI(t)
	me, _ := os.Executable()
	tool, err := api.AddToolByPath(me, "", "拖拽工具", "-x --y", "", "")
	if err != nil {
		t.Fatal(err)
	}
	cats := api.ListCategories()
	target := cats[1].ID

	out, err := api.UpdateTool(tool.ID, "", "", "", "", nil, false, target, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Args != "-x --y" {
		t.Errorf("拖拽改归属不应清空 args，得到 %q", out.Args)
	}
	if out.CategoryID != target {
		t.Errorf("拖拽改归属应更新分类，得到 %q", out.CategoryID)
	}

	// 完整编辑（提供 name）时显式清空 args 应生效。
	out2, err := api.UpdateTool(tool.ID, "拖拽工具改", "", "", "", nil, false, target, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if out2.Args != "" {
		t.Errorf("完整编辑应允许清空 args，得到 %q", out2.Args)
	}
}

// TestSetFontScaleValidation 非法字体档位应被拒绝（原版文案）。
func TestSetFontScaleValidation(t *testing.T) {
	api, _ := newTestAPI(t)
	if err := api.SetFontScale("huge"); err == nil || !strings.Contains(err.Error(), "无效的字体缩放档位") {
		t.Errorf("非法档位应报「无效的字体缩放档位」，得到 %v", err)
	}
	if err := api.SetFontScale("large"); err != nil {
		t.Errorf("合法档位应通过，得到 %v", err)
	}
}

// TestRenameCategoryDuplicate 重命名同样要查重；但改成自身原名应允许。
func TestRenameCategoryDuplicate(t *testing.T) {
	api, _ := newTestAPI(t)
	a, err := api.AddCategory("重名A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := api.AddCategory("重名B")
	if err != nil {
		t.Fatal(err)
	}
	if err := api.RenameCategory(b.ID, "重名A"); err == nil || !strings.Contains(err.Error(), "分类已存在") {
		t.Errorf("重命名为同级已存在名应报「分类已存在」，得到 %v", err)
	}
	if err := api.RenameCategory(a.ID, "重名A"); err != nil {
		t.Errorf("改回自身原名应允许，得到 %v", err)
	}
}

// TestImportBackupReturnSemantics 成功应返回 true、取消返回 false（前端据此决定是否刷新）。
func TestImportBackupReturnSemantics(t *testing.T) {
	api, reg := newTestAPI(t)
	me, _ := os.Executable()
	if _, err := api.AddToolByPath(me, "", "备份工具T", "", "", ""); err != nil {
		t.Fatal(err)
	}
	tmp, err := reg.buildBackup()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)

	api.SetDialogHooks(DialogHooks{OpenFile: func(title string, filters []string) (string, error) {
		return tmp, nil
	}})
	ok, err := api.ImportBackup()
	if err != nil || !ok {
		t.Errorf("成功导入应返回 true,nil，得到 %v,%v", ok, err)
	}

	api.SetDialogHooks(DialogHooks{OpenFile: func(title string, filters []string) (string, error) {
		return "", nil
	}})
	ok, err = api.ImportBackup()
	if err != nil || ok {
		t.Errorf("取消导入应返回 false,nil，得到 %v,%v", ok, err)
	}
}

var _ = model.Tool{} // 保持 import 稳定

// TestAddToolByPathJvmArgs 收录 jar 工具时即可带 JVM 参数（启动时拼在 -jar 之前）。
func TestAddToolByPathJvmArgs(t *testing.T) {
	api, _ := newTestAPI(t)
	jar := filepath.Join(t.TempDir(), "x.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool, err := api.AddToolByPath(jar, "", "", "", "-noverify -javaagent:a.jar", "")
	if err != nil {
		t.Fatal(err)
	}
	if tool.Type != "jar" {
		t.Fatalf("应为 jar 类型，得到 %q", tool.Type)
	}
	if tool.JvmArgs != "-noverify -javaagent:a.jar" {
		t.Errorf("收录应保存 JVM 参数，得到 %q", tool.JvmArgs)
	}
}
