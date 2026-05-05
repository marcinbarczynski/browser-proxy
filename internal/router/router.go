package router

import "net/url"

type Rule struct {
	Match   Matcher
	Browser string
}

type Router struct {
	Default string
	Rules   []Rule
}

// Resolve picks the browser for rawURL. If no rule matches (or the URL is
// unparseable) the default is returned. ruleIndex is -1 when the default
// applies, otherwise the index of the matched rule.
func (r *Router) Resolve(rawURL string) (browser string, ruleIndex int, err error) {
	u, _ := url.Parse(rawURL)
	for i, rule := range r.Rules {
		if rule.Match.Matches(u, rawURL) {
			return rule.Browser, i, nil
		}
	}
	return r.Default, -1, nil
}
