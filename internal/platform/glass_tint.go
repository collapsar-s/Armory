//go:build darwin && cgo

package platform

/*
#cgo CFLAGS: -x objective-c
#import <AppKit/AppKit.h>

// 遍历 contentView 直接子视图定位 NSGlassEffectView（Wails 创建 Liquid Glass 时
// 挂在最底层，见 wails webview_window_darwin.m：[contentView addSubview:glassView
// positioned:NSWindowBelow]）。dark=true 时叠加深色 tint（压住浅玻璃，保证深色
// 主题对比度）；dark=false 时清除 tint（恢复纯玻璃）。返回 false = 未找到玻璃视图。
static bool armorySetGlassTint(void *nswindow, int r, int g, int b, double a) {
	@autoreleasepool {
		NSWindow *win = (__bridge NSWindow *)nswindow;
		if (!win) return false;
		NSView *content = win.contentView;
		if (!content) return false;
		Class glassClass = NSClassFromString(@"NSGlassEffectView");
		for (NSView *sub in content.subviews) {
			if (glassClass && [sub isKindOfClass:glassClass]) {
				if (a <= 0) {
					[sub performSelector:@selector(setTintColor:) withObject:nil];
				} else {
					NSColor *c = [NSColor colorWithRed:r/255.0 green:g/255.0 blue:b/255.0 alpha:a];
					[sub performSelector:@selector(setTintColor:) withObject:c];
				}
				return true;
			}
		}
		return false;
	}
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 深色主题玻璃 tint：深蓝灰底 + 78% 不透明度——压住浅色玻璃但保留透光质感。
var darkGlassTint = struct {
	r, g, b int
	a       float64
}{22, 24, 30, 0.78}

// SetGlassDark 切换原生玻璃层的深浅观感（应用内主题与系统外观脱节的修复）。
// 玻璃层样式是创建时按系统外观定的（Automatic），应用内强制深色时若系统是浅色，
// 会出现“深色文字压浅玻璃”的对比度崩坏——用深色 tint 校正。
func SetGlassDark(w *application.WebviewWindow, dark bool) {
	application.InvokeSyncWithResult(func() bool {
		if dark {
			return bool(C.armorySetGlassTint(w.NativeWindow(),
				C.int(darkGlassTint.r), C.int(darkGlassTint.g), C.int(darkGlassTint.b),
				C.double(darkGlassTint.a)))
		}
		return bool(C.armorySetGlassTint(w.NativeWindow(), 0, 0, 0, 0))
	})
}
