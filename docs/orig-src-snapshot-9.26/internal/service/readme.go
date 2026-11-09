package service

import (
	"os"
	"path/filepath"
	"strings"
)

// 工具说明文档读取（M4-3）：扫描工具所在目录的 README / 说明文件，
// 详情（编辑）弹层只读渲染。找不到返回空串，前端隐藏该区块。

// readmeCandidates 按优先级排列的说明文件名。
var readmeCandidates = []string{
	"README.md", "README.txt", "README", "readme.md", "Readme.md",
	"说明.md", "说明.txt", "使用说明.md", "USAGE.md",
}

const maxReadmeSize = 64 << 10 // 64KB，超出截断（弹层展示足够）

// GetToolReadme 返回工具目录下的说明文档内容（markdown/纯文本），无则空串。
func (a *API) GetToolReadme(toolID string) string {
	tool, err := a.Reg.GetTool(toolID)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(ExpandPath(tool.Path))
	for _, name := range readmeCandidates {
		p := filepath.Join(dir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if len(content) > maxReadmeSize {
			content = content[:maxReadmeSize] + "\n\n…（内容过长已截断）"
		}
		return content
	}
	return ""
}

var _ = errors.New // 占位避免空 import（errors 由后续扩展使用）
