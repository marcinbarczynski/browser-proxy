// Package rewriter applies URL transformations before routing.
//
// Three layers, applied in this order to every URL:
//  1. ForceHTTPS  — flips "http://" to "https://".
//  2. StripParams — drops named (or *-prefixed) query parameters.
//  3. Rules       — ordered list of host-replacements / regex-replacements.
//
// All layers are best-effort: if a URL fails to parse or no rule matches,
// the original URL is returned unchanged.
package rewriter

import (
	"net/url"
	"regexp"
	"strings"
)

type Rewriter struct {
	ForceHTTPS  bool
	StripParams []ParamMatcher
	Rules       []Rule
}

// Apply runs all configured transformations and returns the final URL.
// A nil Rewriter is a no-op.
func (r *Rewriter) Apply(rawURL string) string {
	if r == nil {
		return rawURL
	}
	s := rawURL

	if r.ForceHTTPS && strings.HasPrefix(s, "http://") {
		s = "https://" + strings.TrimPrefix(s, "http://")
	}

	if len(r.StripParams) > 0 {
		if newS, ok := stripParams(s, r.StripParams); ok {
			s = newS
		}
	}

	for _, rule := range r.Rules {
		if newS, ok := rule.Apply(s); ok {
			s = newS
		}
	}
	return s
}

// stripParams parses the URL, removes any query param matched by the matchers,
// and returns the rewritten URL. Returns (rawURL, false) if nothing changed.
func stripParams(rawURL string, matchers []ParamMatcher) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.RawQuery == "" {
		return rawURL, false
	}
	q := u.Query()
	changed := false
	for k := range q {
		for _, m := range matchers {
			if m.Matches(k) {
				q.Del(k)
				changed = true
				break
			}
		}
	}
	if !changed {
		return rawURL, false
	}
	u.RawQuery = q.Encode()
	return u.String(), true
}

// ParamMatcher matches a query-parameter name. A trailing "*" makes it a
// prefix match (e.g. "utm_*" matches utm_source, utm_medium, …); otherwise
// the match is exact and case-sensitive (per RFC 3986).
type ParamMatcher struct {
	Pattern  string
	IsPrefix bool
}

func NewParamMatcher(pattern string) ParamMatcher {
	if strings.HasSuffix(pattern, "*") {
		return ParamMatcher{Pattern: strings.TrimSuffix(pattern, "*"), IsPrefix: true}
	}
	return ParamMatcher{Pattern: pattern}
}

func (m ParamMatcher) Matches(name string) bool {
	if m.IsPrefix {
		return strings.HasPrefix(name, m.Pattern)
	}
	return name == m.Pattern
}

// Rule is a single URL → URL transformation. Apply returns (newURL, true)
// when it modified the URL; (rawURL, false) otherwise.
type Rule interface {
	Apply(rawURL string) (string, bool)
	String() string
}

// HostReplaceRule swaps the URL's host. The Host pattern supports the same
// "*." apex+subdomain wildcard as router.HostMatcher. The port (if any) is
// preserved.
type HostReplaceRule struct {
	Host        string
	Replacement string
}

func (r HostReplaceRule) Apply(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL, false
	}
	if !matchHost(strings.ToLower(u.Hostname()), strings.ToLower(r.Host)) {
		return rawURL, false
	}
	newHost := r.Replacement
	if port := u.Port(); port != "" {
		newHost = newHost + ":" + port
	}
	u.Host = newHost
	return u.String(), true
}

func (r HostReplaceRule) String() string {
	return "host=" + r.Host + " → " + r.Replacement
}

// RegexRule replaces every match of Re in the raw URL with Replacement.
// Replacement supports $1, $2, ... backreferences (Go regexp syntax).
type RegexRule struct {
	Re          *regexp.Regexp
	Replacement string
}

func (r RegexRule) Apply(rawURL string) (string, bool) {
	if !r.Re.MatchString(rawURL) {
		return rawURL, false
	}
	return r.Re.ReplaceAllString(rawURL, r.Replacement), true
}

func (r RegexRule) String() string {
	return "regex=" + r.Re.String() + " → " + r.Replacement
}

// matchHost: copy of router's helper kept local so rewriter doesn't depend
// on router. Same semantics: exact match, or "*.foo" matches "foo" and any
// "*.foo" subdomain.
func matchHost(host, pattern string) bool {
	if host == "" || pattern == "" {
		return false
	}
	if host == pattern {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		base := pattern[2:]
		return host == base || strings.HasSuffix(host, "."+base)
	}
	return false
}
