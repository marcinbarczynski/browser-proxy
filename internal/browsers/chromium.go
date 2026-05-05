package browsers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ChromiumProfile describes one entry from a Chromium-family Local State.
type ChromiumProfile struct {
	Directory string // "Default" or "Profile N"
	Name      string // user-visible display name from chrome://settings
	Email     string // signed-in account email (Local State user_name); may be empty
}

// ListChromiumProfiles enumerates the profiles stored by a Chromium-family
// browser. Returns the source path actually read so callers can show users
// where the data came from.
func ListChromiumProfiles(browser string) (sourcePath string, profiles []ChromiumProfile, err error) {
	paths := chromiumLocalStatePaths(browser)
	if len(paths) == 0 {
		return "", nil, fmt.Errorf("%q is not a recognised Chromium-family browser", browser)
	}
	for _, p := range paths {
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			continue
		}
		profiles, err := parseChromiumLocalState(data)
		if err != nil {
			return p, nil, fmt.Errorf("parse %s: %w", p, err)
		}
		if len(profiles) == 0 {
			return p, nil, fmt.Errorf("Local State %s has no profiles (start the browser once to create one)", p)
		}
		return p, profiles, nil
	}
	return "", nil, fmt.Errorf("no Local State found for %q (looked at %v)", browser, paths)
}

// parseChromiumLocalState extracts profile entries, sorted by Directory for
// stable output.
func parseChromiumLocalState(data []byte) ([]ChromiumProfile, error) {
	var ls struct {
		Profile struct {
			InfoCache map[string]struct {
				Name     string `json:"name"`
				UserName string `json:"user_name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &ls); err != nil {
		return nil, err
	}
	out := make([]ChromiumProfile, 0, len(ls.Profile.InfoCache))
	for dir, info := range ls.Profile.InfoCache {
		out = append(out, ChromiumProfile{
			Directory: dir,
			Name:      info.Name,
			Email:     info.UserName,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Directory < out[j].Directory })
	return out, nil
}

// ResolveChromiumProfile maps a profile spec to a value suitable for
// --profile-directory=. The spec can be either:
//   - the directory name itself ("Default", "Profile 1") — used as-is
//   - the human-readable display name ("Work", "Personal") — looked up in
//     the browser's Local State and translated to its directory key
//
// If the lookup fails (no Local State, malformed JSON, no match) the spec is
// returned unchanged so users can still specify directory names directly.
func ResolveChromiumProfile(browser, spec string) string {
	for _, p := range chromiumLocalStatePaths(browser) {
		if dir := lookupChromiumProfile(p, spec); dir != "" {
			return dir
		}
	}
	return spec
}

// lookupChromiumProfile reads a Chromium-family Local State JSON and tries to
// match spec against either profile directory keys or display names.
func lookupChromiumProfile(localStatePath, spec string) string {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return ""
	}
	var ls struct {
		Profile struct {
			InfoCache map[string]struct {
				Name string `json:"name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &ls); err != nil {
		return ""
	}
	// 1. Direct directory match (user wrote "Profile 1")
	if _, ok := ls.Profile.InfoCache[spec]; ok {
		return spec
	}
	// 2. Display-name match (case-insensitive)
	for dir, info := range ls.Profile.InfoCache {
		if strings.EqualFold(info.Name, spec) {
			return dir
		}
	}
	return ""
}

// chromiumLocalStatePaths returns Local State path candidates for a given
// browser name on the current OS. Thin wrapper around the pure version
// (testable without depending on os.UserHomeDir / runtime.GOOS).
func chromiumLocalStatePaths(browser string) []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	return chromiumLocalStatePathsImpl(browser, home, runtime.GOOS)
}

// chromiumLocalStatePathsImpl is the pure mapping from browser name + home +
// goos to a Local State path. Inputs are normalised so that dash/dot/space/
// underscore separators all match the same channel — i.e. "google-chrome-beta",
// "Google Chrome Beta" and the macOS bundle ID "com.google.Chrome.beta" all
// resolve to the same channel.
//
// Channels recognised (Chrome family): stable, beta, dev (also called
// "unstable" on Linux), canary. Plus a handful of related Chromium-based
// browsers (Chromium itself, Brave, Edge, Vivaldi, Opera, Arc, Thorium).
func chromiumLocalStatePathsImpl(browser, home, goos string) []string {
	if home == "" {
		return nil
	}
	// Normalise: lowercase + collapse dot/dash/underscore/slash to space.
	// Lets a single keyword like "chrome beta" match every spelling.
	n := strings.NewReplacer(".", " ", "-", " ", "_", " ", "/", " ").
		Replace(strings.ToLower(browser))

	switch goos {
	case "darwin":
		base := filepath.Join(home, "Library", "Application Support")
		switch {
		case strings.Contains(n, "chrome canary"):
			return []string{filepath.Join(base, "Google", "Chrome Canary", "Local State")}
		case strings.Contains(n, "chrome beta"):
			return []string{filepath.Join(base, "Google", "Chrome Beta", "Local State")}
		case strings.Contains(n, "chrome dev"), strings.Contains(n, "chrome unstable"):
			return []string{filepath.Join(base, "Google", "Chrome Dev", "Local State")}
		case strings.Contains(n, "chromium"):
			return []string{filepath.Join(base, "Chromium", "Local State")}
		case strings.Contains(n, "brave"):
			return []string{filepath.Join(base, "BraveSoftware", "Brave-Browser", "Local State")}
		case strings.Contains(n, "edge"), strings.Contains(n, "msedge"):
			return []string{filepath.Join(base, "Microsoft Edge", "Local State")}
		case strings.Contains(n, "vivaldi"):
			return []string{filepath.Join(base, "Vivaldi", "Local State")}
		case strings.Contains(n, "opera"):
			return []string{filepath.Join(base, "com.operasoftware.Opera", "Local State")}
		case strings.Contains(n, "arc"):
			return []string{filepath.Join(base, "Arc", "User Data", "Local State")}
		case strings.Contains(n, "chrome"):
			return []string{filepath.Join(base, "Google", "Chrome", "Local State")}
		}
	case "linux":
		base := filepath.Join(home, ".config")
		switch {
		case strings.Contains(n, "chrome canary"):
			return []string{filepath.Join(base, "google-chrome-canary", "Local State")}
		case strings.Contains(n, "chrome beta"):
			return []string{filepath.Join(base, "google-chrome-beta", "Local State")}
		case strings.Contains(n, "chrome dev"), strings.Contains(n, "chrome unstable"):
			return []string{filepath.Join(base, "google-chrome-unstable", "Local State")}
		case strings.Contains(n, "chromium"):
			return []string{filepath.Join(base, "chromium", "Local State")}
		case strings.Contains(n, "brave"):
			return []string{filepath.Join(base, "BraveSoftware", "Brave-Browser", "Local State")}
		case strings.Contains(n, "edge"), strings.Contains(n, "msedge"):
			return []string{filepath.Join(base, "microsoft-edge", "Local State")}
		case strings.Contains(n, "vivaldi"):
			return []string{filepath.Join(base, "vivaldi", "Local State")}
		case strings.Contains(n, "opera"):
			return []string{filepath.Join(base, "opera", "Local State")}
		case strings.Contains(n, "thorium"):
			return []string{filepath.Join(base, "thorium", "Local State")}
		case strings.Contains(n, "chrome"):
			return []string{filepath.Join(base, "google-chrome", "Local State")}
		}
	}
	return nil
}
