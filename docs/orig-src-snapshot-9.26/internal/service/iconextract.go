package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 图标缓存目录（任务书 §2.4：~/.armory/cache/icons/）。
const iconCacheDir = "cache/icons"

// ExtractAppIcon 从 .app 包提取图标并转为 PNG（macOS 专用：defaults 读 plist + sips 转格式）。
// 返回 PNG 输出路径。
func ExtractAppIcon(appPath, outPNG string) error {
	resDir := filepath.Join(appPath, "Contents", "Resources")
	if _, err := os.Stat(resDir); err != nil {
		return fmt.Errorf("不是有效的 .app 包: %s", appPath)
	}

	// 1. 优先读 Info.plist 声明的图标名
	icns := ""
	if out, err := exec.Command("defaults", "read",
		filepath.Join(appPath, "Contents", "Info"), "CFBundleIconFile").Output(); err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			if !strings.HasSuffix(name, ".icns") {
				name += ".icns"
			}
			candidate := filepath.Join(resDir, name)
			if _, err := os.Stat(candidate); err == nil {
				icns = candidate
			}
		}
	}

	// 2. 兜底：取 Resources 下第一个 icns
	if icns == "" {
		entries, err := os.ReadDir(resDir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".icns") {
				icns = filepath.Join(resDir, e.Name())
				break
			}
		}
	}
	if icns == "" {
		return errors.New("未在 .app 中找到 icns 图标")
	}

	// 3. sips 转 PNG（系统自带，无需第三方依赖）
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}
	return exec.Command("sips", "-s", "format", "png", icns, "--out", outPNG).Run()
}

// IconPathFor 工具图标缓存路径。
func IconPathFor(dataDir, toolID string) string {
	return filepath.Join(dataDir, iconCacheDir, toolID+".png")
}
