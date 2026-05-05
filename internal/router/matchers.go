package router

import (
	"net/url"
	"regexp"
	"strings"
)

type Matcher interface {
	Matches(u *url.URL, raw string) bool
	String() string
}

type PrefixMatcher struct{ Prefix string }

func (m PrefixMatcher) Matches(_ *url.URL, raw string) bool {
	return strings.HasPrefix(raw, m.Prefix)
}
func (m PrefixMatcher) String() string { return "prefix=" + m.Prefix }

type SuffixMatcher struct{ Suffix string }

func (m SuffixMatcher) Matches(u *url.URL, raw string) bool {
	target := raw
	if u != nil && u.Path != "" {
		target = u.Path
	}
	return strings.HasSuffix(strings.ToLower(target), strings.ToLower(m.Suffix))
}
func (m SuffixMatcher) String() string { return "suffix=" + m.Suffix }

type RegexMatcher struct{ Re *regexp.Regexp }

func (m RegexMatcher) Matches(_ *url.URL, raw string) bool { return m.Re.MatchString(raw) }
func (m RegexMatcher) String() string                      { return "regex=" + m.Re.String() }

type HostMatcher struct{ Pattern string }

func (m HostMatcher) Matches(u *url.URL, _ string) bool {
	if u == nil {
		return false
	}
	return matchHost(strings.ToLower(u.Hostname()), strings.ToLower(m.Pattern))
}
func (m HostMatcher) String() string { return "host=" + m.Pattern }

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
