package browsers

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sampleProfilesIni = `[Install4F96D1932A9F858E]
Default=8xc8ux2y.default-release
Locked=1

[Profile1]
Name=work
IsRelative=1
Path=abc123.work

[Profile0]
Name=default-release
IsRelative=1
Path=8xc8ux2y.default-release
Default=1

[General]
StartWithLastProfile=1
Version=2
`

func TestParseFirefoxProfiles(t *testing.T) {
	profs := parseFirefoxProfiles(sampleProfilesIni)
	if len(profs) != 2 {
		t.Fatalf("expected 2 profiles, got %d (%+v)", len(profs), profs)
	}

	// Sorted by Name: "default-release" < "work"
	if profs[0].Name != "default-release" {
		t.Errorf("expected default-release first, got %q", profs[0].Name)
	}
	if !profs[0].IsDefault {
		t.Errorf("default-release must be marked default (Profile0 has Default=1 + Install Default points at it)")
	}
	if profs[0].Path != "8xc8ux2y.default-release" {
		t.Errorf("path: got %q", profs[0].Path)
	}

	if profs[1].Name != "work" {
		t.Errorf("expected work second, got %q", profs[1].Name)
	}
	if profs[1].IsDefault {
		t.Errorf("work must NOT be default")
	}
}

func TestParseFirefoxProfiles_OnlyInstallDefault(t *testing.T) {
	// Per-profile Default=1 missing; Install section is the source of truth.
	ini := `[InstallABC]
Default=xyz.work

[Profile0]
Name=personal
Path=aaa.personal

[Profile1]
Name=work
Path=xyz.work
`
	profs := parseFirefoxProfiles(ini)
	if len(profs) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profs))
	}
	for _, p := range profs {
		if p.Name == "work" && !p.IsDefault {
			t.Errorf("work should be default via [Install]")
		}
		if p.Name == "personal" && p.IsDefault {
			t.Errorf("personal should NOT be default")
		}
	}
}

func TestParseFirefoxProfiles_EmptyInput(t *testing.T) {
	if got := parseFirefoxProfiles(""); len(got) != 0 {
		t.Errorf("expected empty profiles for empty input, got %v", got)
	}
}

func TestParseFirefoxProfiles_CommentsAndBlankLines(t *testing.T) {
	ini := `# this is a comment
; semicolon comment too

[Profile0]
   Name = solo
   Path = one.solo
`
	profs := parseFirefoxProfiles(ini)
	if len(profs) != 1 || profs[0].Name != "solo" {
		t.Errorf("got %+v", profs)
	}
}

func TestParseINI_MalformedLines(t *testing.T) {
	// Lines without '=' after a section, lines outside any section — must
	// be ignored, not crash.
	ini := `random outside section
[X]
no equals here
key=val
also_no_eq
[Y]
k=v
`
	secs := parseINI(ini)
	if len(secs) != 2 {
		t.Fatalf("got %d sections, want 2", len(secs))
	}
	if secs[0].Fields["key"] != "val" {
		t.Errorf("section X: got %v", secs[0].Fields)
	}
	if secs[1].Fields["k"] != "v" {
		t.Errorf("section Y: got %v", secs[1].Fields)
	}
}

// ── Path resolution ──────────────────────────────────────────────────────

func TestFirefoxProfilesIniPaths_Linux(t *testing.T) {
	cases := []struct {
		browser, wantSuffix string
	}{
		{"firefox", ".mozilla/firefox/profiles.ini"},
		{"firefox-esr", ".mozilla/firefox/profiles.ini"},
		{"librewolf", ".librewolf/profiles.ini"},
		{"waterfox", ".waterfox/profiles.ini"},
		{"tor browser", ".mozilla/firefox/profiles.ini"},
	}
	for _, c := range cases {
		t.Run(c.browser, func(t *testing.T) {
			got := firefoxProfilesIniPathsImpl(c.browser, "/home/u", "linux")
			if len(got) != 1 || !strings.HasSuffix(got[0], c.wantSuffix) {
				t.Errorf("got %v, want suffix %q", got, c.wantSuffix)
			}
		})
	}
}

func TestFirefoxProfilesIniPaths_Darwin(t *testing.T) {
	cases := []struct {
		browser, wantSuffix string
	}{
		{"Firefox", "Library/Application Support/Firefox/profiles.ini"},
		{"LibreWolf", "Library/Application Support/LibreWolf/profiles.ini"},
		{"Waterfox", "Library/Application Support/Waterfox/profiles.ini"},
	}
	for _, c := range cases {
		t.Run(c.browser, func(t *testing.T) {
			got := firefoxProfilesIniPathsImpl(c.browser, "/Users/u", "darwin")
			if len(got) != 1 || !strings.HasSuffix(got[0], c.wantSuffix) {
				t.Errorf("got %v, want suffix %q", got, c.wantSuffix)
			}
		})
	}
}

func TestFirefoxProfilesIniPaths_NotAFirefoxBrowser(t *testing.T) {
	if got := firefoxProfilesIniPathsImpl("chrome", "/home/u", "linux"); len(got) != 0 {
		t.Errorf("chrome must not produce firefox paths, got %v", got)
	}
	if got := firefoxProfilesIniPathsImpl("safari", "/Users/u", "darwin"); len(got) != 0 {
		t.Errorf("safari must not produce firefox paths, got %v", got)
	}
}

// ── ListFirefoxProfiles end-to-end (Linux only — uses real os.UserHomeDir) ─

func TestListFirefoxProfiles_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific path layout")
	}
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir := filepath.Join(tmpHome, ".mozilla", "firefox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.ini"), []byte(sampleProfilesIni), 0o644); err != nil {
		t.Fatal(err)
	}

	source, profs, err := ListFirefoxProfiles("firefox")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(source, "profiles.ini") {
		t.Errorf("source path: got %q", source)
	}
	if len(profs) != 2 {
		t.Errorf("got %d profiles, want 2", len(profs))
	}
}

func TestListFirefoxProfiles_UnknownBrowser(t *testing.T) {
	_, _, err := ListFirefoxProfiles("chrome")
	if err == nil {
		t.Error("expected error for non-firefox browser")
	}
}
