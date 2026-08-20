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
//     (profile is ignored, with a warning)
//   - Chromium-family + profile      → exec browser --profile-directory=<resolved> <url>
//   - Firefox-family + profile       → exec browser -P <profile> <url>
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
	return runDetached(browserCommand(browser, args...), browser)
}

// browserCommand keeps the browser out of the calling app's systemd scope.
// Desktop-file launches already receive their own scope through gio.
func browserCommand(browser string, args ...string) *exec.Cmd {
	browserPath, browserErr := exec.LookPath(browser)
	if browserErr != nil {
		return exec.Command(browser, args...)
	}
	systemdRun, err := exec.LookPath("systemd-run")
	if err != nil {
		return exec.Command(browserPath, args...)
	}
	// Probe before detaching so user-manager failures remain observable.
	if err := exec.Command(systemdRun, "--user", "--scope", "--quiet", "true").Run(); err != nil {
		return exec.Command(browserPath, args...)
	}
	full := append([]string{"--user", "--scope", "--quiet", "--", browserPath}, args...)
	return exec.Command(systemdRun, full...)
}

func profileArgs(browser, profile string) []string {
	if profile == "" {
		return nil
	}
	switch browsers.DetectFamily(browser) {
	case browsers.Chromium:
		return []string{"--profile-directory=" + browsers.ResolveChromiumProfile(browser, profile)}
	case browsers.Firefox:
		return []string{"-P", profile}
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
