package service

import (
	"os"
	"path/filepath"
	"testing"

	"armory/internal/model"
)

// TestBackupRoundTrip M4-4：导出 → 新库导入，工具/分类/图标应完全还原。
func TestBackupRoundTrip(t *testing.T) {
	dir1 := t.TempDir()
	reg1, err := NewRegistry(dir1)
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

	api1 := &API{Reg: reg1, dataDir: dir1}
	zipPath := filepath.Join(t.TempDir(), "backup.zip")
	if err := api1.packBackup(zipPath); err != nil {
		t.Fatalf("打包失败: %v", err)
	}

	// 全新空库导入
	dir2 := t.TempDir()
	reg2, err := NewRegistry(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg2.AddCategory("干扰分类"); err != nil { // 导入应覆盖掉它
		t.Fatal(err)
	}
	api2 := &API{Reg: reg2, dataDir: dir2}
	if err := api2.restoreBackup(zipPath); err != nil {
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
	// 源库 = 默认 4 分类 + 「测试分类」= 5；干扰分类应被覆盖消失
	if len(cats) != 5 {
		t.Errorf("分类应还原为备份内容（5 个），实际 %d 个", len(cats))
	}
	found := false
	for _, c := range cats {
		if c.ID == cat.ID && c.Name == "测试分类" {
			found = true
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
