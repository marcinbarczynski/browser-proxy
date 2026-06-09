package rewriter

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// makeSlackJWT builds a synthetic, unsigned Slack-style JWT carrying target_uri.
// We never verify signatures, so an arbitrary "sig" is fine.
func makeSlackJWT(targetURI string) string {
	header := `{"alg":"none","typ":"JWT"}`
	payload := fmt.Sprintf(`{"iss":"https://slack.com","sub":"u@example.com","aud":"a","https://slack.com/target_uri":"%s"}`, targetURI)
	enc := func(s string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(s))
	}
	return enc(header) + "." + enc(payload) + ".sig"
}

// ── Slack OIDC ────────────────────────────────────────────────────────────

func TestUnwrapSlackOIDC(t *testing.T) {
	target := "https://tuv-rh.atlassian.net/browse/GALAXY-3873"
	wrapper := "https://slack.com/openid/connect/login_initiate_redirect?login_hint=" + makeSlackJWT(target)

	got, ok := unwrapOnce(wrapper)
	if !ok {
		t.Fatalf("expected unwrap to succeed")
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}
}

func TestSlackOIDC_WrongHost(t *testing.T) {
	wrapper := "https://slack.example.com/openid/connect/login_initiate_redirect?login_hint=" + makeSlackJWT("https://x.com")
	if _, ok := unwrapOnce(wrapper); ok {
		t.Errorf("must not unwrap fake slack host")
	}
}

func TestSlackOIDC_WrongPath(t *testing.T) {
	wrapper := "https://slack.com/api/something?login_hint=" + makeSlackJWT("https://x.com")
	if _, ok := unwrapOnce(wrapper); ok {
		t.Errorf("must not unwrap non-OIDC path")
	}
}

func TestSlackOIDC_NoLoginHint(t *testing.T) {
	if _, ok := unwrapOnce("https://slack.com/openid/connect/login_initiate_redirect"); ok {
		t.Errorf("must not unwrap without login_hint")
	}
}

func TestSlackOIDC_MalformedJWT(t *testing.T) {
	cases := []string{
		"https://slack.com/openid/connect/foo?login_hint=notajwt",
		"https://slack.com/openid/connect/foo?login_hint=a.b",                            // only 2 parts
		"https://slack.com/openid/connect/foo?login_hint=" + "***" + ".***" + ".***",    // bad base64
		"https://slack.com/openid/connect/foo?login_hint=" + "aGVsbG8" + ".aGVsbG8.sig", // not JSON
	}
	for _, c := range cases {
		if _, ok := unwrapOnce(c); ok {
			t.Errorf("unexpectedly unwrapped %q", c)
		}
	}
}

func TestSlackOIDC_NoTargetURIClaim(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"slack.com","sub":"u"}`))
	jwt := header + "." + payload + ".sig"
	wrapper := "https://slack.com/openid/connect/login_initiate_redirect?login_hint=" + jwt
	if _, ok := unwrapOnce(wrapper); ok {
		t.Errorf("must not unwrap without target_uri claim")
	}
}

func TestSlackOIDC_RejectsNonHTTPSchemes(t *testing.T) {
	cases := []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html,<script>",
		"ftp://example.com/x",
	}
	for _, target := range cases {
		wrapper := "https://slack.com/openid/connect/login_initiate_redirect?login_hint=" + makeSlackJWT(target)
		if got, ok := unwrapOnce(wrapper); ok {
			t.Errorf("must reject scheme in %q (got %q)", target, got)
		}
	}
}

// ── Microsoft Safe Links ──────────────────────────────────────────────────

func TestUnwrapSafeLinks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"nam04 region",
			"https://nam04.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com%2Fpage&data=abc&sdata=def",
			"https://example.com/page",
		},
		{
			"eur01 region",
			"https://eur01.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com%2F",
			"https://example.com/",
		},
		{
			"bare host (no region prefix)",
			"https://safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com",
			"https://example.com",
		},
		{
			"capitalised host",
			"https://NAM04.SAFELINKS.PROTECTION.OUTLOOK.COM/?url=https%3A%2F%2Fexample.com",
			"https://example.com",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := unwrapOnce(c.in)
			if !ok {
				t.Fatalf("expected unwrap")
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestSafeLinks_NoURLParam(t *testing.T) {
	if _, ok := unwrapOnce("https://nam04.safelinks.protection.outlook.com/?data=abc"); ok {
		t.Errorf("must not unwrap without url param")
	}
}

func TestSafeLinks_NotARealSafeLinksHost(t *testing.T) {
	// Look-alike, but not actually a Microsoft host.
	if _, ok := unwrapOnce("https://safelinks.protection.outlook.com.attacker.com/?url=https://x.com"); ok {
		t.Errorf("must not unwrap attacker-controlled subdomain")
	}
}

// ── Microsoft Teams Safe Links interstitial ───────────────────────────────

func TestUnwrapTeamsSafeLinks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"basic interstitial",
			"https://statics.teams.cdn.office.net/evergreen-assets/safelinks/1/atp-safelinks.html?url=https%3A%2F%2Fexample.com%2Fpage&locale=en-us&dest=https%3A%2F%2Fteams.microsoft.com%2Fapi&pc=abc",
			"https://example.com/page",
		},
		{
			"capitalised host",
			"https://STATICS.TEAMS.CDN.OFFICE.NET/evergreen-assets/safelinks/1/atp-safelinks.html?url=https%3A%2F%2Fexample.com",
			"https://example.com",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := unwrapOnce(c.in)
			if !ok {
				t.Fatalf("expected unwrap")
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestTeamsSafeLinks_NoURLParam(t *testing.T) {
	in := "https://statics.teams.cdn.office.net/evergreen-assets/safelinks/1/atp-safelinks.html?dest=abc"
	if _, ok := unwrapOnce(in); ok {
		t.Errorf("must not unwrap without url param")
	}
}

func TestTeamsSafeLinks_WrongPath(t *testing.T) {
	// Same CDN host, but not a safelinks asset.
	in := "https://statics.teams.cdn.office.net/evergreen-assets/icons/foo.png?url=https://x.com"
	if _, ok := unwrapOnce(in); ok {
		t.Errorf("must not unwrap non-safelinks path on the Teams CDN")
	}
}

func TestTeamsSafeLinks_LookAlikeHost(t *testing.T) {
	in := "https://statics.teams.cdn.office.net.attacker.com/evergreen-assets/safelinks/1/atp-safelinks.html?url=https://x.com"
	if _, ok := unwrapOnce(in); ok {
		t.Errorf("must not unwrap attacker-controlled host")
	}
}

func TestUnwrapTeams_DoubleInterstitial(t *testing.T) {
	// Teams occasionally wraps its own interstitial inside another one.
	inner := "https://www.linkedin.com/"
	once := "https://statics.teams.cdn.office.net/evergreen-assets/safelinks/1/atp-safelinks.html?url=" + url.QueryEscape(inner)
	twice := "https://statics.teams.cdn.office.net/evergreen-assets/safelinks/1/atp-safelinks.html?url=" + url.QueryEscape(once)

	got, changed := unwrapAll(twice)
	if !changed {
		t.Fatal("expected unwrapAll to change the URL")
	}
	if got != inner {
		t.Errorf("after recursive unwrap: got %q, want %q", got, inner)
	}
}

// ── Google /url ───────────────────────────────────────────────────────────

func TestUnwrapGoogleURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.google.com/url?q=https%3A%2F%2Fexample.com&sa=t&ust=1", "https://example.com"},
		{"https://google.com/url?q=https%3A%2F%2Fexample.com%2Fpage", "https://example.com/page"},
		{"https://www.google.com/url?url=https%3A%2F%2Fexample.com", "https://example.com"}, // url param fallback
	}
	for _, c := range cases {
		got, ok := unwrapOnce(c.in)
		if !ok {
			t.Errorf("expected unwrap of %q", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestGoogle_WrongPath(t *testing.T) {
	if _, ok := unwrapOnce("https://www.google.com/search?q=foo"); ok {
		t.Errorf("must not unwrap /search")
	}
}

// ── LinkedIn ──────────────────────────────────────────────────────────────

func TestUnwrapLinkedIn(t *testing.T) {
	in := "https://www.linkedin.com/redir/redirect?url=https%3A%2F%2Fexample.com%2Fjob&urlhash=abc"
	want := "https://example.com/job"
	got, ok := unwrapOnce(in)
	if !ok || got != want {
		t.Errorf("got %q,%v want %q,true", got, ok, want)
	}
}

func TestLinkedIn_LookAlikeHost(t *testing.T) {
	if _, ok := unwrapOnce("https://www.linkedin.attacker.com/redir/redirect?url=https://x.com"); ok {
		t.Errorf("must not unwrap fake linkedin host")
	}
}

// ── Facebook ──────────────────────────────────────────────────────────────

func TestUnwrapFacebook(t *testing.T) {
	cases := []string{
		"https://l.facebook.com/l.php?u=https%3A%2F%2Fexample.com%2Fpost&h=abc",
		"https://lm.facebook.com/l.php?u=https%3A%2F%2Fexample.com%2Fpost",
	}
	for _, in := range cases {
		got, ok := unwrapOnce(in)
		if !ok || got != "https://example.com/post" {
			t.Errorf("Facebook unwrap %q: got %q,%v", in, got, ok)
		}
	}
}

// ── YouTube ───────────────────────────────────────────────────────────────

func TestUnwrapYouTube(t *testing.T) {
	in := "https://www.youtube.com/redirect?q=https%3A%2F%2Fexample.com&v=abc"
	got, ok := unwrapOnce(in)
	if !ok || got != "https://example.com" {
		t.Errorf("YouTube unwrap: got %q,%v", got, ok)
	}
}

// ── Recursion (nested wrappers) ───────────────────────────────────────────

func TestUnwrap_NestedSlackInsideSafeLinks(t *testing.T) {
	// Real-world case: an Outlook email contains a Slack OIDC link,
	// so Microsoft wraps the Slack wrapper.
	innerTarget := "https://tuv-rh.atlassian.net/browse/GALAXY-1"
	slackURL := "https://slack.com/openid/connect/login_initiate_redirect?login_hint=" + makeSlackJWT(innerTarget)
	outer := "https://nam04.safelinks.protection.outlook.com/?url=" + url.QueryEscape(slackURL)

	got, changed := unwrapAll(outer)
	if !changed {
		t.Fatal("expected unwrapAll to change the URL")
	}
	if got != innerTarget {
		t.Errorf("after recursive unwrap: got %q, want %q", got, innerTarget)
	}
}

func TestUnwrap_NoChange(t *testing.T) {
	cases := []string{
		"https://github.com/foo/bar",
		"https://example.com/page?id=42",
		"https://meet.google.com/abc-defg",
		"not a url at all",
		"",
	}
	for _, c := range cases {
		got, changed := unwrapAll(c)
		if changed {
			t.Errorf("unwrapAll(%q) reported change, got %q", c, got)
		}
		if got != c {
			t.Errorf("unwrapAll(%q) altered URL to %q", c, got)
		}
	}
}

// ── Integration with Rewriter ─────────────────────────────────────────────

func TestRewriter_UnwrapBeforeStripParams(t *testing.T) {
	// Wrapper with utm_* on the *target* URL — unwrap first, then strip from target.
	target := "https://example.com/page?utm_source=email&id=42"
	wrapper := "https://nam04.safelinks.protection.outlook.com/?url=" + url.QueryEscape(target) + "&data=opaque"

	r := &Rewriter{
		UnwrapRedirects: true,
		StripParams:     []ParamMatcher{NewParamMatcher("utm_*")},
	}
	got := r.Apply(wrapper)
	want := "https://example.com/page?id=42"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewriter_UnwrapDisabled(t *testing.T) {
	wrapper := "https://nam04.safelinks.protection.outlook.com/?url=" + url.QueryEscape("https://example.com/page")
	r := &Rewriter{UnwrapRedirects: false}
	if got := r.Apply(wrapper); got != wrapper {
		t.Errorf("unwrap=false should leave URL untouched, got %q", got)
	}
}

func TestRewriter_ForceHTTPSThenUnwrap(t *testing.T) {
	// http://safelinks → https://safelinks → unwrap → target
	target := "https://example.com/page"
	wrapper := "http://nam04.safelinks.protection.outlook.com/?url=" + url.QueryEscape(target)
	r := &Rewriter{ForceHTTPS: true, UnwrapRedirects: true}
	if got := r.Apply(wrapper); got != target {
		t.Errorf("got %q, want %q", got, target)
	}
}

// ── Sanity: no infinite loops ─────────────────────────────────────────────

func TestUnwrap_StableFixedPoint(t *testing.T) {
	// After unwrapping, applying unwrap again must produce the same URL.
	target := "https://github.com/foo"
	wrapper := "https://slack.com/openid/connect/login_initiate_redirect?login_hint=" + makeSlackJWT(target)

	once, _ := unwrapAll(wrapper)
	twice, _ := unwrapAll(once)
	if once != twice {
		t.Errorf("unwrap is not idempotent: %q → %q", once, twice)
	}

	// And don't introduce stray characters
	if !strings.HasPrefix(once, "https://github.com/") {
		t.Errorf("unexpected unwrap result: %q", once)
	}
}
