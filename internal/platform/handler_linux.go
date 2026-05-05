//go:build linux

package platform

import "github.com/max/browser-proxy/internal/source"

// RunDaemon is a no-op on Linux. The .desktop file passes the URL via argv,
// so the binary is invoked once per click and exits after handling.
func RunDaemon(_ func(url string, src source.Info)) {}

// IsBundleStart is always false on Linux.
func IsBundleStart() bool { return false }
