package service

import (
	"os"
	"path/filepath"
	"testing"

	"armory/internal/model"
	"armory/internal/store"
)

// TestBackupRoundTrip M4-4：导出 → 全新库导入，工具/分类/图标应完全还原。
// 通过 SetDataDirForTest 注入临时目录，绝不触碰真实 ~/.armory。
func TestBackupRoundTrip(t *testing.T) {
	dir1 := t.TempDir()
	store.SetDataDirForTest(dir1)
	defer store.SetDataDirForTest("")

	reg1, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := reg1.AddCategory("测试分类")
	if err != nil {
		t.Fatal(err)
	}
	tool, err := reg1.AddTool(model.Tool{
		Name: "渗透工具A", Path: "/opt/tools/a.jar", Type: "jar", CategoryID: cat.ID,
		EnvVersion: "Java 21",
	})
	if err != nil {
		t.Fatal(err)
	}
	iconsDir := filepath.Join(dir1, "cache", "icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(iconsDir, tool.ID+".png"), []byte("fake-icon"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath, err := reg1.buildBackup()
	if err != nil {
		t.Fatalf("打包失败: %v", err)
	}
	defer os.Remove(zipPath)

	// 全新空库导入（干扰数据应被覆盖）
	dir2 := t.TempDir()
	store.SetDataDirForTest(dir2)
	reg2, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg2.AddCategory("干扰分类"); err != nil {
		t.Fatal(err)
	}
	if err := reg2.restoreBackup(zipPath); err != nil {
		t.Fatalf("还原失败: %v", err)
	}

	got, err := reg2.GetTool(tool.ID)
	if err != nil {
		t.Fatalf("还原后工具缺失: %v", err)
	}
	if got.Name != "渗透工具A" || got.EnvVersion != "Java 21" || got.CategoryID != cat.ID {
		t.Errorf("工具字段不一致: %+v", got)
	}
	cats := reg2.ListCategories()
	// 源库仅含「测试分类」（无默认分类）；「干扰分类」应被覆盖消失
	found := false
	for _, c := range cats {
		if c.ID == cat.ID && c.Name == "测试分类" {
			found = true
		}
		if c.Name == "干扰分类" {
			t.Errorf("干扰分类未被覆盖: %+v", c)
		}
	}
	if !found {
		t.Errorf("「测试分类」未被还原")
	}
	iconData, err := os.ReadFile(filepath.Join(dir2, "cache", "icons", tool.ID+".png"))
	if err != nil || string(iconData) != "fake-icon" {
		t.Errorf("图标未还原: %v", err)
	}
}
