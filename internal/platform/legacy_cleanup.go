package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// cleanupLegacyExtension removes anything left behind by the in-browser
// Chrome-extension feature that shipped in v1.0.0–v1.0.4 and was dropped
// in v1.1.0. Best-effort: missing files / directories are not errors,
// nothing is logged unless something exists. Both Install and Uninstall
// call this so a v1.0.x → v1.1.0 upgrade leaves no orphans behind.
//
// Concretely, we delete:
//   - the unpacked extension directory under ~/Library/Application Support/
//     browser-proxy/extension (macOS) or $XDG_DATA_HOME/browser-proxy/extension
//     (Linux)
//   - "com.maxischmaxi.browser_proxy.json" from every Chromium-family
//     browser's NativeMessagingHosts directory we used to write to
//
// Once enough time has passed and most users are on >= v1.1.0 we can
// remove this file entirely.
func cleanupLegacyExtension() (removed []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// 1) The unpacked extension directory.
	var extDir string
	if runtime.GOOS == "darwin" {
		extDir = filepath.Join(home, "Library", "Application Support", "browser-proxy", "extension")
	} else {
		base := filepath.Join(home, ".local", "share")
		if x := os.Getenv("XDG_DATA_HOME"); x != "" {
			base = x
		}
		extDir = filepath.Join(base, "browser-proxy", "extension")
	}
	if _, err := os.Stat(extDir); err == nil {
		if rmErr := os.RemoveAll(extDir); rmErr == nil {
			removed = append(removed, extDir)
		}
	}

	// 2) Per-browser native-messaging manifests we used to install.
	for _, dir := range legacyNativeMessagingDirs(home) {
		manifest := filepath.Join(dir, "com.maxischmaxi.browser_proxy.json")
		if _, err := os.Stat(manifest); err == nil {
			if rmErr := os.Remove(manifest); rmErr == nil {
				removed = append(removed, manifest)
			}
		}
	}
	return removed
}

func legacyNativeMessagingDirs(home string) []string {
	if runtime.GOOS == "darwin" {
		base := filepath.Join(home, "Library", "Application Support")
		return []string{
			filepath.Join(base, "Google", "Chrome", "NativeMessagingHosts"),
			filepath.Join(base, "Google", "Chrome Beta", "NativeMessagingHosts"),
			filepath.Join(base, "Google", "Chrome Canary", "NativeMessagingHosts"),
			filepath.Join(base, "Chromium", "NativeMessagingHosts"),
			filepath.Join(base, "BraveSoftware", "Brave-Browser", "NativeMessagingHosts"),
			filepath.Join(base, "Microsoft Edge", "NativeMessagingHosts"),
			filepath.Join(base, "Vivaldi", "NativeMessagingHosts"),
			filepath.Join(base, "Arc", "User Data", "NativeMessagingHosts"),
		}
	}
	base := filepath.Join(home, ".config")
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		base = x
	}
	return []string{
		filepath.Join(base, "google-chrome", "NativeMessagingHosts"),
		filepath.Join(base, "google-chrome-beta", "NativeMessagingHosts"),
		filepath.Join(base, "google-chrome-canary", "NativeMessagingHosts"),
		filepath.Join(base, "google-chrome-unstable", "NativeMessagingHosts"),
		filepath.Join(base, "chromium", "NativeMessagingHosts"),
		filepath.Join(base, "BraveSoftware", "Brave-Browser", "NativeMessagingHosts"),
		filepath.Join(base, "microsoft-edge", "NativeMessagingHosts"),
		filepath.Join(base, "vivaldi", "NativeMessagingHosts"),
		filepath.Join(base, "opera", "NativeMessagingHosts"),
	}
}
