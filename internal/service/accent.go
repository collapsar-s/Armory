package service

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// 系统强调色读取：macOS 把用户设置的强调色存在全局偏好 AppleHighlightColor 中
// （仅当用户改过默认「多配色」后才有该键），值为 "R G B 名称" 三个 0~1 浮点数，
// 如 "0.949020 0.780392 0.290196 Yellow"。读取失败或格式不符时回退系统蓝。

// accentDefault 默认强调色（macOS 系统蓝，与原 CSS 变量一致）。
const accentDefault = "#0a84ff"

var (
	accentOnce  sync.Once
	accentValue string
)

// SystemAccentColor 返回系统强调色（十六进制 #RRGGBB），进程内只读取一次。
func SystemAccentColor() string {
	accentOnce.Do(func() {
		out, err := exec.Command("defaults", "read", "-g", "AppleHighlightColor").Output()
		if err != nil {
			accentValue = accentDefault // 键不存在（默认多配色）或读取失败
			return
		}
		if hex := accentHexFromOutput(string(out)); hex != "" {
			accentValue = hex
		} else {
			accentValue = accentDefault
		}
	})
	return accentValue
}

// accentHexFromOutput 解析 defaults read 的输出为 #rrggbb；格式不符返回空串。
// 独立成纯函数便于单测（不依赖 defaults 命令）。
func accentHexFromOutput(out string) string {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 3 {
		return ""
	}
	hex := "#"
	for _, f := range fields[:3] {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil || v < 0 || v > 1 {
			return ""
		}
		c := int(math.Round(v * 255))
		if c > 255 {
			c = 255
		}
		hex += fmt.Sprintf("%02x", c) // 每通道固定两位
	}
	return strings.ToLower(hex)
}
