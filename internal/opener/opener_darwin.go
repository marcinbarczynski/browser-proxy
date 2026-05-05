//go:build darwin

package opener

import (
	"fmt"
	"os/exec"
	"strings"
)

// Open launches the given browser with the URL via macOS' open(1).
//   - bundle ID (contains '.', no '/' and no ".app") → open -b <id>
//   - anything else                                  → open -a <name>
func Open(browser, url string) error {
	args := []string{"-a", browser, url}
	if isBundleID(browser) {
		args = []string{"-b", browser, url}
	}
	out, err := exec.Command("open", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("open %q: %w (%s)", browser, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isBundleID(s string) bool {
	return strings.Contains(s, ".") &&
		!strings.Contains(s, "/") &&
		!strings.HasSuffix(strings.ToLower(s), ".app")
}
