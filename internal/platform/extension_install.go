package platform

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/maxischmaxi/browser-proxy/extension"
)

// chromiumBrowsers is the set we attempt to auto-register the native-
// messaging host for during `browser-proxy install`. We only write the
// manifest if the browser's profile directory already exists — i.e. the
// user has launched it at least once. Browsers that aren't installed are
// silently skipped.
var chromiumBrowsers = []string{
	"chrome", "chrome-beta", "chrome-canary", "chromium",
	"brave", "edge", "vivaldi", "arc", "opera",
}

// ExtensionAssetsDir returns the per-OS directory where `install` extracts
// the bundled extension files. Users point Chrome's "Load unpacked" at the
// `chrome` subdirectory of this path.
func ExtensionAssetsDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "browser-proxy", "extension")
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "browser-proxy", "extension")
	}
	return filepath.Join(home, ".local", "share", "browser-proxy", "extension")
}

// installExtensionAssets writes the embedded extension files into
// ExtensionAssetsDir(). Existing files are overwritten so a reinstall picks
// up new versions of background.js / manifest.json. Returns the absolute
// path of the chrome/ subdirectory (the one users load into Chrome).
func installExtensionAssets() (string, error) {
	root := ExtensionAssetsDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", root, err)
	}

	err := fs.WalkDir(extension.Files, "chrome", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(root, p)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := extension.Files.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		return "", fmt.Errorf("extract extension: %w", err)
	}

	return filepath.Join(root, "chrome"), nil
}

// autoRegisterNativeHost writes the native-messaging manifest for every
// Chromium-family browser whose profile directory exists on this machine.
// Returns the list of browsers it actually wrote a manifest for.
//
// Errors per-browser are logged to stderr but do not abort the whole sweep;
// a corrupted Vivaldi config shouldn't stop us from registering with Chrome.
func autoRegisterNativeHost() []string {
	var registered []string
	for _, browser := range chromiumBrowsers {
		dir, err := nativeMessagingDir(browser)
		if err != nil {
			continue
		}
		// The PARENT of NativeMessagingHosts/ is the browser's profile root
		// — exists iff the user has launched the browser at least once.
		if _, err := os.Stat(filepath.Dir(dir)); os.IsNotExist(err) {
			continue
		}
		if _, err := InstallNativeMessagingHost(browser, extension.ExtensionID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: native-host registration for %s failed: %v\n", browser, err)
			continue
		}
		registered = append(registered, browser)
	}
	return registered
}

// PrintExtensionSetup is the post-install nudge: tells the user where the
// extension files landed, which browsers got auto-registered, and how to
// activate the extension by hand. Called by Install() on both platforms.
func PrintExtensionSetup(chromeDir string, registered []string) {
	fmt.Println()
	fmt.Println("In-browser routing (optional):")
	fmt.Printf("  Extension files:    %s\n", chromeDir)
	if len(registered) == 0 {
		fmt.Println("  Native-messaging:   no Chromium browsers detected — register later with:")
		fmt.Println("                      browser-proxy install-extension <browser>")
	} else {
		fmt.Printf("  Native-messaging:   registered for %s\n", strings.Join(registered, ", "))
	}
	fmt.Println()
	fmt.Println("  To activate, in your browser:")
	fmt.Println("    1. open chrome://extensions (or brave://extensions, edge://extensions, …)")
	fmt.Println("    2. enable 'Developer mode' (top-right toggle)")
	fmt.Println("    3. click 'Load unpacked' and select the directory above")
	fmt.Println("    4. the extension's toolbar icon → 'Test connection' should turn green")
}

// InstallExtensionAssetsAndRegister bundles the two operations called from
// Install(): extract the embedded extension files, then auto-register the
// native-messaging host for every Chromium browser found on the system.
//
// Returns the chrome/ subdirectory and the list of registered browsers.
// Errors here are non-fatal for the calling Install — we still want the
// default-browser registration to succeed even if extension setup fails.
func InstallExtensionAssetsAndRegister() (string, []string, error) {
	chromeDir, err := installExtensionAssets()
	if err != nil {
		return "", nil, err
	}
	return chromeDir, autoRegisterNativeHost(), nil
}

// UninstallExtensionAssetsAndUnregister is the symmetric counterpart called
// from Uninstall(): removes the extracted extension directory and every
// native-messaging manifest we may have written. Best-effort: missing
// pieces are not an error.
func UninstallExtensionAssetsAndUnregister() (removed []string) {
	for _, browser := range chromiumBrowsers {
		if path, ok, err := UninstallNativeMessagingHost(browser); err == nil && ok {
			removed = append(removed, path)
		}
	}
	dir := ExtensionAssetsDir()
	if err := os.RemoveAll(dir); err == nil {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			removed = append(removed, dir)
		}
	}
	return removed
}
