package service

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"armory/internal/model"
	"armory/internal/store"
)

// backup.go 实现数据的打包导出（zip）与导入还原（与原版一致）。
//
// 导出：zip 结构 = tools.json（整份注册表）+ icons/<id>.png，经保存对话框
// 落盘，默认文件名 armory-backup-<YYYYMMDD-HHMMSS>.zip。
// 导入：无参调用，后端自行弹出选择对话框（选择备份包（.zip）），整包覆盖还原。

// buildBackup 将当前全部数据与图标打包为 zip，写入临时文件并返回其路径。
func (r *Registry) buildBackup() (string, error) {
	data, err := jsonMarshalIndent(r.Snapshot())
	if err != nil {
		return "", fmt.Errorf("序列化备份数据失败: %w", err)
	}

	f, err := os.CreateTemp("", "armory-backup-*.zip")
	if err != nil {
		return "", fmt.Errorf("创建备份文件失败: %w", err)
	}
	// 出错时清理临时文件；成功时保留并返回路径。
	success := false
	defer func() {
		_ = f.Close()
		if !success {
			_ = os.Remove(f.Name())
		}
	}()

	zw := zip.NewWriter(f)
	w, err := zw.Create("tools.json")
	if err != nil {
		return "", fmt.Errorf("写入 tools.json 失败: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return "", fmt.Errorf("写入 tools.json 内容失败: %w", err)
	}

	// 打包图标（尽力而为：个别图标读失败不阻断整体导出）。
	iconsDir := store.IconDir()
	if entries, err := os.ReadDir(iconsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			full := filepath.Join(iconsDir, e.Name())
			src, err := os.Open(full)
			if err != nil {
				continue
			}
			dst, err := zw.Create("icons/" + e.Name())
			if err != nil {
				_ = src.Close()
				continue
			}
			_, _ = io.Copy(dst, src)
			_ = src.Close()
		}
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("完成备份打包失败: %w", err)
	}
	success = true
	return f.Name(), nil
}

// ExportBackup 弹出保存对话框（默认名 armory-backup-<时间戳>.zip），写入备份 zip。
// 用户取消时返回空串（前端据此不提示）。
func (a *API) ExportBackup() (string, error) {
	if a.dialogs.SaveFile == nil {
		return "", fmt.Errorf("导出对话框不可用")
	}
	tmp, err := a.reg.buildBackup()
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp) }()

	defaultName := fmt.Sprintf("armory-backup-%s.zip", time.Now().Format("20060102-150405"))
	dest, err := a.dialogs.SaveFile("导出备份", defaultName, []string{"zip"})
	if err != nil {
		return "", err
	}
	if dest == "" {
		return "", nil // 用户取消
	}
	if err := copyFile(tmp, dest); err != nil {
		return "", fmt.Errorf("写入备份文件失败: %w", err)
	}
	return dest, nil
}

// ImportBackup 弹出选择对话框（选择备份包（.zip））并整包还原。
// 返回值语义与原版前端一致：成功 true；用户取消 false；出错通过 error 返回。
// （前端逻辑为 `if (!await $.importBackup()) return;`，因此成功必须返回 truthy，
//
//	否则会“数据已还原但界面不刷新/无提示”，看起来像导入没反应。）
func (a *API) ImportBackup() (bool, error) {
	if a.dialogs.OpenFile == nil {
		return false, fmt.Errorf("文件对话框不可用")
	}
	path, err := a.dialogs.OpenFile("选择备份包（.zip）", []string{"zip"})
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil // 用户取消
	}
	if err := a.reg.restoreBackup(path); err != nil {
		return false, err
	}
	return true, nil
}

// restoreBackup 从 zip 备份包还原数据与图标。
func (r *Registry) restoreBackup(zipPath string) error {
	zipPath = ExpandPath(zipPath)
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无法打开备份包: %w", err)
	}
	defer zr.Close()

	var dataBytes []byte
	type iconFile struct {
		name string
		body []byte
	}
	var icons []iconFile

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return fmt.Errorf("读取备份条目 %s 失败: %w", f.Name, readErr)
		}
		switch {
		case f.Name == "tools.json":
			dataBytes = body
		case strings.HasPrefix(f.Name, "icons/") && len(f.Name) > len("icons/"):
			icons = append(icons, iconFile{name: filepath.Base(f.Name), body: body})
		case strings.HasPrefix(f.Name, "cache/icons/") && len(f.Name) > len("cache/icons/"):
			// 兼容按原版目录布局打包的备份。
			icons = append(icons, iconFile{name: filepath.Base(f.Name), body: body})
		}
	}
	if dataBytes == nil {
		return fmt.Errorf("备份包缺少 tools.json，不是有效的 Armory 备份")
	}
	var d model.RegistryData
	if err := jsonUnmarshal(dataBytes, &d); err != nil {
		return fmt.Errorf("备份数据解析失败: %w", err)
	}
	if len(d.Categories) == 0 {
		return fmt.Errorf("备份包缺少分类数据")
	}
	// 覆盖前自动留底：当前数据快照写 tools.json.bak-<时间戳>，误还原可手捞回来。
	if snap, err := jsonMarshalIndent(r.Snapshot()); err == nil {
		bak := filepath.Join(store.DataDir(), "tools.json.bak-"+time.Now().Format("20060102-150405"))
		_ = os.WriteFile(bak, snap, 0o600)
	}
	if err := r.ReplaceAll(d); err != nil {
		return err
	}
	// 还原图标（尽量写入；单个失败不中断，但记录首个错误）。
	var firstErr error
	for _, ic := range icons {
		if err := os.WriteFile(filepath.Join(store.IconDir(), ic.name), ic.body, 0o644); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return fmt.Errorf("部分图标还原失败: %w", firstErr)
	}
	return nil
}

// copyFile 原子性不强求，直接整文件拷贝。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
