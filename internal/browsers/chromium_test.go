package browsers

import (
	"os"
	"path/filepath"
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
