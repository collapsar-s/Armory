package service

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func TestAccentHexFromOutput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"标准输出（Yellow）", "0.949020 0.780392 0.290196 Yellow\n", "#f2c74a"},
		{"三通道无名称", "0.698039 0.843137 1.000000", "#b2d7ff"},
		{"多余空白", "  0.000000  1.000000  0.500000  Green ", "#00ff80"},
		{"通道越界", "1.5 0.5 0.5", ""},
		{"负数通道", "-0.1 0.5 0.5", ""},
		{"非数字", "red green blue", ""},
		{"只有两通道", "0.5 0.5", ""},
		{"空输出", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := accentHexFromOutput(c.in); got != c.want {
				t.Errorf("accentHexFromOutput(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFriendlyStartError(t *testing.T) {
	// 模拟 exec.LookPath 失败后 cmd.Start 返回的 *exec.Error。
	underlying := &exec.Error{Name: "java", Err: exec.ErrNotFound}
	got := friendlyStartError(underlying)
	want := "启动失败: 未找到 java，请确认已安装；jar/py 工具可在「设置 → 运行环境」中配置版本"
	if got.Error() != want {
		t.Errorf("friendlyStartError = %q, want %q", got.Error(), want)
	}
	if _, ok := got.(interface{ Unwrap() error }); ok {
		if !errors.Is(got, underlying) {
			t.Error("friendlyStartError 应保留原始错误链（errors.Is）")
		}
	}
	// 其它错误原样带「启动失败:」前缀。
	other := fmt.Errorf("permission denied")
	if g := friendlyStartError(other).Error(); g != "启动失败: permission denied" {
		t.Errorf("friendlyStartError(other) = %q", g)
	}
}
