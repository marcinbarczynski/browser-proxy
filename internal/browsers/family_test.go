package browsers

import "testing"

func TestDetectFamily(t *testing.T) {
	cases := []struct {
		name string
		want Family
	}{
		// Chromium-likes
		{"Google Chrome", Chromium},
		{"google-chrome", Chromium},
		{"google-chrome-stable", Chromium},
		{"Chromium", Chromium},
		{"chromium", Chromium},
		{"Brave Browser", Chromium},
		{"brave-browser", Chromium},
		{"Microsoft Edge", Chromium},
		{"microsoft-edge", Chromium},
		{"Vivaldi", Chromium},
		{"Opera", Chromium},
		{"Thorium", Chromium},
		{"Arc", Chromium},
		// Firefox-likes
		{"Firefox", Firefox},
		{"firefox", Firefox},
		{"Firefox Developer Edition", Firefox},
		{"LibreWolf", Firefox},
		{"librewolf", Firefox},
		{"Waterfox", Firefox},
		{"Zen Browser", Firefox},
		// Other
		{"Safari", Other},
		{"Preview", Other},
		{"/bin/echo", Other},
		{"qutebrowser", Other},
		{"", Other},
	}
	for _, tc := range cases {
		if got := DetectFamily(tc.name); got != tc.want {
			t.Errorf("DetectFamily(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
