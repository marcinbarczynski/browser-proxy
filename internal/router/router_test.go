package router

import (
	"regexp"
	"testing"

	"github.com/max/browser-proxy/internal/source"
)

func mustRouter(t *testing.T, def string, rules ...Rule) *Router {
	t.Helper()
	return &Router{Default: def, Rules: rules}
}

var noSrc = source.Info{}

func TestPrefixMatch(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{URL: PrefixMatcher{Prefix: "https://github.com/"}, Browser: "Firefox"},
	)
	cases := map[string]string{
		"https://github.com/foo/bar": "Firefox",
		"https://example.com/":       "Default",
		"http://github.com/":         "Default",
		"https://github.community/":  "Default",
	}
	for url, want := range cases {
		if got := r.Resolve(url, noSrc); got.Browser != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got.Browser, want)
		}
	}
}

func TestHostMatch(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{URL: HostMatcher{Pattern: "*.atlassian.net"}, Browser: "Firefox"},
		Rule{URL: HostMatcher{Pattern: "example.com"}, Browser: "Brave"},
	)
	cases := map[string]string{
		"https://acme.atlassian.net/board": "Firefox",
		"https://atlassian.net/":           "Firefox",
		"https://example.com/foo":          "Brave",
		"https://EXAMPLE.com/foo":          "Brave",
		"https://other.com/":               "Default",
	}
	for url, want := range cases {
		if got := r.Resolve(url, noSrc); got.Browser != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got.Browser, want)
		}
	}
}

func TestRegexMatch(t *testing.T) {
	re := regexp.MustCompile(`^https://meet\.google\.com`)
	r := mustRouter(t, "Default",
		Rule{URL: RegexMatcher{Re: re}, Browser: "Chrome"},
	)
	cases := map[string]string{
		"https://meet.google.com/abc-defg": "Chrome",
		"https://google.com/meet":          "Default",
	}
	for url, want := range cases {
		if got := r.Resolve(url, noSrc); got.Browser != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got.Browser, want)
		}
	}
}

func TestSuffixMatch(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{URL: SuffixMatcher{Suffix: ".pdf"}, Browser: "Preview"},
	)
	cases := map[string]string{
		"https://example.com/doc.pdf":     "Preview",
		"https://example.com/DOC.PDF":     "Preview",
		"https://example.com/doc.pdf?x=1": "Preview",
		"https://example.com/doc.html":    "Default",
		"https://example.com/dir.pdf/foo": "Default",
	}
	for url, want := range cases {
		if got := r.Resolve(url, noSrc); got.Browser != want {
			t.Errorf("Resolve(%q) = %q, want %q", url, got.Browser, want)
		}
	}
}

func TestFirstRuleWins(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{URL: HostMatcher{Pattern: "github.com"}, Browser: "First"},
		Rule{URL: PrefixMatcher{Prefix: "https://github.com/"}, Browser: "Second"},
	)
	d := r.Resolve("https://github.com/foo", noSrc)
	if d.Browser != "First" || d.RuleIndex != 0 {
		t.Errorf("expected First/0, got %s/%d", d.Browser, d.RuleIndex)
	}
}

func TestNoRuleMatches(t *testing.T) {
	r := mustRouter(t, "FallbackBrowser")
	d := r.Resolve("https://anything.example/", noSrc)
	if d.Browser != "FallbackBrowser" || d.RuleIndex != -1 || d.MatchedRule() {
		t.Errorf("expected default decision, got %+v", d)
	}
}

func TestUnparseableURLFallsBackToDefault(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{URL: HostMatcher{Pattern: "example.com"}, Browser: "Other"},
	)
	if got := r.Resolve(":::not a url:::", noSrc); got.Browser != "Default" {
		t.Errorf("expected default for malformed URL, got %s", got.Browser)
	}
}

func TestSourceOnlyRule(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{Source: "Slack", Browser: "ChromeFromSlack"},
	)
	cases := []struct {
		url  string
		src  source.Info
		want string
	}{
		{"https://anywhere.example/", source.Info{Name: "Slack"}, "ChromeFromSlack"},
		{"https://anywhere.example/", source.Info{Name: "slack"}, "ChromeFromSlack"},
		{"https://anywhere.example/", source.Info{Name: "Mail"}, "Default"},
		{"https://anywhere.example/", source.Info{}, "Default"},
	}
	for _, tc := range cases {
		if got := r.Resolve(tc.url, tc.src); got.Browser != tc.want {
			t.Errorf("Resolve(%q, %+v) = %q, want %q", tc.url, tc.src, got.Browser, tc.want)
		}
	}
}

func TestSourceByBundleID(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{Source: "com.tinyspeck.slackmacgap", Browser: "Chrome"},
	)
	withID := source.Info{Name: "Slack", BundleID: "com.tinyspeck.slackmacgap"}
	if got := r.Resolve("https://x.example/", withID); got.Browser != "Chrome" {
		t.Errorf("expected Chrome with bundle id match, got %s", got.Browser)
	}
	if got := r.Resolve("https://x.example/", source.Info{Name: "Slack"}); got.Browser != "Default" {
		t.Errorf("bundle-id rule must not match by name, got %s", got.Browser)
	}
}

func TestURLAndSourceCombinedAreANDed(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{URL: PrefixMatcher{Prefix: "https://docs."}, Source: "Slack", Browser: "Combined"},
	)
	cases := []struct {
		url  string
		src  source.Info
		want string
	}{
		{"https://docs.example.com/", source.Info{Name: "Slack"}, "Combined"},
		{"https://docs.example.com/", source.Info{Name: "Mail"}, "Default"},
		{"https://other.example/", source.Info{Name: "Slack"}, "Default"},
		{"https://docs.example.com/", source.Info{}, "Default"},
	}
	for _, tc := range cases {
		if got := r.Resolve(tc.url, tc.src); got.Browser != tc.want {
			t.Errorf("Resolve(%q, %+v) = %q, want %q", tc.url, tc.src, got.Browser, tc.want)
		}
	}
}

func TestRuleDescribe(t *testing.T) {
	cases := []struct {
		rule Rule
		want string
	}{
		{Rule{URL: PrefixMatcher{Prefix: "https://x"}}, "prefix=https://x"},
		{Rule{Source: "Slack"}, "source=Slack"},
		{Rule{URL: HostMatcher{Pattern: "*.foo"}, Source: "Mail"}, "host=*.foo AND source=Mail"},
	}
	for _, tc := range cases {
		if got := tc.rule.Describe(); got != tc.want {
			t.Errorf("Describe = %q, want %q", got, tc.want)
		}
	}
}

func TestProfilePassThrough(t *testing.T) {
	r := mustRouter(t, "Default",
		Rule{URL: HostMatcher{Pattern: "*.atlassian.net"}, Browser: "Google Chrome", Profile: "Work"},
		Rule{Source: "Mail", Browser: "Firefox", Profile: "Personal"},
	)
	d := r.Resolve("https://acme.atlassian.net/", noSrc)
	if d.Browser != "Google Chrome" || d.Profile != "Work" {
		t.Errorf("rule 0: got %+v, want browser=Google Chrome profile=Work", d)
	}
	d = r.Resolve("https://anywhere/", source.Info{Name: "Mail"})
	if d.Browser != "Firefox" || d.Profile != "Personal" {
		t.Errorf("rule 1: got %+v, want browser=Firefox profile=Personal", d)
	}
	d = r.Resolve("https://other/", noSrc)
	if d.Browser != "Default" || d.Profile != "" {
		t.Errorf("default: got %+v, want browser=Default profile empty", d)
	}
}
