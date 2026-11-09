//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ServiceManagement
#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

// 开机自启（M3-1）：SMAppService 是 macOS 13+ 官方登录项机制，
// 注册后在「系统设置 → 通用 → 登录项」中可见、可由用户管理。
// 注意：SMAppService 只对 .app bundle 有效——裸二进制（dev 模式）注册会报错，
// 这里提前检测给出友好提示。返回 NULL=成功，否则返回需 Go 侧释放的错误串。
static char *armorySetLoginItem(BOOL enable) {
	@autoreleasepool {
		NSString *path = [[NSBundle mainBundle] bundlePath];
		if (![path hasSuffix:@".app"]) {
			return strdup("开发模式（非 .app 运行）无法注册开机自启，请使用 bin/armory.app 正式包");
		}
		NSError *err = nil;
		SMAppService *svc = [SMAppService mainAppService];
		BOOL ok = enable ? [svc registerAndReturnError:&err] : [svc unregisterAndReturnError:&err];
		if (!ok) {
			NSString *msg = err.localizedDescription ?: @"登录项注册失败";
			return strdup(msg.UTF8String);
		}
		return NULL;
	}
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SetLaunchAtLogin 注册/注销开机自启登录项。
// 必须在主线程调用（经 InvokeSyncWithResult），失败返回具体原因
// （典型失败：dev 裸二进制运行、或用户在系统设置里拒绝）。
func SetLaunchAtLogin(enable bool) error {
	return application.InvokeSyncWithResult(func() error {
		cErr := C.armorySetLoginItem(C.bool(enable))
		if cErr == nil {
			return nil
		}
		defer C.free(unsafe.Pointer(cErr))
		return fmt.Errorf("%s", C.GoString(cErr))
	})
}
