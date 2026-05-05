// Package browsers contains browser-family heuristics and per-family
// helpers (e.g. profile resolution) that the opener uses to translate a
// rule's "browser + profile" into the right launch arguments.
package browsers

import "strings"

type Family int

const (
	Other Family = iota
	Chromium
	Firefox
)

func (f Family) String() string {
	switch f {
	case Chromium:
		return "chromium"
	case Firefox:
		return "firefox"
	default:
		return "other"
	}
}

var firefoxKeywords = []string{
	"firefox", "librewolf", "waterfox", "tor browser", "icecat", "zen browser",
}

var chromiumKeywords = []string{
	"chrome", "chromium", "brave", "edge", "vivaldi",
	"opera", "thorium", "arc", "yandex", "comet",
}

// DetectFamily classifies a browser name by substring match (case-insensitive).
// Order matters: Firefox is checked first because some Firefox forks could
// otherwise overlap (none currently do, but it's the safer ordering).
func DetectFamily(name string) Family {
	n := strings.ToLower(name)
	for _, k := range firefoxKeywords {
		if strings.Contains(n, k) {
			return Firefox
		}
	}
	for _, k := range chromiumKeywords {
		if strings.Contains(n, k) {
			return Chromium
		}
	}
	return Other
}
