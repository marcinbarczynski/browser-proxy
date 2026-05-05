package rewriter

import (
	"regexp"
	"testing"
)

func TestNilRewriterIsNoOp(t *testing.T) {
	var r *Rewriter
	if got := r.Apply("https://example.com/"); got != "https://example.com/" {
		t.Errorf("nil rewriter should be a no-op, got %q", got)
	}
}

func TestForceHTTPS(t *testing.T) {
	r := &Rewriter{ForceHTTPS: true}
	cases := map[string]string{
		"http://example.com/":     "https://example.com/",
		"http://example.com/path": "https://example.com/path",
		"https://example.com/":    "https://example.com/", // already https
		"ws://example.com/":       "ws://example.com/",    // not http
		"ftp://example.com/":      "ftp://example.com/",
		"//example.com/":          "//example.com/", // protocol-relative
	}
	for in, want := range cases {
		if got := r.Apply(in); got != want {
			t.Errorf("Apply(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripParams(t *testing.T) {
	r := &Rewriter{
		StripParams: []ParamMatcher{
			NewParamMatcher("utm_*"),
			NewParamMatcher("fbclid"),
			NewParamMatcher("gclid"),
		},
	}
	cases := []struct {
		in, want string
	}{
		{"https://x.com/?utm_source=foo", "https://x.com/"},
		{"https://x.com/?utm_medium=email&utm_term=x", "https://x.com/"},
		{"https://x.com/?fbclid=xyz", "https://x.com/"},
		{"https://x.com/?gclid=abc", "https://x.com/"},
		{"https://x.com/?fbclid=xyz&id=42", "https://x.com/?id=42"},
		{"https://x.com/?id=42", "https://x.com/?id=42"},               // unchanged
		{"https://x.com/?utmsmth=1", "https://x.com/?utmsmth=1"},       // not utm_*
		{"https://x.com/path", "https://x.com/path"},                   // no query at all
		{"https://x.com/?id=1&utm_source=z&id=2", "https://x.com/?id=1&id=2"}, // multi-value preserved
	}
	for _, c := range cases {
		if got := r.Apply(c.in); got != c.want {
			t.Errorf("Apply(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParamMatcher(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"fbclid", "fbclid", true},
		{"fbclid", "fbclid2", false},
		{"utm_*", "utm_source", true},
		{"utm_*", "utm_", true},
		{"utm_*", "utmsource", false}, // missing underscore
		{"utm_*", "ref", false},
		{"*", "anything", true}, // bare * matches everything
	}
	for _, c := range cases {
		m := NewParamMatcher(c.pattern)
		if got := m.Matches(c.name); got != c.want {
			t.Errorf("NewParamMatcher(%q).Matches(%q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestHostReplaceRule(t *testing.T) {
	r := &Rewriter{
		Rules: []Rule{
			HostReplaceRule{Host: "twitter.com", Replacement: "nitter.net"},
			HostReplaceRule{Host: "*.youtube.com", Replacement: "invidious.example"},
		},
	}
	cases := []struct {
		in, want string
	}{
		{"https://twitter.com/user", "https://nitter.net/user"},
		{"https://twitter.com:8080/user", "https://nitter.net:8080/user"},     // port preserved
		{"https://TWITTER.COM/user", "https://nitter.net/user"},               // case-insensitive
		{"https://www.youtube.com/watch?v=x", "https://invidious.example/watch?v=x"},
		{"https://youtube.com/x", "https://invidious.example/x"},              // wildcard apex
		{"https://example.com/twitter.com", "https://example.com/twitter.com"}, // not the host
		{"https://other.example/", "https://other.example/"},                  // no match
	}
	for _, c := range cases {
		if got := r.Apply(c.in); got != c.want {
			t.Errorf("Apply(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRegexRule(t *testing.T) {
	r := &Rewriter{
		Rules: []Rule{
			RegexRule{
				Re:          regexp.MustCompile(`^https://www\.google\.com/search\?q=(.+)$`),
				Replacement: "https://duckduckgo.com/?q=$1",
			},
		},
	}
	cases := []struct {
		in, want string
	}{
		{"https://www.google.com/search?q=hello", "https://duckduckgo.com/?q=hello"},
		{"https://www.google.com/search?q=multi+word", "https://duckduckgo.com/?q=multi+word"},
		{"https://www.google.com/maps", "https://www.google.com/maps"}, // no match
	}
	for _, c := range cases {
		if got := r.Apply(c.in); got != c.want {
			t.Errorf("Apply(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestComposedRewrites(t *testing.T) {
	r := &Rewriter{
		ForceHTTPS:  true,
		StripParams: []ParamMatcher{NewParamMatcher("utm_*")},
		Rules: []Rule{
			HostReplaceRule{Host: "twitter.com", Replacement: "nitter.net"},
		},
	}
	in := "http://twitter.com/foo?utm_source=email&id=42"
	want := "https://nitter.net/foo?id=42"
	if got := r.Apply(in); got != want {
		t.Errorf("Apply(%q) = %q, want %q", in, got, want)
	}
}

func TestRulesAreOrdered(t *testing.T) {
	r := &Rewriter{
		Rules: []Rule{
			HostReplaceRule{Host: "a.example", Replacement: "b.example"},
			HostReplaceRule{Host: "b.example", Replacement: "c.example"},
		},
	}
	// First rule rewrites to b.example, second rewrites to c.example.
	if got := r.Apply("https://a.example/"); got != "https://c.example/" {
		t.Errorf("expected chained rewrite to c.example, got %q", got)
	}
}

func TestUnparseableURLPassthrough(t *testing.T) {
	r := &Rewriter{
		StripParams: []ParamMatcher{NewParamMatcher("utm_*")},
		Rules: []Rule{
			HostReplaceRule{Host: "x.com", Replacement: "y.com"},
		},
	}
	in := ":::not a url:::"
	if got := r.Apply(in); got != in {
		t.Errorf("expected passthrough for malformed URL, got %q", got)
	}
}

func TestEmptyRewriterReturnsInputUnchanged(t *testing.T) {
	r := &Rewriter{}
	in := "https://example.com/path?id=42&q=hello"
	if got := r.Apply(in); got != in {
		t.Errorf("expected unchanged URL, got %q", got)
	}
}
