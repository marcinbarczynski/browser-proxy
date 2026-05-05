//go:build linux

package opener

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/maxischmaxi/browser-proxy/internal/browsers"
)

// Open launches the given browser with the URL, optionally targeting a profile.
//   - browser ending in ".desktop"   → "gio launch <name>.desktop <url>"
//                                      (profile is ignored, with a warning)
//   - Chromium-family + profile      → exec browser --profile-directory=<resolved> <url>
//   - Firefox-family + profile       → exec browser -P <profile> --new-instance <url>
//   - anything else                  → exec browser <url>
func Open(browser, profile, url string) error {
	if strings.HasSuffix(browser, ".desktop") {
		if profile != "" {
			fmt.Fprintf(os.Stderr,
				"warning: profile %q ignored — gio launch can't pass extra flags. "+
					"Use a binary name (e.g. firefox / google-chrome) instead of %s.\n",
				profile, browser)
		}
		return runDetached(exec.Command("gio", "launch", browser, url), browser)
	}

	args := profileArgs(browser, profile)
	args = append(args, url)
	return runDetached(exec.Command(browser, args...), browser)
}

func profileArgs(browser, profile string) []string {
	if profile == "" {
		return nil
	}
	switch browsers.DetectFamily(browser) {
	case browsers.Chromium:
		return []string{"--profile-directory=" + browsers.ResolveChromiumProfile(browser, profile)}
	case browsers.Firefox:
		// --new-instance is needed when Firefox is already running with another
		// profile; otherwise -P would be silently ignored via Firefox's remoting.
		return []string{"-P", profile, "--new-instance"}
	default:
		fmt.Fprintf(os.Stderr,
			"warning: profile %q ignored: unknown browser family for %q "+
				"(supported: Chromium-likes, Firefox-likes)\n",
			profile, browser)
		return nil
	}
}

func runDetached(cmd *exec.Cmd, browser string) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %q: %w", browser, err)
	}
	return cmd.Process.Release()
}
