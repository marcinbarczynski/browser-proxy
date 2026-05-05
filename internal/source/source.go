// Package source identifies the application that triggered a URL open.
package source

// Info describes the source application. Empty fields mean unknown/undetected.
//
//   - Name     human-readable name (macOS: localizedName; Linux: /proc comm)
//   - BundleID macOS bundle identifier; always empty on Linux
type Info struct {
	Name     string
	BundleID string
}

func (i Info) Empty() bool { return i.Name == "" && i.BundleID == "" }
