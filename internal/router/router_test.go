package router

import (
	"regexp"
	"testing"
)

func mustRouter(t *testing.T, def string, rules ...Rule) *Router {
	t.Helper()
	return &Router{Default: def, Rules: rules}
}

func TestPrefixMatch(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{Match: PrefixMatcher{Prefix: "https://github.com/"}, Browser: "Firefox"},
	)
	cases := map[string]string{
		"https://github.com/foo/bar":  "Firefox",
		"https://example.com/":        "Default",
		"http://github.com/":          "Default", // prefix is https-only
		"https://github.community/":   "Default", // not a prefix
	}
	for url, want := range cases {
		if got, _, _ := r.Resolve(url); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestHostMatch(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{Match: HostMatcher{Pattern: "*.atlassian.net"}, Browser: "Firefox"},
		Rule{Match: HostMatcher{Pattern: "example.com"}, Browser: "Brave"},
	)
	cases := map[string]string{
		"https://acme.atlassian.net/board": "Firefox",
		"https://atlassian.net/":           "Firefox", // apex matches *. wildcard
		"https://example.com/foo":          "Brave",
		"https://EXAMPLE.com/foo":          "Brave", // case-insensitive host
		"https://other.com/":               "Default",
	}
	for url, want := range cases {
		if got, _, _ := r.Resolve(url); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestRegexMatch(t *testing.T) {
	re := regexp.MustCompile(`^https://meet\.google\.com`)
	r := mustRouter(t, "Default",
		Rule{Match: RegexMatcher{Re: re}, Browser: "Chrome"},
	)
	cases := map[string]string{
		"https://meet.google.com/abc-defg": "Chrome",
		"https://google.com/meet":          "Default",
	}
	for url, want := range cases {
		if got, _, _ := r.Resolve(url); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestSuffixMatch(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{Match: SuffixMatcher{Suffix: ".pdf"}, Browser: "Preview"},
	)
	cases := map[string]string{
		"https://example.com/doc.pdf":      "Preview",
		"https://example.com/DOC.PDF":      "Preview", // case-insensitive
		"https://example.com/doc.pdf?x=1":  "Preview", // query stripped, suffix tested against path
		"https://example.com/doc.html":     "Default",
		"https://example.com/dir.pdf/foo":  "Default", // path doesn't end in .pdf
	}
	for url, want := range cases {
		if got, _, _ := r.Resolve(url); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestFirstRuleWins(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{Match: HostMatcher{Pattern: "github.com"}, Browser: "First"},
		Rule{Match: PrefixMatcher{Prefix: "https://github.com/"}, Browser: "Second"},
	)
	got, idx, _ := r.Resolve("https://github.com/foo")
	if got != "First" || idx != 0 {
		t.Errorf("expected First/0, got %s/%d", got, idx)
	}
}

func TestNoRuleMatches(t *testing.T) {
	r := mustRouter(t, "FallbackBrowser")
	got, idx, _ := r.Resolve("https://anything.example/")
	if got != "FallbackBrowser" || idx != -1 {
		t.Errorf("expected FallbackBrowser/-1, got %s/%d", got, idx)
	}
}

func TestUnparseableURLFallsBackToDefault(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{Match: HostMatcher{Pattern: "example.com"}, Browser: "Other"},
	)
	got, _, _ := r.Resolve(":::not a url:::")
	if got != "Default" {
		t.Errorf("expected default for malformed URL, got %s", got)
	}
}
