package browsers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// FirefoxProfile is one entry from a Firefox-family profiles.ini.
//
// The Name is the user-visible label and is also the value that goes into
// `firefox -P "<name>"` — i.e. exactly the value users put in the rule's
// `profile = "..."` field. Path is the on-disk directory (under the profile
// root); shown for context.
type FirefoxProfile struct {
	Name      string
	Path      string
	IsDefault bool
}

// ListFirefoxProfiles enumerates profiles for a Firefox-family browser.
func ListFirefoxProfiles(browser string) (sourcePath string, profiles []FirefoxProfile, err error) {
	paths := firefoxProfilesIniPaths(browser)
	if len(paths) == 0 {
		return "", nil, fmt.Errorf("%q is not a recognised Firefox-family browser", browser)
	}
	for _, p := range paths {
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			continue
		}
		profiles := parseFirefoxProfiles(string(data))
		if len(profiles) == 0 {
			return p, nil, fmt.Errorf("profiles.ini %s has no profiles (start Firefox once to create one)", p)
		}
		return p, profiles, nil
	}
	return "", nil, fmt.Errorf("no profiles.ini found for %q (looked at %v)", browser, paths)
}

// parseFirefoxProfiles reads a Firefox profiles.ini and returns its profile
// entries. The "default" flag is sourced from either the per-profile Default=1
// line OR the [InstallXXX] Default=<path> entry (which is more authoritative
// in modern Firefox — it's the profile chosen at startup).
func parseFirefoxProfiles(content string) []FirefoxProfile {
	sections := parseINI(content)
	var profiles []FirefoxProfile
	var installDefault string

	for _, sec := range sections {
		switch {
		case strings.HasPrefix(sec.Name, "Profile"):
			p := FirefoxProfile{
				Name: sec.Fields["Name"],
				Path: sec.Fields["Path"],
			}
			if sec.Fields["Default"] == "1" {
				p.IsDefault = true
			}
			if p.Name != "" {
				profiles = append(profiles, p)
			}
		case strings.HasPrefix(sec.Name, "Install"):
			if d := sec.Fields["Default"]; d != "" {
				installDefault = d
			}
		}
	}

	if installDefault != "" {
		for i := range profiles {
			if profiles[i].Path == installDefault {
				profiles[i].IsDefault = true
			}
		}
	}

	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles
}

// iniSection holds one parsed [Section] and its key=value fields.
type iniSection struct {
	Name   string
	Fields map[string]string
}

// parseINI is a tiny tolerant INI reader: lower-cased keys preserved as-is,
// blank/comment lines skipped, no quoting/escaping. Sufficient for the
// straight-forward Firefox profiles.ini format.
func parseINI(content string) []iniSection {
	var sections []iniSection
	var current *iniSection

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &iniSection{
				Name:   line[1 : len(line)-1],
				Fields: map[string]string{},
			}
			continue
		}
		if current == nil {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		current.Fields[key] = val
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}

// firefoxProfilesIniPaths returns candidate profiles.ini paths for the given
// browser name. LibreWolf, Waterfox use their own dirs; Firefox/ESR/Tor
// share ~/.mozilla/firefox/.
func firefoxProfilesIniPaths(browser string) []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	return firefoxProfilesIniPathsImpl(browser, home, runtime.GOOS)
}

func firefoxProfilesIniPathsImpl(browser, home, goos string) []string {
	if home == "" {
		return nil
	}
	n := strings.NewReplacer(".", " ", "-", " ", "_", " ", "/", " ").
		Replace(strings.ToLower(browser))
	var paths []string

	switch goos {
	case "linux":
		switch {
		case strings.Contains(n, "librewolf"):
			paths = append(paths, filepath.Join(home, ".librewolf", "profiles.ini"))
		case strings.Contains(n, "waterfox"):
			paths = append(paths, filepath.Join(home, ".waterfox", "profiles.ini"))
		case strings.Contains(n, "firefox") || strings.Contains(n, "tor browser") || strings.Contains(n, "icecat") || strings.Contains(n, "zen"):
			paths = append(paths, filepath.Join(home, ".mozilla", "firefox", "profiles.ini"))
		}
	case "darwin":
		base := filepath.Join(home, "Library", "Application Support")
		switch {
		case strings.Contains(n, "librewolf"):
			paths = append(paths, filepath.Join(base, "LibreWolf", "profiles.ini"))
		case strings.Contains(n, "waterfox"):
			paths = append(paths, filepath.Join(base, "Waterfox", "profiles.ini"))
		case strings.Contains(n, "firefox") || strings.Contains(n, "tor browser"):
			paths = append(paths, filepath.Join(base, "Firefox", "profiles.ini"))
		}
	}
	return paths
}
