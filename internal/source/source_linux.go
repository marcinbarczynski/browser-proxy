//go:build linux

package source

import (
	"os"
	"strconv"
	"strings"
)

// Detect walks the parent process chain looking for the first non-launcher
// process. /proc-based; returns an empty Info if nothing useful is found.
func Detect() Info {
	pid := os.Getppid()
	for i := 0; i < 6 && pid > 1; i++ {
		name := readComm(pid)
		if name != "" && !isLauncher(name) {
			return Info{Name: name}
		}
		pid = readPPID(pid)
	}
	return Info{}
}

func readComm(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readPPID(pid int) int {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0
	}
	return parsePPIDFromStat(string(data))
}

// parsePPIDFromStat extracts the parent pid from a /proc/<pid>/stat line.
// The comm field is wrapped in parens and may contain spaces or parens itself,
// so we anchor on the LAST close-paren.
func parsePPIDFromStat(s string) int {
	rparen := strings.LastIndex(s, ")")
	if rparen < 0 || rparen+2 > len(s) {
		return 0
	}
	fields := strings.Fields(s[rparen+1:])
	if len(fields) < 2 {
		return 0
	}
	p, _ := strconv.Atoi(fields[1])
	return p
}

// isLauncher reports whether name is a generic launcher we should skip when
// walking up the process tree to find the real source app. comm is truncated
// to 15 chars on Linux, hence the abbreviated entries.
func isLauncher(name string) bool {
	switch name {
	case
		"xdg-open", "xdg-mime",
		"mimeopen", "mimetype",
		"gtk-launch", "gio", "gio-launch-deskto", "gio-launch-desk",
		"kde-open5", "kde-open",
		"exo-open",
		"dde-open", "dde-file-mana",
		"flatpak-portal", "xdg-desktop-por", "xdg-desktop-portal",
		"sh", "bash", "zsh", "dash", "fish",
		"systemd", "init",
		"browser-proxy":
		return true
	}
	return false
}
