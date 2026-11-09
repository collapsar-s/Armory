//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>

// 窗口级淡入：alpha 0 → orderFront → animator 1（Core Animation，系统合成器 GPU 动画，
// 与 Launchpad 同机制）。调用方保证 nswindow 在主线程获取。
static void armoryShowAnimated(void *nswindow, double duration) {
	@autoreleasepool {
		NSWindow *win = (__bridge NSWindow *)nswindow;
		if (!win) return;
		[win setAlphaValue:0.0];              // 先置透明再显示，杜绝首帧白闪
		[win makeKeyAndOrderFront:nil];
		[NSAnimationContext beginGrouping];
		NSAnimationContext.currentContext.duration = duration;
		NSAnimationContext.currentContext.timingFunction =
			[CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseOut];
		[[win animator] setAlphaValue:1.0];
		[NSAnimationContext endGrouping];
	}
}

// 窗口级淡出：animator alpha 1 → 0，完成后 orderOut 并复位 alpha（供下次显示）。
static void armoryHideAnimated(void *nswindow, double duration) {
	@autoreleasepool {
		NSWindow *win = (__bridge NSWindow *)nswindow;
		if (!win) return;
		if (![win isVisible]) return;         // 已隐藏则忽略，避免复位态卡死
		[NSAnimationContext beginGrouping];
		NSAnimationContext.currentContext.duration = duration;
		NSAnimationContext.currentContext.timingFunction =
			[CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseIn];
		NSAnimationContext.currentContext.completionHandler = ^(void){
			[win orderOut:nil];
			[win setAlphaValue:1.0];          // 复位，下次 Show 从不透明起步
		};
		[[win animator] setAlphaValue:0.0];
		[NSAnimationContext endGrouping];
	}
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Launchpad 对标动画时长（秒）：淡入稍长带浮起感，淡出干脆。
const (
	showAnimDuration = 0.22
	hideAnimDuration = 0.16
)

// ShowWindowAnimated 窗口淡入显示（原生 alpha 动画）。
// 主线程单次同步块内完成：置透明 → 显示聚焦 → 启动淡入，无首帧白闪。
func ShowWindowAnimated(w *application.WebviewWindow) {
	application.InvokeSyncWithResult(func() bool {
		C.armoryShowAnimated(w.NativeWindow(), C.double(showAnimDuration))
		return true
	})
	w.Focus() // 聚焦等 Wails 层收尾（webview 键盘焦点），不阻塞 alpha 动画
}

// HideWindowAnimated 窗口淡出隐藏；动画完成后窗口 orderOut。
func HideWindowAnimated(w *application.WebviewWindow) {
	application.InvokeSyncWithResult(func() bool {
		C.armoryHideAnimated(w.NativeWindow(), C.double(hideAnimDuration))
		return true
	})
}
