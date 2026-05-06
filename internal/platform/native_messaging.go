package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NativeMessagingHostName is the name advertised in the manifest. The
// browser extension's `sendNativeMessage` calls must use this exact string.
const NativeMessagingHostName = "com.maxischmaxi.browser_proxy"

type nativeMessagingManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// InstallNativeMessagingHost writes the Chrome native-messaging host manifest
// for the given Chromium-family browser, allowing the extension with id
// extensionID to invoke us. Returns the absolute path of the written file.
func InstallNativeMessagingHost(browser, extensionID string) (string, error) {
	dir, err := nativeMessagingDir(browser)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate binary: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", err
	}

	manifest := nativeMessagingManifest{
		Name:           NativeMessagingHostName,
		Description:    "Browser Proxy native-messaging host for in-browser link routing",
		Path:           exe,
		Type:           "stdio",
		AllowedOrigins: []string{"chrome-extension://" + extensionID + "/"},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	path := filepath.Join(dir, NativeMessagingHostName+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// UninstallNativeMessagingHost deletes the manifest. Reports whether a file
// was actually removed (false if it didn't exist) and the path checked.
func UninstallNativeMessagingHost(browser string) (string, bool, error) {
	dir, err := nativeMessagingDir(browser)
	if err != nil {
		return "", false, err
	}
	path := filepath.Join(dir, NativeMessagingHostName+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, false, nil
	} else if err != nil {
		return path, false, err
	}
	if err := os.Remove(path); err != nil {
		return path, false, err
	}
	return path, true, nil
}

// nativeMessagingDir resolves the per-browser, per-OS directory where the
// browser looks for native-messaging host manifests. The browser argument
// is a short name; case-insensitive.
func nativeMessagingDir(browser string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	key := strings.ToLower(strings.TrimSpace(browser))
	if runtime.GOOS == "darwin" {
		base := filepath.Join(home, "Library", "Application Support")
		switch key {
		case "chrome", "google-chrome":
			return filepath.Join(base, "Google", "Chrome", "NativeMessagingHosts"), nil
		case "chrome-beta", "google-chrome-beta":
			return filepath.Join(base, "Google", "Chrome Beta", "NativeMessagingHosts"), nil
		case "chrome-canary", "google-chrome-canary":
			return filepath.Join(base, "Google", "Chrome Canary", "NativeMessagingHosts"), nil
		case "chromium":
			return filepath.Join(base, "Chromium", "NativeMessagingHosts"), nil
		case "brave":
			return filepath.Join(base, "BraveSoftware", "Brave-Browser", "NativeMessagingHosts"), nil
		case "edge", "microsoft-edge":
			return filepath.Join(base, "Microsoft Edge", "NativeMessagingHosts"), nil
		case "vivaldi":
			return filepath.Join(base, "Vivaldi", "NativeMessagingHosts"), nil
		case "arc":
			return filepath.Join(base, "Arc", "User Data", "NativeMessagingHosts"), nil
		}
		return "", fmt.Errorf("unsupported browser %q on macOS (try: chrome, chrome-beta, chrome-canary, chromium, brave, edge, vivaldi, arc)", browser)
	}

	// Linux + others: ~/.config/<browser>/NativeMessagingHosts
	base := filepath.Join(home, ".config")
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		base = x
	}
	switch key {
	case "chrome", "google-chrome":
		return filepath.Join(base, "google-chrome", "NativeMessagingHosts"), nil
	case "chrome-beta", "google-chrome-beta":
		return filepath.Join(base, "google-chrome-beta", "NativeMessagingHosts"), nil
	case "chrome-unstable", "google-chrome-unstable":
		return filepath.Join(base, "google-chrome-unstable", "NativeMessagingHosts"), nil
	case "chromium":
		return filepath.Join(base, "chromium", "NativeMessagingHosts"), nil
	case "brave":
		return filepath.Join(base, "BraveSoftware", "Brave-Browser", "NativeMessagingHosts"), nil
	case "edge", "microsoft-edge":
		return filepath.Join(base, "microsoft-edge", "NativeMessagingHosts"), nil
	case "vivaldi":
		return filepath.Join(base, "vivaldi", "NativeMessagingHosts"), nil
	case "opera":
		return filepath.Join(base, "opera", "NativeMessagingHosts"), nil
	}
	return "", fmt.Errorf("unsupported browser %q on Linux (try: chrome, chrome-beta, chromium, brave, edge, vivaldi, opera)", browser)
}
