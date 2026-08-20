// Package source identifies the application that triggered a URL open.
package source

// Info describes the source application. Empty fields mean unknown.
type Info struct {
	Name       string
	BundleID   string
	Candidates []string
	Via        string
}

func (i Info) Empty() bool { return i.Name == "" && i.BundleID == "" && len(i.Candidates) == 0 }
