//go:build linux

package source

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	viaAncestor = "ancestor"
	viaScope    = "scope"
)

const maxAncestors = 6

var (
	procRoot = "/proc"
	getppid  = os.Getppid
)

// Detect checks the parent chain first, then the surviving systemd app scope.
func Detect() Info {
	if info := detectAncestors(); !info.Empty() {
		return info
	}
	cg := selfCgroupPath()
	if cg == "" {
		return Info{}
	}
	if names := scopeAppNames(scopeUnit(cg)); len(names) > 0 {
		return Info{Name: names[0], Candidates: names, Via: viaScope}
	}
	return Info{}
}

// detectAncestors keeps the executable basename because comm is truncated.
func detectAncestors() Info {
	pid := getppid()
	for i := 0; i < maxAncestors && pid > 1; i++ {
		name := readComm(pid)
		if name != "" && !isLauncher(name) {
			return Info{
				Name:       name,
				Candidates: dedupe([]string{name, readExeBase(pid)}),
				Via:        viaAncestor,
			}
		}
		pid = readPPID(pid)
	}
	return Info{}
}

func readComm(pid int) string {
	data, err := os.ReadFile(pidPath(pid, "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readExeBase(pid int) string {
	target, err := os.Readlink(pidPath(pid, "exe"))
	if err != nil {
		return ""
	}
	// An upgraded-in-place binary reads back as "/usr/bin/slack (deleted)".
	target = strings.TrimSuffix(target, " (deleted)")
	return filepath.Base(target)
}

func readPPID(pid int) int {
	data, err := os.ReadFile(pidPath(pid, "stat"))
	if err != nil {
		return 0
	}
	return parsePPIDFromStat(string(data))
}

func pidPath(pid int, file string) string {
	return filepath.Join(procRoot, strconv.Itoa(pid), file)
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
		"systemd", "systemd-run", "init",
		"gnome-shell", "plasmashell", "gjs",
		"browser-proxy":
		return true
	}
	return false
}

func DebugLines() []string {
	cg := selfCgroupPath()
	unit := scopeUnit(cg)
	return []string{
		"cgroup:       " + orNone(cg),
		"scope unit:   " + orNone(unit),
		"ancestors:    " + orNone(strings.Join(ancestorChain(), " <- ")),
		"scope names:  " + orNone(strings.Join(scopeAppNames(unit), ", ")),
		"MM_NOTTTY:    " + orUnset(os.Getenv("MM_NOTTTY")) +
			"   (Electron sets this when it spawns xdg-open; not a routing signal)",
	}
}

// ancestorChain lists the parent chain as "comm[pid]", launchers included and
// flagged, so `test -v` shows where and why the walk stopped.
func ancestorChain() []string {
	var out []string
	pid := getppid()
	for i := 0; i < maxAncestors && pid > 1; i++ {
		name := readComm(pid)
		if name == "" {
			name = "?"
		}
		if isLauncher(name) {
			name += " (launcher)"
		}
		out = append(out, name+"["+strconv.Itoa(pid)+"]")
		pid = readPPID(pid)
	}
	return out
}

// dedupe returns names without duplicates or empties, preserving order.
func dedupe(names []string) []string {
	var out []string
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func orUnset(s string) string {
	if s == "" {
		return "unset"
	}
	return s
}
