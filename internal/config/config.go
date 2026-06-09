package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/maxischmaxi/browser-proxy/internal/logging"
	"github.com/maxischmaxi/browser-proxy/internal/rewriter"
	"github.com/maxischmaxi/browser-proxy/internal/router"
)

// Config bundles everything Load returns: routing table, URL rewriter, and
// logger. Caller must call Close to release the log file handle.
type Config struct {
	Router   *router.Router
	Rewriter *rewriter.Rewriter
	Log      *logging.Logger
}

// Close releases resources (currently: the log file). Safe on nil.
func (c *Config) Close() error {
	if c == nil {
		return nil
	}
	return c.Log.Close()
}

type rawConfig struct {
	Default         string       `toml:"default"`
	ForceHTTPS      bool         `toml:"force_https"`
	UnwrapRedirects *bool        `toml:"unwrap_redirects"` // *bool so unset defaults to true
	StripParams     []string     `toml:"strip_params"`
	Rewrites        []rawRewrite `toml:"rewrites"`
	Rules           []rawRule    `toml:"rules"`
	Log             bool         `toml:"log"`
	LogFile         string       `toml:"log_file"`
}

type rawRule struct {
	Prefix  string `toml:"prefix"`
	Suffix  string `toml:"suffix"`
	Regex   string `toml:"regex"`
	Host    string `toml:"host"`
	Source  string `toml:"source"`
	Browser string `toml:"browser"`
	Profile string `toml:"profile"`
}

type rawRewrite struct {
	Host            string `toml:"host"`
	ReplacementHost string `toml:"replacement_host"`
	Regex           string `toml:"regex"`
	Replacement     string `toml:"replacement"`
}

// DefaultPath returns the XDG config path used by the tool.
func DefaultPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "browser-proxy", "config.toml")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "browser-proxy", "config.toml")
}

// Load reads, parses and validates a config file.
func Load(path string) (*Config, error) {
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

	rt, err := buildRouter(raw)
	if err != nil {
		return nil, err
	}
	rw, err := buildRewriter(raw)
	if err != nil {
		return nil, err
	}

	// Logger opens the file lazily on first write — keeps dry-runs (test) clean.
	log := logging.New(raw.Log, raw.LogFile)
	return &Config{Router: rt, Rewriter: rw, Log: log}, nil
}

func buildRouter(raw rawConfig) (*router.Router, error) {
	r := &router.Router{Default: raw.Default}
	for i, rr := range raw.Rules {
		rule, err := buildRule(i, rr)
		if err != nil {
			return nil, err
		}
		r.Rules = append(r.Rules, rule)
	}
	return r, nil
}

func buildRule(i int, rr rawRule) (router.Rule, error) {
	if rr.Browser == "" {
		return router.Rule{}, fmt.Errorf("rule %d: 'browser' is required", i)
	}

	urlMatchers := 0
	for _, s := range []string{rr.Prefix, rr.Suffix, rr.Regex, rr.Host} {
		if s != "" {
			urlMatchers++
		}
	}
	if urlMatchers > 1 {
		return router.Rule{}, fmt.Errorf("rule %d: at most one of prefix/suffix/regex/host (got %d)", i, urlMatchers)
	}
	if urlMatchers == 0 && rr.Source == "" {
		return router.Rule{}, fmt.Errorf("rule %d: at least one of prefix/suffix/regex/host/source must be set", i)
	}

	rule := router.Rule{Source: rr.Source, Browser: rr.Browser, Profile: rr.Profile}
	switch {
	case rr.Prefix != "":
		rule.URL = router.PrefixMatcher{Prefix: rr.Prefix}
	case rr.Suffix != "":
		rule.URL = router.SuffixMatcher{Suffix: rr.Suffix}
	case rr.Host != "":
		rule.URL = router.HostMatcher{Pattern: rr.Host}
	case rr.Regex != "":
		re, err := regexp.Compile(rr.Regex)
		if err != nil {
			return router.Rule{}, fmt.Errorf("rule %d: invalid regex %q: %w", i, rr.Regex, err)
		}
		rule.URL = router.RegexMatcher{Re: re}
	}
	return rule, nil
}

func buildRewriter(raw rawConfig) (*rewriter.Rewriter, error) {
	rw := &rewriter.Rewriter{ForceHTTPS: raw.ForceHTTPS}
	// unwrap_redirects defaults to true — only off if user explicitly disables.
	rw.UnwrapRedirects = true
	if raw.UnwrapRedirects != nil {
		rw.UnwrapRedirects = *raw.UnwrapRedirects
	}
	for i, p := range raw.StripParams {
		if p == "" {
			return nil, fmt.Errorf("strip_params[%d]: empty pattern", i)
		}
		rw.StripParams = append(rw.StripParams, rewriter.NewParamMatcher(p))
	}
	for i, rr := range raw.Rewrites {
		rule, err := buildRewriteRule(i, rr)
		if err != nil {
			return nil, err
		}
		rw.Rules = append(rw.Rules, rule)
	}
	return rw, nil
}

func buildRewriteRule(i int, rr rawRewrite) (rewriter.Rule, error) {
	hasHost := rr.Host != ""
	hasRegex := rr.Regex != ""
	if hasHost && hasRegex {
		return nil, fmt.Errorf("rewrite %d: cannot mix 'host' with 'regex'", i)
	}
	switch {
	case hasHost:
		if rr.ReplacementHost == "" {
			return nil, fmt.Errorf("rewrite %d: 'replacement_host' is required when 'host' is set", i)
		}
		return rewriter.HostReplaceRule{Host: rr.Host, Replacement: rr.ReplacementHost}, nil
	case hasRegex:
		re, err := regexp.Compile(rr.Regex)
		if err != nil {
			return nil, fmt.Errorf("rewrite %d: invalid regex %q: %w", i, rr.Regex, err)
		}
		return rewriter.RegexRule{Re: re, Replacement: rr.Replacement}, nil
	default:
		return nil, fmt.Errorf("rewrite %d: must specify either 'host'+'replacement_host' or 'regex'+'replacement'", i)
	}
}

const Example = `# Browser Proxy configuration
# First matching rule wins; if no rule matches, "default" is used.

default = "Google Chrome"

# ── Logging ───────────────────────────────────────────────────────────────
# Off by default. When enabled, every routed/rewritten URL is appended to a
# log file (mode 0600). Default location:
#   macOS:  ~/Library/Logs/browser-proxy.log
#   Linux:  $XDG_STATE_HOME/browser-proxy/browser-proxy.log
#           (typically ~/.local/state/browser-proxy/browser-proxy.log)
log = false
# log_file = "~/browser-proxy.log"   # uncomment to override the path

# ── URL rewrites (applied BEFORE routing) ─────────────────────────────────

# Upgrade every http:// to https:// before matching.
force_https = true

# Built-in: peel off known wrapper URLs so routing rules see the actual target.
# Recognized wrappers: Slack OIDC (login_initiate_redirect), Microsoft Safe
# Links (Outlook *.safelinks.protection.outlook.com), Microsoft Teams Safe
# Links interstitial (statics.teams.cdn.office.net/.../atp-safelinks.html),
# Google /url (Gmail), LinkedIn /redir, Facebook l.php, YouTube /redirect.
# Set to false to disable.
unwrap_redirects = true

# Drop these query parameters from every URL. "*" suffix = prefix wildcard.
strip_params = [
  "utm_*",   # utm_source, utm_medium, utm_campaign, …
  "fbclid",
  "gclid",
  "mc_cid",
  "mc_eid",
  "ref",
  "ref_src",
]

# Hostname swap: any visit to twitter.com goes to nitter.net instead.
# [[rewrites]]
# host = "twitter.com"
# replacement_host = "nitter.net"

# Generic regex rewrite (Go re2 syntax; $1, $2 are backreferences).
# [[rewrites]]
# regex = "^https://www\\.google\\.com/search\\?q=(.+)$"
# replacement = "https://duckduckgo.com/?q=$1"

# ── Routing rules (first match wins) ──────────────────────────────────────

[[rules]]
prefix = "https://github.com/"
browser = "Firefox"

[[rules]]
host = "*.atlassian.net"
browser = "Google Chrome"
profile = "Work"

[[rules]]
regex = "^https://meet\\.google\\.com"
browser = "Google Chrome"

[[rules]]
suffix = ".pdf"
browser = "Preview"

[[rules]]
source = "Slack"
browser = "Google Chrome"
profile = "Work"

# Every link clicked from Microsoft Teams opens in Chrome's Work profile.
# macOS: also matchable by bundle ID "com.microsoft.teams2" (new Teams) or
# "com.microsoft.teams" (classic).
[[rules]]
source = "Microsoft Teams"
browser = "Google Chrome"
profile = "Work"

[[rules]]
prefix = "https://docs."
source = "Mail"
browser = "Firefox"
profile = "Personal"
`

// WriteExample writes the example config to path, creating parent dirs.
func WriteExample(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Example), 0o644)
}
