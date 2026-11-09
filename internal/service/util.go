package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"armory/internal/model"
)

// jsonMarshalIndent / jsonUnmarshal 是对 encoding/json 的薄封装，统一序列化风格。
func jsonMarshalIndent(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// ExpandPath 展开路径中的 ~ 与环境变量，返回绝对路径。
// 空白输入原样返回；无法取到 home 时保留原路径。
func ExpandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	p = os.ExpandEnv(p)
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// fileExists 判断路径是否存在（目录不算，与原版一致）。
func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// baseName 返回路径去掉扩展名后的 basename。
func baseName(p string) string {
	b := filepath.Base(p)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// bundleName 返回 .app 包去掉扩展名后的名称。
func bundleName(p string) string {
	return strings.TrimSuffix(filepath.Base(p), ".app")
}

// firstTopDirCategoryID 返回第一个“顶层目录”（非标题）的 id，作为空分类收录时的默认归属。
func firstTopDirCategoryID(cats []model.Category) string {
	for _, c := range cats {
		if c.ParentID == "" && c.Kind != "title" {
			return c.ID
		}
	}
	if len(cats) > 0 {
		return cats[0].ID
	}
	return ""
}
