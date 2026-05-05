//go:build linux

package opener

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// Open launches the given browser with the URL.
//   - browser ending in ".desktop" → "gio launch <name>.desktop <url>"
//   - anything else               → exec.Command(browser, url)
func Open(browser, url string) error {
	var cmd *exec.Cmd
	if strings.HasSuffix(browser, ".desktop") {
		cmd = exec.Command("gio", "launch", browser, url)
	} else {
		cmd = exec.Command(browser, url)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %q: %w", browser, err)
	}
	return cmd.Process.Release()
}
