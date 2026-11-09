//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#import <AppKit/AppKit.h>

// 将红绿灯按钮的容器视图整体偏移（系统无公开 API，属视图层级操作）。
// 返回 false 表示窗口原生视图尚未就绪（按钮不存在）。
static bool armoryOffsetTrafficLights(void *nswindow, double dx, double dy) {
	@autoreleasepool {
		NSWindow *win = (__bridge NSWindow *)nswindow;
		if (!win) return false;
		NSButton *btn = [win standardWindowButton:NSWindowCloseButton];
		if (!btn) return false;
		NSView *container = [btn superview];
		if (!container) return false;
		NSRect f = container.frame;
		f.origin.x += dx;   // 右移（x 轴向右为正）
		f.origin.y -= dy;   // 下移（AppKit y 轴向上为正）
		[container setFrame:f];
		return true;
	}
}
*/
import "C"

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// OffsetTrafficLights 将系统红绿灯按钮向内偏移：右移 dx、下移 dy（逻辑像素）。
// 背景：Wails beta.26 未暴露红绿灯位置配置（任务书外补能力）；
// 窗口原生视图就绪前操作会失败，因此内部以 200ms 间隔重试，最长 5 秒。
// 注意：进入全屏后系统会重建标题栏，偏移会失效（启动器场景可接受）。
func OffsetTrafficLights(w *application.WebviewWindow, dx, dy float64) {
	go func() {
		for i := 0; i < 25; i++ {
			time.Sleep(200 * time.Millisecond)
			ok := application.InvokeSyncWithResult(func() bool {
				// NativeWindow 需在主线程读取（impl 非线程安全）
				return bool(C.armoryOffsetTrafficLights(w.NativeWindow(), C.double(dx), C.double(dy)))
			})
			if ok {
				return
			}
		}
	}()
}
