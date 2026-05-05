//go:build darwin

package opener

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/maxischmaxi/browser-proxy/internal/browsers"
)

// Open launches the given browser with the URL via macOS' open(1).
//   - bundle ID (contains '.', no '/' and no ".app") → open -b <id>
//   - anything else                                  → open -a <name>
//
// When profile is set, additional --args are forwarded:
//   - Chromium-family → --profile-directory=<resolved>
//   - Firefox-family  → -P <profile> --new-instance
//   - Other           → warning, profile ignored
func Open(browser, profile, url string) error {
	openArgs := []string{"-a", browser}
	if isBundleID(browser) {
		openArgs = []string{"-b", browser}
	}

	if extra := profileArgs(browser, profile); len(extra) > 0 {
		openArgs = append(openArgs, "--args")
		openArgs = append(openArgs, extra...)
		openArgs = append(openArgs, url)
	} else {
		// Without --args, "open" treats the trailing arg as the URL to open.
		openArgs = append(openArgs, url)
	}

	out, err := exec.Command("open", openArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("open %q: %w (%s)", browser, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func profileArgs(browser, profile string) []string {
	if profile == "" {
		return nil
	}
	switch browsers.DetectFamily(browser) {
	case browsers.Chromium:
		return []string{"--profile-directory=" + browsers.ResolveChromiumProfile(browser, profile)}
	case browsers.Firefox:
		return []string{"-P", profile, "--new-instance"}
	default:
		fmt.Fprintf(os.Stderr,
			"warning: profile %q ignored: unknown browser family for %q "+
				"(supported: Chromium-likes, Firefox-likes)\n",
			profile, browser)
		return nil
	}
}

func isBundleID(s string) bool {
	return strings.Contains(s, ".") &&
		!strings.Contains(s, "/") &&
		!strings.HasSuffix(strings.ToLower(s), ".app")
}
