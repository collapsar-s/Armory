//go:build darwin && cgo

package platform

/*
#cgo CFLAGS: -x objective-c
#import <AppKit/AppKit.h>

// 恢复红绿灯三按钮的可见与可用状态。
//
// 窗口 styleMask 为 Titled|Closable|Miniaturizable|Resizable|FullSizeContentView
// 时 AppKit 以标准位置（左上角、系统标准间距）放置三个按钮，并挂好系统原生动作
// （红=关闭，黄=最小化，绿=缩放/全屏）。Wails 在 frameless 模式下会把它们隐藏
// （applyWindowButtonStates），这里仅取消隐藏/禁用，恢复与系统红绿灯完全一致的
// 外观与行为。返回 false 表示窗口原生视图尚未就绪（按钮不存在）。
static bool armoryRestoreTrafficLights(void *nswindow) {
	@autoreleasepool {
		NSWindow *win = (__bridge NSWindow *)nswindow;
		if (!win) return false;
		NSButton *closeButton = [win standardWindowButton:NSWindowCloseButton];
		NSButton *minButton = [win standardWindowButton:NSWindowMiniaturizeButton];
		NSButton *zoomButton = [win standardWindowButton:NSWindowZoomButton];
		if (!closeButton || !minButton || !zoomButton) {
			return false;
		}
		closeButton.hidden = NO;
		closeButton.enabled = YES;
		minButton.hidden = NO;
		minButton.enabled = YES;
		zoomButton.hidden = NO;
		zoomButton.enabled = YES;
		return true;
	}
}
*/
import "C"

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// OffsetTrafficLights 恢复系统红绿灯按钮的显示与原生行为。
//
// 背景：Wails frameless 模式会隐藏三键；窗口原生视图就绪前 standardWindowButton
// 可能返回 nil，因此内部短重试（200ms × 5s），与 9-27“启动后校正一次”行为一致。
func OffsetTrafficLights(w *application.WebviewWindow) {
	go func() {
		for i := 0; i < 25; i++ {
			time.Sleep(200 * time.Millisecond)
			ok := application.InvokeSyncWithResult(func() bool {
				// NativeWindow 需在主线程读取（impl 非线程安全）
				return bool(C.armoryRestoreTrafficLights(w.NativeWindow()))
			})
			if ok {
				return
			}
		}
	}()
}
