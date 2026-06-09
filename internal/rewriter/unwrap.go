package rewriter

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
)

// Built-in unwrappers strip the wrapper layer that messaging/email apps add
// around outbound links so that routing rules see the actual destination.
//
// Recognized wrappers:
//
//   - Slack "Sign in with Slack" OIDC redirect:
//     https://slack.com/openid/connect/login_initiate_redirect?login_hint=<JWT>
//     where the JWT payload contains a "https://slack.com/target_uri" claim.
//
//   - Microsoft Defender for Office 365 Safe Links (Outlook, Teams):
//     https://*.safelinks.protection.outlook.com/?url=<encoded>&data=…
//
//   - Microsoft Teams Safe Links interstitial ("Verifying Link…"):
//     https://statics.teams.cdn.office.net/evergreen-assets/safelinks/1/atp-safelinks.html?url=<encoded>&dest=…&pc=…
//     where the clicked destination is in the "url" query param.
//
//   - Google URL wrapper (Gmail, Calendar, Search results):
//     https://www.google.com/url?q=<encoded>&sa=…
//
//   - LinkedIn redirect:
//     https://www.linkedin.com/redir/redirect?url=<encoded>
//
//   - Facebook link shim (desktop + mobile):
//     https://l.facebook.com/l.php?u=<encoded>
//
//   - YouTube outbound redirect:
//     https://www.youtube.com/redirect?q=<encoded>
//
// JWT payloads are NOT cryptographically verified — that's the next hop's
// responsibility. We only extract the target URL.

type unwrapper struct {
	name string
	fn   func(*url.URL) (string, bool)
}

var builtinUnwrappers = []unwrapper{
	{"slack-oidc", unwrapSlackOIDC},
	{"microsoft-safelinks", unwrapMicrosoftSafeLinks},
	{"teams-safelinks", unwrapTeamsSafeLinks},
	{"google-url", unwrapGoogleURL},
	{"linkedin-redir", unwrapLinkedIn},
	{"facebook-l", unwrapFacebook},
	{"youtube-redirect", unwrapYouTube},
}

// unwrapAll repeatedly unwraps wrapper URLs until no rule matches or the
// iteration cap is reached (defends against pathological loops). Returns
// (rewritten, true) when at least one layer was peeled off.
func unwrapAll(rawURL string) (string, bool) {
	const maxIterations = 5
	current := rawURL
	changed := false
	for i := 0; i < maxIterations; i++ {
		next, ok := unwrapOnce(current)
		if !ok {
			break
		}
		current = next
		changed = true
	}
	return current, changed
}

func unwrapOnce(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL, false
	}
	for _, w := range builtinUnwrappers {
		if target, ok := w.fn(u); ok && isHTTPURL(target) {
			return target, true
		}
	}
	return rawURL, false
}

// isHTTPURL whitelists the schemes we accept as unwrap output. Prevents an
// attacker from smuggling javascript:/file:/data: URLs into a wrapper param.
func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")
}

// hostEquals reports whether u.Host (case-insensitive) matches one of the
// allowed values. Exact match only — no subdomain wildcard.
func hostEquals(u *url.URL, allowed ...string) bool {
	h := strings.ToLower(u.Host)
	for _, a := range allowed {
		if h == a {
			return true
		}
	}
	return false
}

// hostHasSuffix reports whether u.Host ends with .suffix, case-insensitive.
func hostHasSuffix(u *url.URL, suffix string) bool {
	h := strings.ToLower(u.Host)
	s := strings.ToLower(suffix)
	return h == strings.TrimPrefix(s, ".") || strings.HasSuffix(h, s)
}

// ── Slack OIDC ────────────────────────────────────────────────────────────

func unwrapSlackOIDC(u *url.URL) (string, bool) {
	if !hostEquals(u, "slack.com") {
		return "", false
	}
	if !strings.HasPrefix(u.Path, "/openid/connect/") {
		return "", false
	}
	hint := u.Query().Get("login_hint")
	if hint == "" {
		return "", false
	}
	parts := strings.SplitN(hint, ".", 3)
	if len(parts) != 3 {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", false
	}
	target, _ := claims["https://slack.com/target_uri"].(string)
	return target, target != ""
}

// ── Microsoft Safe Links (Outlook ATP + Teams) ────────────────────────────

func unwrapMicrosoftSafeLinks(u *url.URL) (string, bool) {
	if !hostEquals(u, "safelinks.protection.outlook.com") &&
		!hostHasSuffix(u, ".safelinks.protection.outlook.com") {
		return "", false
	}
	target := u.Query().Get("url")
	return target, target != ""
}

// ── Microsoft Teams Safe Links interstitial ───────────────────────────────

// unwrapTeamsSafeLinks peels the Teams ATP "Verifying Link…" interstitial that
// Teams routes outbound clicks through. It lives on the Office CDN — a
// different host than Outlook's *.safelinks.protection.outlook.com — so it
// needs its own handler. The clicked destination is in the "url" query param.
// Teams sometimes nests this page inside itself (a double interstitial);
// unwrapAll's recursion peels each layer.
func unwrapTeamsSafeLinks(u *url.URL) (string, bool) {
	if !hostEquals(u, "statics.teams.cdn.office.net") {
		return "", false
	}
	if !strings.HasPrefix(u.Path, "/evergreen-assets/safelinks/") {
		return "", false
	}
	target := u.Query().Get("url")
	return target, target != ""
}

// ── Google URL wrapper ────────────────────────────────────────────────────

func unwrapGoogleURL(u *url.URL) (string, bool) {
	if !hostEquals(u, "www.google.com", "google.com") {
		return "", false
	}
	if u.Path != "/url" {
		return "", false
	}
	q := u.Query()
	if t := q.Get("q"); t != "" {
		return t, true
	}
	if t := q.Get("url"); t != "" {
		return t, true
	}
	return "", false
}

// ── LinkedIn redirect ─────────────────────────────────────────────────────

func unwrapLinkedIn(u *url.URL) (string, bool) {
	if !hostEquals(u, "www.linkedin.com", "linkedin.com") {
		return "", false
	}
	if !strings.HasPrefix(u.Path, "/redir/") {
		return "", false
	}
	target := u.Query().Get("url")
	return target, target != ""
}

// ── Facebook l.php ────────────────────────────────────────────────────────

func unwrapFacebook(u *url.URL) (string, bool) {
	if !hostEquals(u, "l.facebook.com", "lm.facebook.com") {
		return "", false
	}
	if u.Path != "/l.php" {
		return "", false
	}
	target := u.Query().Get("u")
	return target, target != ""
}

// ── YouTube outbound redirect ─────────────────────────────────────────────

func unwrapYouTube(u *url.URL) (string, bool) {
	if !hostEquals(u, "www.youtube.com", "youtube.com") {
		return "", false
	}
	if u.Path != "/redirect" {
		return "", false
	}
	target := u.Query().Get("q")
	return target, target != ""
}
