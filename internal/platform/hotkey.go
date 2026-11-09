//go:build darwin && cgo

package platform

/*
#cgo CFLAGS: -x objective-c
#import <Carbon/Carbon.h>

extern void armoryHotkeyFired(void);

static OSStatus armoryHotkeyHandler(EventHandlerCallRef nextHandler, EventRef theEvent, void *userData) {
	EventHotKeyID hkID;
	if (GetEventParameter(theEvent, kEventParamDirectObject, typeEventHotKeyID,
		NULL, sizeof(hkID), NULL, &hkID) == noErr &&
		hkID.signature == 'vulp' && hkID.id == 1) {
		armoryHotkeyFired();
		return noErr;
	}
	return CallNextEventHandler(nextHandler, theEvent);
}

static int armoryInstallHotkey(unsigned int modifiers, unsigned int keyCode) {
	EventTypeSpec spec;
	spec.eventClass = kEventClassKeyboard;
	spec.eventKind  = kEventHotKeyPressed;
	OSStatus st = InstallEventHandler(GetApplicationEventTarget(),
		armoryHotkeyHandler, 1, &spec, NULL, NULL);
	if (st != noErr) {
		return (int)st;
	}
	EventHotKeyID hkID;
	hkID.signature = 'vulp';
	hkID.id = 1;
	EventHotKeyRef ref = NULL;
	st = RegisterEventHotKey(keyCode, modifiers, hkID, GetApplicationEventTarget(), 0, &ref);
	return (int)st;
}
*/
import "C"

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var toggleFn func()

//export armoryHotkeyFired
func armoryHotkeyFired() {
	if toggleFn != nil {
		toggleFn()
	}
}

// InstallToggleHotkey 注册全局呼出热键并绑定切换回调。
// modifiers/keyCode 使用 Carbon 常量：⌥=optionKey，Space=kVK_Space(49)。
// 返回卸载函数；失败时返回 OSStatus 错误。
// 实现说明：按原版以 cgo 封装 Carbon RegisterEventHotKey（⌥Space 全局生效，
// 与系统其它全局热键同机制）。
func InstallToggleHotkey(fn func()) (func(), error) {
	toggleFn = fn
	st := C.armoryInstallHotkey(C.uint(C.optionKey), C.uint(49)) // ⌥Space
	if st != 0 {
		toggleFn = nil
		return nil, fmt.Errorf("RegisterEventHotKey failed, OSStatus=%d", int(st))
	}
	return func() { toggleFn = nil }, nil
}

// ToggleWindow 切换主面板显隐（Launchpad 式：原生窗口淡入/淡出，见 window_anim.go）。
// 面板真正显示时回调 onShown（供上层发前端事件，web 层同步播内容回落动画）。
// 关键：Carbon 热键回调运行在主线程，必须用 InvokeAsync 派发，
// 若用 InvokeSync 会与主线程 WaitGroup 互相等待造成死锁。
func ToggleWindow(w *application.WebviewWindow, onShown func()) {
	application.InvokeAsync(func() {
		if w.IsVisible() {
			HideWindowAnimated(w)
		} else {
			ShowWindowAnimated(w)
			if onShown != nil {
				onShown()
			}
		}
	})
}
