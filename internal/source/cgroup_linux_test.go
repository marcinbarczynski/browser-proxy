//go:build linux

package source

import (
	"strings"
	"testing"
)

func TestSelfCgroupPath(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "cgroup v2 unified",
			lines: []string{"0::/user.slice/user@1000.service/app.slice/app-slack-4711.scope"},
			want:  "/user.slice/user@1000.service/app.slice/app-slack-4711.scope",
		},
		{
			name: "v2 line wins over v1 leftovers",
			lines: []string{
				"1:name=systemd:/legacy",
				"0::/user.slice/app-slack-4711.scope",
			},
			want: "/user.slice/app-slack-4711.scope",
		},
		{
			name:  "cgroup v1 systemd hierarchy need not be first",
			lines: []string{"2:cpu:/", "1:name=systemd:/user.slice/app-slack-4711.scope"},
			want:  "/user.slice/app-slack-4711.scope",
		},
		{
			name:  "v1 systemd hierarchy beats empty hybrid path",
			lines: []string{"0::/", "1:name=systemd:/user.slice/app-slack-4711.scope"},
			want:  "/user.slice/app-slack-4711.scope",
		},
		{
			name:  "garbage",
			lines: []string{"nonsense", ""},
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.setSelfCgroup(tc.lines...)
			if got := selfCgroupPath(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestScopeUnit(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/user.slice/user@1000.service/app.slice/app-slack-4711.scope", "app-slack-4711.scope"},
		{"/user.slice/user@1000.service/app.slice/app-slack-4711.scope/renderer", "app-slack-4711.scope"},
		{"/system.slice/foo.service", "foo.service"},
		// user@1000.service encloses the scope, but the innermost unit wins.
		{"/user.slice/user@1000.service/session.scope", "session.scope"},
		{"/user.slice", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := scopeUnit(tc.in); got != tc.want {
			t.Errorf("scopeUnit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestScopeAppNames(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"app-slack-4711.scope", []string{"slack"}},
		{"app-slack.scope", []string{"slack"}},
		{"app-gnome-slack-4711.scope", []string{"slack"}},
		{"app-slack@1.service", []string{"slack"}},
		{"app-slack-4711.service", []string{"slack"}},
		// systemd escapes dashes in desktop ids as \x2d.
		{`app-teams\x2dfor\x2dlinux-4711.scope`, []string{"teams-for-linux"}},
		// flatpak: the reverse-DNS id and its final component are both usable.
		{"app-flatpak-com.slack.Slack-4711.scope", []string{"com.slack.Slack", "Slack"}},
		// not launcher-created: no .desktop name to recover.
		{"session-2.scope", nil},
		{"vte-spawn-abc.scope", nil},
		{"init.scope", nil},
		// nothing usable left after stripping.
		{"app-4711.scope", nil},
		{"app-.scope", nil},
		// a shell is never the app that opened a link.
		{"app-bash-123.scope", nil},
		{"", nil},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := scopeAppNames(tc.in); !equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUnescapeUnit(t *testing.T) {
	cases := []struct{ in, want string }{
		{"slack", "slack"},
		{`teams\x2dfor\x2dlinux`, "teams-for-linux"},
		{`trailing\x2`, `trailing\x2`}, // truncated escape stays as-is
		{`bad\xzz`, `bad\xzz`},
	}
	for _, tc := range cases {
		if got := unescapeUnit(tc.in); got != tc.want {
			t.Errorf("unescapeUnit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTrimPidSuffix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"slack-4711", "slack"},
		{"slack", "slack"},
		{"teams-for-linux-1", "teams-for-linux"},
		{"slack-", "slack-"},
		{"4711", "4711"},
	}
	for _, tc := range cases {
		if got := trimPidSuffix(tc.in); got != tc.want {
			t.Errorf("trimPidSuffix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDebugLinesNeverPanics(t *testing.T) {
	f := newFixture(t)
	f.setSelfCgroup("0::/" + slackScope)
	f.addProc(4711, "slack", "/usr/lib/slack/slack", 1)

	lines := DebugLines()
	if len(lines) == 0 {
		t.Fatal("no debug lines")
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"cgroup:", "scope unit:", "ancestors:", "scope names:", "slack"} {
		if !strings.Contains(joined, want) {
			t.Errorf("debug output missing %q:\n%s", want, joined)
		}
	}
}
