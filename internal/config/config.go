package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/max/browser-proxy/internal/router"
)

type rawConfig struct {
	Default string    `toml:"default"`
	Rules   []rawRule `toml:"rules"`
}

type rawRule struct {
	Prefix  string `toml:"prefix"`
	Suffix  string `toml:"suffix"`
	Regex   string `toml:"regex"`
	Host    string `toml:"host"`
	Browser string `toml:"browser"`
}

// DefaultPath returns the XDG config path used by the tool.
func DefaultPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "browser-proxy", "config.toml")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "browser-proxy", "config.toml")
}

// Load reads, parses and validates a config file, returning a populated Router.
func Load(path string) (*router.Router, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var raw rawConfig
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if raw.Default == "" {
		return nil, errors.New("config: 'default' must be set")
	}

	r := &router.Router{Default: raw.Default}
	for i, rr := range raw.Rules {
		if rr.Browser == "" {
			return nil, fmt.Errorf("rule %d: 'browser' is required", i)
		}
		m, err := buildMatcher(i, rr)
		if err != nil {
			return nil, err
		}
		r.Rules = append(r.Rules, router.Rule{Match: m, Browser: rr.Browser})
	}
	return r, nil
}

func buildMatcher(i int, rr rawRule) (router.Matcher, error) {
	count := 0
	for _, s := range []string{rr.Prefix, rr.Suffix, rr.Regex, rr.Host} {
		if s != "" {
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("rule %d: exactly one of prefix/suffix/regex/host must be set (got %d)", i, count)
	}
	switch {
	case rr.Prefix != "":
		return router.PrefixMatcher{Prefix: rr.Prefix}, nil
	case rr.Suffix != "":
		return router.SuffixMatcher{Suffix: rr.Suffix}, nil
	case rr.Host != "":
		return router.HostMatcher{Pattern: rr.Host}, nil
	case rr.Regex != "":
		re, err := regexp.Compile(rr.Regex)
		if err != nil {
			return nil, fmt.Errorf("rule %d: invalid regex %q: %w", i, rr.Regex, err)
		}
		return router.RegexMatcher{Re: re}, nil
	}
	return nil, errors.New("unreachable")
}

const Example = `# Browser Proxy configuration
# Edit this file and your default browser will route URLs based on the rules below.
# First matching rule wins; if no rule matches, "default" is used.

default = "Google Chrome"

# Match by URL prefix
[[rules]]
prefix = "https://github.com/"
browser = "Firefox"

# Match by hostname (supports "*." wildcard, matches the apex too)
[[rules]]
host = "*.atlassian.net"
browser = "Firefox"

# Match by regular expression (Go re2 syntax — escape backslashes in TOML)
[[rules]]
regex = "^https://meet\\.google\\.com"
browser = "Google Chrome"

# Match by path suffix (case-insensitive)
[[rules]]
suffix = ".pdf"
browser = "Preview"
`

// WriteExample writes the example config to path, creating parent dirs.
func WriteExample(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Example), 0o644)
}
