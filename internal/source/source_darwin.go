//go:build darwin

package source

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>

static void source_detect(char **outName, char **outID) {
    NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
    NSString *name = (front && front.localizedName)    ? front.localizedName    : @"";
    NSString *bid  = (front && front.bundleIdentifier) ? front.bundleIdentifier : @"";
    *outName = strdup([name UTF8String]);
    *outID   = strdup([bid  UTF8String]);
}
*/
import "C"

import "unsafe"

// Detect returns the frontmost macOS application. Cocoa-call; safe to call from
// the Apple-Event handler thread (read-only NSWorkspace query).
func Detect() Info {
	var nameC, idC *C.char
	C.source_detect(&nameC, &idC)
	defer C.free(unsafe.Pointer(nameC))
	defer C.free(unsafe.Pointer(idC))
	return Info{
		Name:     C.GoString(nameC),
		BundleID: C.GoString(idC),
	}
}
