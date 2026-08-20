//go:build linux

package source

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// selfCgroupPath prefers cgroup v2, then the cgroup v1 systemd hierarchy.
func selfCgroupPath() string {
	data, err := os.ReadFile(filepath.Join(procRoot, "self", "cgroup"))
	if err != nil {
		return ""
	}
	unified, systemd, first := "", "", ""
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(f) != 3 || f[2] == "" {
			continue
		}
		if f[0] == "0" && f[1] == "" {
			unified = f[2]
		}
		for _, controller := range strings.Split(f[1], ",") {
			if controller == "name=systemd" {
				systemd = f[2]
			}
		}
		if first == "" {
			first = f[2]
		}
	}
	if unified != "" && (unified != "/" || systemd == "") {
		return unified
	}
	if systemd != "" {
		return systemd
	}
	return first
}

// scopeUnit returns the innermost systemd scope or service.
func scopeUnit(cgPath string) string {
	segs, i := scopeSegments(cgPath)
	if i < 0 {
		return ""
	}
	return segs[i]
}

func scopeSegments(cgPath string) ([]string, int) {
	if cgPath == "" {
		return nil, -1
	}
	segs := strings.Split(strings.Trim(cgPath, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if strings.HasSuffix(segs[i], ".scope") || strings.HasSuffix(segs[i], ".service") {
			return segs, i
		}
	}
	return segs, -1
}

// scopeAppNames derives app aliases from a desktop-created systemd unit:
//
//	app-slack-4711.scope                   -> slack
//	app-gnome-slack-4711.scope             -> slack
//	app-flatpak-com.slack.Slack-4711.scope -> com.slack.Slack, Slack
//	app-slack@1.service                    -> slack
//	session-2.scope                        -> (nothing)
func scopeAppNames(unit string) []string {
	s := strings.TrimSuffix(strings.TrimSuffix(unit, ".scope"), ".service")
	if i := strings.IndexByte(s, '@'); i >= 0 { // systemd instance: app-slack@1
		s = s[:i]
	}
	s = unescapeUnit(s)
	// Only app units are named after the launching desktop file.
	if !strings.HasPrefix(s, "app-") {
		return nil
	}
	s = trimPidSuffix(strings.TrimPrefix(s, "app-"))
	if s == "" {
		return nil
	}

	// The launcher that started the app may prefix the desktop id.
	for _, prefix := range []string{"gnome-", "kde-", "flatpak-", "snap-", "uwsm-"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	out := []string{s}
	// Reverse-DNS app ids (flatpak) reduce to their final component.
	for _, c := range append([]string(nil), out...) {
		if i := strings.LastIndexByte(c, '.'); i >= 0 {
			out = append(out, c[i+1:])
		}
	}
	return filterNames(dedupe(out))
}

// filterNames drops entries that cannot be an app name.
func filterNames(names []string) []string {
	var out []string
	for _, n := range names {
		if isAllDigits(n) || isLauncher(n) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// trimPidSuffix removes the trailing "-<launcher pid>" from a scope name.
func trimPidSuffix(s string) string {
	i := strings.LastIndexByte(s, '-')
	if i < 0 || i == len(s)-1 {
		return s
	}
	if isAllDigits(s[i+1:]) {
		return s[:i]
	}
	return s
}

// unescapeUnit reverses systemd's unit-name escaping (`\x2d` -> "-").
func unescapeUnit(s string) string {
	if !strings.Contains(s, `\x`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+3 < len(s) && s[i+1] == 'x' {
			if v, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 4
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
