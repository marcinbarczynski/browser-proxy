package router

import (
	"net/url"
	"strings"

	"github.com/maxischmaxi/browser-proxy/internal/source"
)

// Rule combines optional URL- and Source-constraints with a target browser
// (and optionally a profile within that browser). At least one of URL/Source
// must be set; both must match when both are set (AND).
type Rule struct {
	URL     Matcher
	Source  string
	Browser string
	Profile string
}

// Describe renders the rule's CONSTRAINTS for diagnostic output. The action
// (browser/profile) is rendered separately by the caller.
func (r Rule) Describe() string {
	var parts []string
	if r.URL != nil {
		parts = append(parts, r.URL.String())
	}
	if r.Source != "" {
		parts = append(parts, "source="+r.Source)
	}
	return strings.Join(parts, " AND ")
}

// Decision is the outcome of routing a single URL through the rules.
type Decision struct {
	Browser   string
	Profile   string
	RuleIndex int // -1 when the default applies
}

// MatchedRule reports whether a non-default rule produced this Decision.
func (d Decision) MatchedRule() bool { return d.RuleIndex >= 0 }

type Router struct {
	Default string
	Rules   []Rule
}

// Resolve picks the browser+profile for rawURL given the source app.
// Unparseable URLs fall back to the default.
func (r *Router) Resolve(rawURL string, src source.Info) Decision {
	u, _ := url.Parse(rawURL)
	for i, rule := range r.Rules {
		if rule.URL != nil && !rule.URL.Matches(u, rawURL) {
			continue
		}
		if rule.Source != "" && !sourceMatches(src, rule.Source) {
			continue
		}
		return Decision{Browser: rule.Browser, Profile: rule.Profile, RuleIndex: i}
	}
	return Decision{Browser: r.Default, RuleIndex: -1}
}

// sourceMatches compares actual against want, treating dot-bearing strings as
// macOS bundle-IDs and everything else as a human-readable name.
func sourceMatches(actual source.Info, want string) bool {
	w := strings.ToLower(strings.TrimSpace(want))
	if w == "" {
		return true
	}
	if isBundleIDLike(w) {
		return strings.ToLower(actual.BundleID) == w
	}
	return strings.ToLower(actual.Name) == w
}

func isBundleIDLike(s string) bool {
	return strings.Contains(s, ".") && !strings.ContainsAny(s, " \t/")
}
