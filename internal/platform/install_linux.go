//go:build linux

package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const desktopFile = "browser-proxy.desktop"

const desktopTemplate = `[Desktop Entry]
Type=Application
Version=1.0
Name=Browser Proxy
GenericName=Web Browser Router
Comment=Routes URLs to the configured browser
Exec={{.Binary}} open %u
NoDisplay=true
StartupNotify=false
MimeType=x-scheme-handler/http;x-scheme-handler/https;text/html;application/xhtml+xml;
`

func appsDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "applications")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share", "applications")
}

// Install writes a .desktop file for the running binary and registers it
// with xdg-mime / xdg-settings as the default web browser.
func Install() error {
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate binary: %w", err)
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}

	dir := appsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmpl := template.Must(template.New("d").Parse(desktopTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Binary string }{Binary: binary}); err != nil {
		return err
	}

	target := filepath.Join(dir, desktopFile)
	if err := os.WriteFile(target, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	if err := exec.Command("update-desktop-database", dir).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: update-desktop-database failed: %v\n", err)
	}
	if err := exec.Command("xdg-mime", "default", desktopFile,
		"x-scheme-handler/http", "x-scheme-handler/https").Run(); err != nil {
		return fmt.Errorf("xdg-mime default: %w", err)
	}
	if err := exec.Command("xdg-settings", "set", "default-web-browser", desktopFile).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: xdg-settings failed: %v\n", err)
	}

	fmt.Printf("Installed: %s\n", target)
	fmt.Println("Some apps cache the default browser — log out/in if a click still goes to the previous one.")

	// v1.0.0 – v1.0.4 also installed a Chrome companion extension and
	// wrote native-messaging manifests into every Chromium browser's
	// config dir. The feature was dropped in v1.1.0 — sweep up any
	// leftovers from those versions on every install.
	for _, p := range cleanupLegacyExtension() {
		fmt.Printf("Removed leftover: %s\n", p)
	}

	// Install never touches ~/.config/browser-proxy/config.toml — that's
	// `init`'s job.
	return nil
}

// Uninstall removes the .desktop file and any leftover in-browser-
// extension artifacts from v1.0.0–v1.0.4 if they exist. Leaves
// config.toml alone. The user has to set a new default browser.
func Uninstall() error {
	target := filepath.Join(appsDir(), desktopFile)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = exec.Command("update-desktop-database", appsDir()).Run()
	fmt.Printf("Removed: %s\n", target)

	for _, p := range cleanupLegacyExtension() {
		fmt.Printf("Removed: %s\n", p)
	}

	fmt.Println("Set a new default browser via xdg-settings or your DE settings.")
	return nil
}
