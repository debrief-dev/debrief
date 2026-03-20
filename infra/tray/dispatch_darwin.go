//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// Forward declaration of the Go callback
extern void goTrayStartCallback(void);

// Dispatch the tray start to the main thread.
// NSStatusItem creation must happen on the main thread.
static void dispatchTrayStart(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        goTrayStartCallback();
    });
}
*/
import "C"

import "sync"

var (
	startFunc     func()
	startFuncOnce sync.Once
)

//export goTrayStartCallback
func goTrayStartCallback() {
	if startFunc != nil {
		startFunc()
	}
}

// dispatchStartOnMainThread dispatches the given function to the macOS main
// thread via dispatch_async(dispatch_get_main_queue(), ...).
// This is required because NSStatusItem/NSMenu must be created on the main thread.
func dispatchStartOnMainThread(f func()) {
	startFuncOnce.Do(func() {
		startFunc = f
		C.dispatchTrayStart()
	})
}
