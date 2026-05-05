//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework Foundation -framework AppKit
#include "handler_darwin.h"
*/
import "C"

import "github.com/maxischmaxi/browser-proxy/internal/source"

type incoming struct {
	url string
	src source.Info
}

var urlCh = make(chan incoming, 32)

//export HandleURL
func HandleURL(curl, csrcID, csrcName *C.char) {
	msg := incoming{
		url: C.GoString(curl),
		src: source.Info{
			Name:     C.GoString(csrcName),
			BundleID: C.GoString(csrcID),
		},
	}
	select {
	case urlCh <- msg:
	default:
	}
}

// RunDaemon starts the Cocoa event loop and forwards each URL+source to handle.
// Blocks until the application exits.
func RunDaemon(handle func(url string, src source.Info)) {
	go func() {
		for m := range urlCh {
			handle(m.url, m.src)
		}
	}()
	C.RunMacApp()
}
