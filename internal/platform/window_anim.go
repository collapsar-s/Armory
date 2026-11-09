//go:build darwin && cgo

package platform

/*
#cgo CFLAGS: -x objective-c
#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>

// 窗口级淡入：alpha 0 → orderFront → animator 1（Core Animation，系统合成器 GPU 动画，
// 与 Launchpad 同机制）。9-27 实测行为：0.18s EaseOut，起始 frame 向内缩 2%
// （Spotlight 式“弹出”手感）。调用方保证 nswindow 在主线程获取。
static void armoryShowAnimated(void *nswindow, double duration) {
	@autoreleasepool {
		NSWindow *win = (__bridge NSWindow *)nswindow;
		if (!win) return;
		[win setAlphaValue:0.0];              // 先置透明再显示，杜绝首帧白闪
		NSRect targetFrame = [win frame];
		NSRect startFrame = targetFrame;
		CGFloat shrinkX = targetFrame.size.width * 0.02;
		CGFloat shrinkY = targetFrame.size.height * 0.02;
		startFrame.origin.x += shrinkX;
		startFrame.origin.y += shrinkY;
		startFrame.size.width -= shrinkX * 2;
		startFrame.size.height -= shrinkY * 2;
		[win setFrame:startFrame display:NO];
		[win makeKeyAndOrderFront:nil];

		[NSAnimationContext beginGrouping];
		NSAnimationContext.currentContext.duration = duration;
		NSAnimationContext.currentContext.timingFunction =
			[CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseOut];
		[[win animator] setAlphaValue:1.0];
		[[win animator] setFrame:targetFrame display:YES];
		[NSAnimationContext endGrouping];
	}
}

// 窗口级淡出：animator alpha 1 → 0（frame 同步缩回 2%），完成后还原 frame、
// orderOut 并复位 alpha（供下次显示）。9-27 实测：0.15s EaseIn。
static void armoryHideAnimated(void *nswindow, double duration) {
	@autoreleasepool {
		NSWindow *win = (__bridge NSWindow *)nswindow;
		if (!win) return;
		if (![win isVisible]) return;         // 已隐藏则忽略，避免复位态卡死
		NSRect targetFrame = [win frame];
		NSRect endFrame = targetFrame;
		CGFloat shrinkX = targetFrame.size.width * 0.02;
		CGFloat shrinkY = targetFrame.size.height * 0.02;
		endFrame.origin.x += shrinkX;
		endFrame.origin.y += shrinkY;
		endFrame.size.width -= shrinkX * 2;
		endFrame.size.height -= shrinkY * 2;

		[NSAnimationContext beginGrouping];
		NSAnimationContext.currentContext.duration = duration;
		NSAnimationContext.currentContext.timingFunction =
			[CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseIn];
		NSAnimationContext.currentContext.completionHandler = ^(void){
			[win setFrame:targetFrame display:NO]; // 先还原真实 frame，避免下次显示尺寸错误
			[win orderOut:nil];
			[win setAlphaValue:1.0];               // 复位，下次 Show 从不透明起步
		};
		[[win animator] setAlphaValue:0.0];
		[[win animator] setFrame:endFrame display:YES];
		[NSAnimationContext endGrouping];
	}
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Spotlight 对标动画时长（秒），9-27 实测值：淡入稍长带浮起感，淡出干脆。
const (
	showAnimDuration = 0.18
	hideAnimDuration = 0.15
)

// ShowWindowAnimated 窗口淡入显示（原生 alpha + frame 缩放动画）。
// 主线程单次同步块内完成：置透明 → 缩放 frame → 显示聚焦 → 启动淡入，无首帧白闪。
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
