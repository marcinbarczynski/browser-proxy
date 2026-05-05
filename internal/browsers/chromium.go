package browsers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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
	// 2. Display-name match (user wrote "Work")
	for dir, info := range ls.Profile.InfoCache {
		if strings.EqualFold(info.Name, spec) {
			return dir
		}
	}
	return ""
}

// chromiumLocalStatePaths returns the candidate Local State paths for the
// given browser name on the current OS. Order doesn't matter because the
// patterns are mutually exclusive.
func chromiumLocalStatePaths(browser string) []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	n := strings.ToLower(browser)

	switch runtime.GOOS {
	case "darwin":
		base := filepath.Join(home, "Library", "Application Support")
		switch {
		case strings.Contains(n, "chrome canary"):
			return []string{filepath.Join(base, "Google", "Chrome Canary", "Local State")}
		case strings.Contains(n, "chromium"):
			return []string{filepath.Join(base, "Chromium", "Local State")}
		case strings.Contains(n, "chrome"):
			return []string{filepath.Join(base, "Google", "Chrome", "Local State")}
		case strings.Contains(n, "brave"):
			return []string{filepath.Join(base, "BraveSoftware", "Brave-Browser", "Local State")}
		case strings.Contains(n, "edge"):
			return []string{filepath.Join(base, "Microsoft Edge", "Local State")}
		case strings.Contains(n, "vivaldi"):
			return []string{filepath.Join(base, "Vivaldi", "Local State")}
		case strings.Contains(n, "opera"):
			return []string{filepath.Join(base, "com.operasoftware.Opera", "Local State")}
		case strings.Contains(n, "arc"):
			return []string{filepath.Join(base, "Arc", "User Data", "Local State")}
		}
	case "linux":
		base := filepath.Join(home, ".config")
		switch {
		case strings.Contains(n, "chromium"):
			return []string{filepath.Join(base, "chromium", "Local State")}
		case strings.Contains(n, "google-chrome"), strings.Contains(n, "chrome"):
			return []string{filepath.Join(base, "google-chrome", "Local State")}
		case strings.Contains(n, "brave"):
			return []string{filepath.Join(base, "BraveSoftware", "Brave-Browser", "Local State")}
		case strings.Contains(n, "edge"):
			return []string{filepath.Join(base, "microsoft-edge", "Local State")}
		case strings.Contains(n, "vivaldi"):
			return []string{filepath.Join(base, "vivaldi", "Local State")}
		case strings.Contains(n, "opera"):
			return []string{filepath.Join(base, "opera", "Local State")}
		case strings.Contains(n, "thorium"):
			return []string{filepath.Join(base, "thorium", "Local State")}
		}
	}
	return nil
}
