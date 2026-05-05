//go:build linux

package source

import "testing"

func TestParsePPIDFromStat(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{
			name: "init line",
			in:   "1 (systemd) S 0 1 1 0 -1 4194560 ...",
			want: 0,
		},
		{
			name: "normal child",
			in:   "1234 (slack) R 1000 1234 1000 ...",
			want: 1000,
		},
		{
			name: "comm with spaces",
			in:   "5678 (Web Content) S 4321 5678 4321 ...",
			want: 4321,
		},
		{
			name: "comm with parens inside",
			in:   "9999 (some (nested) name) S 100 9999 100 ...",
			want: 100,
		},
		{
			name: "malformed empty",
			in:   "",
			want: 0,
		},
		{
			name: "malformed no paren",
			in:   "1234 nope nothing",
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePPIDFromStat(tc.in); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestIsLauncher(t *testing.T) {
	mustBe := []string{"xdg-open", "gio", "gio-launch-deskto", "browser-proxy", "bash"}
	mustNot := []string{"slack", "Slack", "firefox", "code", "thunderbird"}
	for _, n := range mustBe {
		if !isLauncher(n) {
			t.Errorf("%q should be a launcher", n)
		}
	}
	for _, n := range mustNot {
		if isLauncher(n) {
			t.Errorf("%q must NOT be classified as launcher", n)
		}
	}
}
