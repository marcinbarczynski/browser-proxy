//go:build darwin

package platform

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	bundleName     = "Browser Proxy"
	bundleID       = "dev.local.browser-proxy"
	bundleExec     = "browser-proxy"
	bundleVersion  = "0.1.0"
	lsregisterPath = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
)

const infoPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>{{.BundleID}}</string>
    <key>CFBundleName</key>
    <string>{{.Name}}</string>
    <key>CFBundleDisplayName</key>
    <string>{{.Name}}</string>
    <key>CFBundleExecutable</key>
    <string>{{.Executable}}</string>
    <key>CFBundleVersion</key>
    <string>{{.Version}}</string>
    <key>CFBundleShortVersionString</key>
    <string>{{.Version}}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>LSUIElement</key>
    <true/>
    <key>CFBundleURLTypes</key>
    <array>
        <dict>
            <key>CFBundleURLName</key>
            <string>HTTP/HTTPS Handler</string>
            <key>CFBundleURLSchemes</key>
            <array>
                <string>http</string>
                <string>https</string>
            </array>
        </dict>
    </array>
</dict>
</plist>
`

func bundlePath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Applications", bundleName+".app")
}

// Install creates ~/Applications/Browser Proxy.app, signs it ad-hoc and
// registers it with Launch Services. It does NOT change the system default
// browser — the user picks "Browser Proxy" in System Settings.
func Install() error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate binary: %w", err)
	}
	src, err = filepath.Abs(src)
	if err != nil {
		return err
	}

	bundle := bundlePath()
	macos := filepath.Join(bundle, "Contents", "MacOS")
	res := filepath.Join(bundle, "Contents", "Resources")
	for _, d := range []string{macos, res} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	dst := filepath.Join(macos, bundleExec)
	if err := copyFile(src, dst, 0o755); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}

	tmpl := template.Must(template.New("p").Parse(infoPlistTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"BundleID":   bundleID,
		"Name":       bundleName,
		"Executable": bundleExec,
		"Version":    bundleVersion,
	}); err != nil {
		return err
	}
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	if err := os.WriteFile(plist, buf.Bytes(), 0o644); err != nil {
		return err
	}

	if err := exec.Command("codesign", "--sign", "-", "--force", "--deep", bundle).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: codesign failed: %v (the bundle may still work for local use)\n", err)
	}

	if _, err := os.Stat(lsregisterPath); err == nil {
		_ = exec.Command(lsregisterPath, "-f", bundle).Run()
	}

	fmt.Printf("Installed: %s\n\n", bundle)
	fmt.Println("Now set Browser Proxy as your default browser:")
	fmt.Println("  System Settings → Desktop & Dock → Default web browser → Browser Proxy")
	return nil
}

// Uninstall deletes the bundle from ~/Applications.
func Uninstall() error {
	bundle := bundlePath()
	if err := os.RemoveAll(bundle); err != nil {
		return err
	}
	fmt.Printf("Removed: %s\n", bundle)
	fmt.Println("Pick a new default browser in System Settings.")
	return nil
}

// IsBundleStart reports whether this binary is being run from inside an .app
// bundle (i.e. by Launch Services). When true, main.go enters daemon mode.
func IsBundleStart() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(exe, ".app/Contents/MacOS/")
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(perm)
}
