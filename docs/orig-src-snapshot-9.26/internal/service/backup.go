package service

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"armory/internal/model"
)

// 备份包格式（M4-4）：zip 内相对路径
//   tools.json      工具库全量（工具/分类/设置）
//   cache/icons/*   图标缓存
// 导入时校验 tools.json 可解析且包含分类字段，替换当前库并热重载。

const backupFile = "tools.json"
const backupIconsPrefix = "cache/icons/"

// ExportBackup 导出工具库到用户选择的位置（对话框取消返回 ""）。
func (a *API) ExportBackup() (string, error) {
	if a.saveFileHook == nil {
		return "", errors.New("导出对话框不可用")
	}
	name := fmt.Sprintf("armory-backup-%s.zip", time.Now().Format("20060102-150405"))
	target, err := a.saveFileHook("导出备份", name)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(target) == "" {
		return "", nil // 用户取消
	}
	if !strings.HasSuffix(strings.ToLower(target), ".zip") {
		target += ".zip"
	}
	if err := a.packBackup(target); err != nil {
		return "", err
	}
	return target, nil
}

func (a *API) packBackup(target string) error {
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()

	add := func(rel string, data []byte) error {
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}

	data, err := os.ReadFile(filepath.Join(a.dataDir, backupFile))
	if err != nil {
		return fmt.Errorf("读取工具库失败: %w", err)
	}
	if err := add(backupFile, data); err != nil {
		return err
	}

	iconsDir := filepath.Join(a.dataDir, "cache", "icons")
	entries, _ := os.ReadDir(iconsDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		iconData, err := os.ReadFile(filepath.Join(iconsDir, e.Name()))
		if err != nil {
			continue // 单个图标缺失不阻塞备份
		}
		if err := add(backupIconsPrefix+e.Name(), iconData); err != nil {
			return err
		}
	}
	return nil
}

// ImportBackup 从用户选择的备份包还原（覆盖当前工具库；对话框取消返回 ""）。
func (a *API) ImportBackup() (string, error) {
	if a.openFileHook == nil {
		return "", errors.New("文件对话框不可用")
	}
	path, err := a.openFileHook("选择备份包（.zip）")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", nil // 用户取消
	}
	if err := a.restoreBackup(path); err != nil {
		return "", err
	}
	return path, nil
}

func (a *API) restoreBackup(zipPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return errors.New("无法读取备份包: " + err.Error())
	}
	defer zr.Close()

	var toolsJSON []byte
	icons := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		switch {
		case f.Name == backupFile:
			toolsJSON = data
		case strings.HasPrefix(f.Name, backupIconsPrefix) && !f.FileInfo().IsDir():
			icons[strings.TrimPrefix(f.Name, backupIconsPrefix)] = data
		}
	}
	if toolsJSON == nil {
		return errors.New("备份包缺少 tools.json，不是有效的 Armory 备份")
	}

	// 预解析校验（不通过则不落盘，防止坏包毁库）
	var snap struct {
		Tools      []model.Tool      `json:"Tools"`
		Categories []model.Category  `json:"Categories"`
		Settings   model.Settings    `json:"Settings"`
	}
	if err := json.Unmarshal(toolsJSON, &snap); err != nil {
		return errors.New("备份包内容损坏: " + err.Error())
	}
	if len(snap.Categories) == 0 {
		return errors.New("备份包缺少分类数据")
	}

	// 落盘：先写 tools.json（经 Registry 热替换），再覆盖图标缓存
	if err := a.Reg.ReplaceAll(snap.Tools, snap.Categories, snap.Settings); err != nil {
		return err
	}
	iconsDir := filepath.Join(a.dataDir, "cache", "icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		return err
	}
	// 清空旧图标再写入备份包里的（备份包可能不含全部旧图标）
	oldEntries, _ := os.ReadDir(iconsDir)
	for _, e := range oldEntries {
		_ = os.Remove(filepath.Join(iconsDir, e.Name()))
	}
	for name, data := range icons {
		if strings.Contains(name, "..") || strings.Contains(name, "/") {
			continue // 防路径穿越
		}
		if err := os.WriteFile(filepath.Join(iconsDir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
