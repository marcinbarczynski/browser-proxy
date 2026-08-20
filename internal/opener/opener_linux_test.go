//go:build linux

package opener

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBrowserCommandUsesTransientScope(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "systemd-run"), "#!/bin/sh\nexit 0\n")
	browser := filepath.Join(bin, "firefox")
	writeExecutable(t, browser, "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)

	cmd := browserCommand("firefox", "-P", "Work", "https://example.com")
	args := cmd.Args
	if filepath.Base(args[0]) != "systemd-run" {
		t.Fatalf("argv[0] = %q, want systemd-run", args[0])
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"--user", "--scope", "--quiet"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %v", want, args)
		}
	}
	for _, unwanted := range []string{"--service", "--collect", "--unit"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("unexpected %q in %v", unwanted, args)
		}
	}
	if tail := args[len(args)-4:]; !equalArgs(tail, []string{browser, "-P", "Work", "https://example.com"}) {
		t.Errorf("browser command mangled: %v", tail)
	}
	if args[len(args)-5] != "--" {
		t.Errorf("browser args must follow a %q separator: %v", "--", args)
	}
}

func TestBrowserCommandFallsBackWithoutSystemdRun(t *testing.T) {
	bin := t.TempDir()
	browser := filepath.Join(bin, "firefox")
	writeExecutable(t, browser, "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin) // no systemd-run anywhere
	cmd := browserCommand("firefox", "-P", "Work", "https://example.com")
	if want := []string{browser, "-P", "Work", "https://example.com"}; !equalArgs(cmd.Args, want) {
		t.Errorf("got %v, want %v", cmd.Args, want)
	}
}

func TestBrowserCommandFallsBackWhenScopeProbeFails(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "systemd-run"), "#!/bin/sh\nexit 1\n")
	browser := filepath.Join(bin, "firefox")
	writeExecutable(t, browser, "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)

	cmd := browserCommand("firefox", "https://example.com")
	if want := []string{browser, "https://example.com"}; !equalArgs(cmd.Args, want) {
		t.Errorf("got %v, want %v", cmd.Args, want)
	}
}

func TestOpenReportsMissingBrowser(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Open("missing-browser", "", "https://example.com"); err == nil {
		t.Fatal("expected a launch error")
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}
