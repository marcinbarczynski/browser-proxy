package platform

import (
	"runtime"
	"strings"
	"testing"
)

func TestNativeMessagingDirRecognizesCommonBrowsers(t *testing.T) {
	commonOnAll := []string{"chrome", "chromium", "brave", "edge", "vivaldi"}
	for _, b := range commonOnAll {
		dir, err := nativeMessagingDir(b)
		if err != nil {
			t.Errorf("nativeMessagingDir(%q): %v", b, err)
			continue
		}
		if !strings.HasSuffix(dir, "NativeMessagingHosts") {
			t.Errorf("dir for %q does not end in NativeMessagingHosts: %s", b, dir)
		}
	}
}

func TestNativeMessagingDirIsCaseInsensitive(t *testing.T) {
	a, err := nativeMessagingDir("Chrome")
	if err != nil {
		t.Fatalf("Chrome: %v", err)
	}
	b, err := nativeMessagingDir("CHROME")
	if err != nil {
		t.Fatalf("CHROME: %v", err)
	}
	if a != b {
		t.Errorf("case mismatch: %q vs %q", a, b)
	}
}

func TestNativeMessagingDirRejectsUnknown(t *testing.T) {
	if _, err := nativeMessagingDir("netscape-navigator"); err == nil {
		t.Error("expected error for unknown browser")
	}
}

func TestNativeMessagingDirIsOSSpecific(t *testing.T) {
	dir, err := nativeMessagingDir("chrome")
	if err != nil {
		t.Fatal(err)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, "Library/Application Support/Google/Chrome") {
			t.Errorf("darwin chrome dir wrong: %s", dir)
		}
	case "linux":
		if !strings.Contains(dir, "google-chrome") {
			t.Errorf("linux chrome dir wrong: %s", dir)
		}
	}
}
