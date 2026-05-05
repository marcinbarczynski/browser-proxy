//go:build linux

package platform

// RunDaemon is a no-op on Linux. The .desktop file passes the URL via argv,
// so the binary is invoked once per click and exits after handling.
func RunDaemon(_ func(string)) {}

// IsBundleStart is always false on Linux.
func IsBundleStart() bool { return false }
