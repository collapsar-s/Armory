package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"armory/internal/store"
)

// newTestRegistry 为测试准备隔离的数据目录并返回 Registry。
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	store.SetDataDirForTest(t.TempDir())
	t.Cleanup(func() { store.SetDataDirForTest("") })
	reg, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// mkTestFile 在临时目录创建一个真实存在的文件并返回路径（收录校验要求路径存在）。
func mkTestFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestAddToolByPathDuplicatePath 同一路径不允许重复收录（防呆：批量拖拽/重复导入）。
func TestAddToolByPathDuplicatePath(t *testing.T) {
	reg := newTestRegistry(t)
	api := NewAPI(reg)

	p := mkTestFile(t, "dup.jar")
	if _, err := api.AddToolByPath(p, "", "首个", "", "", ""); err != nil {
		t.Fatalf("首次收录应成功: %v", err)
	}
	_, err := api.AddToolByPath(p, "", "第二个", "", "", "")
	if err == nil {
		t.Fatal("重复收录同一路径应报错")
	}
	if !strings.Contains(err.Error(), "已收录") {
		t.Errorf("报错应说明已收录，得到: %v", err)
	}
}

// TestTagRenameAndDelete 标签跨工具批量重命名/删除；未使用的标签报错。
func TestTagRenameAndDelete(t *testing.T) {
	reg := newTestRegistry(t)
	api := NewAPI(reg)

	t1, err := api.AddToolByPath(mkTestFile(t, "x.jar"), "", "A", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := api.AddToolByPath(mkTestFile(t, "y.jar"), "", "B", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.UpdateTool(t1.ID, "A", "", "", "", []string{"GUI", "需Agent"}, false, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := api.UpdateTool(t2.ID, "B", "", "", "", []string{"需Agent", "常用"}, false, "", "", ""); err != nil {
		t.Fatal(err)
	}

	// 重命名：所有工具上的 需Agent → 挂Agent
	if err := api.RenameTag("需Agent", "挂Agent"); err != nil {
		t.Fatalf("重命名失败: %v", err)
	}
	got1, _ := reg.GetTool(t1.ID)
	if len(got1.Tags) != 2 || got1.Tags[0] != "GUI" || got1.Tags[1] != "挂Agent" {
		t.Errorf("A 标签应为 [GUI 挂Agent]，得到 %v", got1.Tags)
	}
	got2, _ := reg.GetTool(t2.ID)
	if len(got2.Tags) != 2 || got2.Tags[0] != "挂Agent" || got2.Tags[1] != "常用" {
		t.Errorf("B 标签应为 [挂Agent 常用]，得到 %v", got2.Tags)
	}

	// 重命名到既有名称 → 去重不产生重复标签
	if err := api.RenameTag("挂Agent", "GUI"); err != nil {
		t.Fatalf("重命名到既有名称失败: %v", err)
	}
	got1, _ = reg.GetTool(t1.ID)
	if len(got1.Tags) != 1 || got1.Tags[0] != "GUI" {
		t.Errorf("重命名重合后 A 应只剩 [GUI]，得到 %v", got1.Tags)
	}

	// 删除：从所有工具移除
	if err := api.DeleteTag("GUI"); err != nil {
		t.Fatalf("删除标签失败: %v", err)
	}
	got1, _ = reg.GetTool(t1.ID)
	got2, _ = reg.GetTool(t2.ID)
	if len(got1.Tags) != 0 || len(got2.Tags) != 1 || got2.Tags[0] != "常用" {
		t.Errorf("删除 GUI 后 A=%v B=%v", got1.Tags, got2.Tags)
	}

	// 不存在的标签报错
	if err := api.RenameTag("不存在", "x"); err == nil {
		t.Error("重命名不存在的标签应报错")
	}
	if err := api.DeleteTag("不存在"); err == nil {
		t.Error("删除不存在的标签应报错")
	}
}

// TestUpdateToolWorkingDir 工作目录仅在完整编辑时覆盖；部分更新（拖拽）保留。
func TestUpdateToolWorkingDir(t *testing.T) {
	reg := newTestRegistry(t)
	api := NewAPI(reg)

	p := mkTestFile(t, "wd.jar")
	tool, err := api.AddToolByPath(p, "", "WD", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// 完整编辑设置工作目录
	if _, err := api.UpdateTool(tool.ID, "WD", "", "", "", nil, false, "", "", "~/tools"); err != nil {
		t.Fatal(err)
	}
	got, _ := reg.GetTool(tool.ID)
	if got.WorkingDir != "~/tools" {
		t.Errorf("完整编辑应写入 workingDir，得到 %q", got.WorkingDir)
	}
	// 部分更新（拖拽改归属）：workingDir 传空占位 → 保留
	if _, err := api.UpdateTool(tool.ID, "", "", "", "", nil, false, "cat-开发日常", "", ""); err != nil {
		t.Fatal(err)
	}
	got, _ = reg.GetTool(tool.ID)
	if got.WorkingDir != "~/tools" || got.CategoryID != "cat-开发日常" {
		t.Errorf("部分更新应保留 workingDir 并改分组，得到 wd=%q cat=%q", got.WorkingDir, got.CategoryID)
	}
	// 完整编辑传空 → 清空（回退默认工作目录）
	if _, err := api.UpdateTool(tool.ID, "WD", "", "", "", nil, false, "", "", ""); err != nil {
		t.Fatal(err)
	}
	got, _ = reg.GetTool(tool.ID)
	if got.WorkingDir != "" {
		t.Errorf("完整编辑传空应清空 workingDir，得到 %q", got.WorkingDir)
	}
}
