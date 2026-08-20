//go:build !linux

package source

// DebugLines has nothing to report off Linux; detailed diagnostics are provided
// by the Linux detector only.
func DebugLines() []string { return nil }
