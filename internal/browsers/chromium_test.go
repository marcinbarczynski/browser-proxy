package browsers

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sampleLocalState = `{
  "profile": {
    "info_cache": {
      "Default":   {"name": "Personal"},
      "Profile 1": {"name": "Work"},
      "Profile 2": {"name": "Side Project"}
    }
  }
}`

func writeLocalState(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "Local State")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLookupChromiumProfile(t *testing.T) {
	path := writeLocalState(t, sampleLocalState)

	cases := []struct {
		spec, want string
	}{
		// Direct directory match
		{"Default", "Default"},
		{"Profile 1", "Profile 1"},
		{"Profile 2", "Profile 2"},
		// Display-name match
		{"Personal", "Default"},
		{"Work", "Profile 1"},
		{"Side Project", "Profile 2"},
		// Case-insensitive display name
		{"work", "Profile 1"},
		{"PERSONAL", "Default"},
		// Unknown
		{"Nope", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := lookupChromiumProfile(path, tc.spec); got != tc.want {
			t.Errorf("lookupChromiumProfile(%q) = %q, want %q", tc.spec, got, tc.want)
		}
	}
}

func TestLookupChromiumProfile_MissingFile(t *testing.T) {
	if got := lookupChromiumProfile("/no/such/file", "Default"); got != "" {
		t.Errorf("expected empty result for missing file, got %q", got)
	}
}

func TestLookupChromiumProfile_InvalidJSON(t *testing.T) {
	path := writeLocalState(t, "{not json")
	if got := lookupChromiumProfile(path, "Default"); got != "" {
		t.Errorf("expected empty result for invalid JSON, got %q", got)
	}
}

func TestLookupChromiumProfile_MissingInfoCache(t *testing.T) {
	path := writeLocalState(t, `{"profile": {}}`)
	if got := lookupChromiumProfile(path, "Default"); got != "" {
		t.Errorf("expected empty result for missing info_cache, got %q", got)
	}
}

func TestResolveChromiumProfile_PassthroughOnNoMatch(t *testing.T) {
	// Unknown browser → no Local State path → passthrough.
	if got := ResolveChromiumProfile("UnknownBrowser", "Profile 1"); got != "Profile 1" {
		t.Errorf("expected passthrough %q, got %q", "Profile 1", got)
	}
}

// ── ListChromiumProfiles ─────────────────────────────────────────────────

func TestParseChromiumLocalState(t *testing.T) {
	js := `{
		"profile": {
			"info_cache": {
				"Default":   {"name": "Personal", "user_name": "me@personal.com"},
				"Profile 1": {"name": "Work",     "user_name": "me@work.com"},
				"Profile 2": {"name": "No Email"}
			}
		}
	}`
	profs, err := parseChromiumLocalState([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if len(profs) != 3 {
		t.Fatalf("got %d, want 3", len(profs))
	}
	// Sorted by Directory: Default, Profile 1, Profile 2
	if profs[0].Directory != "Default" || profs[0].Name != "Personal" || profs[0].Email != "me@personal.com" {
		t.Errorf("[0]: %+v", profs[0])
	}
	if profs[1].Directory != "Profile 1" || profs[1].Email != "me@work.com" {
		t.Errorf("[1]: %+v", profs[1])
	}
	if profs[2].Directory != "Profile 2" || profs[2].Email != "" {
		t.Errorf("[2]: %+v", profs[2])
	}
}

func TestParseChromiumLocalState_InvalidJSON(t *testing.T) {
	if _, err := parseChromiumLocalState([]byte("{not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseChromiumLocalState_MissingInfoCache(t *testing.T) {
	profs, err := parseChromiumLocalState([]byte(`{"profile": {}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(profs) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profs))
	}
}

func TestListChromiumProfiles_UnknownBrowser(t *testing.T) {
	_, _, err := ListChromiumProfiles("safari")
	if err == nil {
		t.Error("expected error for non-chromium browser")
	}
}

func TestListChromiumProfiles_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific path layout")
	}
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir := filepath.Join(tmpHome, ".config", "google-chrome-beta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	js := `{"profile":{"info_cache":{
		"Default":   {"name":"jeschek.dev",   "user_name":"max@jeschek.dev"},
		"Profile 1": {"name":"logic-joe.com", "user_name":"m.jeschek@logic-joe.com"}
	}}}`
	if err := os.WriteFile(filepath.Join(dir, "Local State"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}

	source, profs, err := ListChromiumProfiles("google-chrome-beta")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "google-chrome-beta") {
		t.Errorf("source path missing channel: %q", source)
	}
	if len(profs) != 2 || profs[0].Email != "max@jeschek.dev" {
		t.Errorf("profiles: %+v", profs)
	}
}

// ── chromiumLocalStatePathsImpl: per-channel path resolution ──────────────

func TestChromiumLocalStatePaths_Linux(t *testing.T) {
	cases := []struct {
		name, browser, wantSuffix string
	}{
		{"stable binary", "google-chrome", ".config/google-chrome/Local State"},
		{"stable alias", "google-chrome-stable", ".config/google-chrome/Local State"},
		{"beta binary", "google-chrome-beta", ".config/google-chrome-beta/Local State"},
		{"beta absolute path", "/opt/google/chrome-beta/google-chrome-beta", ".config/google-chrome-beta/Local State"},
		{"unstable", "google-chrome-unstable", ".config/google-chrome-unstable/Local State"},
		{"chromium", "chromium", ".config/chromium/Local State"},
		{"chromium-browser", "chromium-browser", ".config/chromium/Local State"},
		{"brave", "brave-browser", ".config/BraveSoftware/Brave-Browser/Local State"},
		{"edge", "microsoft-edge", ".config/microsoft-edge/Local State"},
		{"vivaldi", "vivaldi", ".config/vivaldi/Local State"},
		{"thorium", "thorium-browser", ".config/thorium/Local State"},
		{"unknown returns nil", "lynx", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := chromiumLocalStatePathsImpl(c.browser, "/home/u", "linux")
			if c.wantSuffix == "" {
				if len(got) != 0 {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.HasSuffix(got[0], c.wantSuffix) {
				t.Errorf("got %v, want suffix %q", got, c.wantSuffix)
			}
		})
	}
}

func TestChromiumLocalStatePaths_Darwin(t *testing.T) {
	cases := []struct {
		name, browser, wantSuffix string
	}{
		{"chrome stable", "Google Chrome", "Library/Application Support/Google/Chrome/Local State"},
		{"chrome beta name", "Google Chrome Beta", "Library/Application Support/Google/Chrome Beta/Local State"},
		{"chrome beta bundle id", "com.google.Chrome.beta", "Library/Application Support/Google/Chrome Beta/Local State"},
		{"chrome dev name", "Google Chrome Dev", "Library/Application Support/Google/Chrome Dev/Local State"},
		{"chrome canary", "Google Chrome Canary", "Library/Application Support/Google/Chrome Canary/Local State"},
		{"chromium", "Chromium", "Library/Application Support/Chromium/Local State"},
		{"brave", "Brave Browser", "Library/Application Support/BraveSoftware/Brave-Browser/Local State"},
		{"edge", "Microsoft Edge", "Library/Application Support/Microsoft Edge/Local State"},
		{"opera", "Opera", "Library/Application Support/com.operasoftware.Opera/Local State"},
		{"arc", "Arc", "Library/Application Support/Arc/User Data/Local State"},
		{"unknown returns nil", "Safari", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := chromiumLocalStatePathsImpl(c.browser, "/Users/u", "darwin")
			if c.wantSuffix == "" {
				if len(got) != 0 {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.HasSuffix(got[0], c.wantSuffix) {
				t.Errorf("got %v, want suffix %q", got, c.wantSuffix)
			}
		})
	}
}

// Critical regression: ensure beta isn't accidentally matched as stable
// (the bug that hid logicjoe profiles in google-chrome instead of -beta).
func TestChromiumPaths_BetaDoesNotFallToStable(t *testing.T) {
	got := chromiumLocalStatePathsImpl("google-chrome-beta", "/home/u", "linux")
	if len(got) != 1 || strings.Contains(got[0], "google-chrome/Local State") {
		t.Errorf("beta must not resolve to stable path, got %v", got)
	}
	if !strings.HasSuffix(got[0], "google-chrome-beta/Local State") {
		t.Errorf("expected google-chrome-beta path, got %v", got)
	}
}

// End-to-end: integration test for the user's actual case — display-name
// "logic-joe.com" in google-chrome-beta should resolve to "Profile 1".
func TestResolveChromiumProfile_BetaChannelDisplayNameLookup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific path layout")
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	betaDir := filepath.Join(tmpHome, ".config", "google-chrome-beta")
	if err := os.MkdirAll(betaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	js := `{"profile":{"info_cache":{
		"Default":   {"name":"jeschek.dev"},
		"Profile 1": {"name":"logic-joe.com"}
	}}}`
	if err := os.WriteFile(filepath.Join(betaDir, "Local State"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ResolveChromiumProfile("google-chrome-beta", "logic-joe.com")
	if got != "Profile 1" {
		t.Errorf("expected Profile 1 (display-name lookup), got %q", got)
	}
	got = ResolveChromiumProfile("google-chrome-beta", "Default")
	if got != "Default" {
		t.Errorf("expected Default (direct match), got %q", got)
	}
}
