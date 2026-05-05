//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework Foundation
#include "handler_darwin.h"
*/
import "C"

var urlCh = make(chan string, 32)

//export HandleURL
func HandleURL(curl *C.char) {
	s := C.GoString(curl)
	select {
	case urlCh <- s:
	default:
	}
}

// RunDaemon starts the Cocoa event loop and forwards each received URL to handle.
// It blocks until the application exits.
func RunDaemon(handle func(string)) {
	go func() {
		for u := range urlCh {
			handle(u)
		}
	}()
	C.RunMacApp()
}
