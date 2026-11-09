package service

import (
	"os"
	"path/filepath"
)

// readme.go 查找并读取工具的说明文档（README / 说明.txt 等）。

// readmeCandidates 为可识别的说明文档文件名（按优先级排列，与原版一致）。
var readmeCandidates = []string{"README.md", "README.txt", "readme.md", "readme.txt", "使用说明.md", "说明.txt", "说明.md"}

// GetToolReadme 返回工具所在目录下说明文档的原始内容；找不到时返回空串。
func (r *Registry) GetToolReadme(id string) (string, error) {
	t, err := r.GetTool(id)
	if err != nil {
		return "", err
	}
	dir := readmeDirForTool(t.Path, t.Type)
	if dir == "" {
		return "", nil
	}
	for _, name := range readmeCandidates {
		p := filepath.Join(dir, name)
		if b, err := os.ReadFile(p); err == nil {
			return string(b), nil
		}
	}
	return "", nil
}

// readmeDirForTool 计算工具说明文档可能所在的目录。
func readmeDirForTool(path, typ string) string {
	path = ExpandPath(path)
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}
