// Package logging writes URL-routing events to a per-user log file.
// The Logger is opt-in (controlled from the TOML config) and best-effort:
// any I/O failure becomes a no-op so that routing itself is never blocked.
//
// File creation is LAZY — the log file is only opened on the first write.
// Dry-run commands like `browser-proxy test` therefore never touch the disk.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Logger writes append-only log lines. A nil receiver and a disabled Logger
// both silently drop everything.
type Logger struct {
	enabled bool
	path    string

	// populated on first write
	out  io.Writer
	file *os.File

	// sticky failure flag — once an open attempt fails, stay silent
	failed bool

	now func() time.Time // injectable for tests
}

// New returns a Logger. When enabled is false, every method is a no-op.
// path may be empty (use DefaultPath) and supports a leading "~/".
// The file is NOT opened here — it's opened lazily on the first log call.
func New(enabled bool, path string) *Logger {
	if !enabled {
		return &Logger{out: io.Discard, now: time.Now}
	}
	if path == "" {
		path = DefaultPath()
	}
	return &Logger{enabled: true, path: expandHome(path), now: time.Now}
}

// NewWithWriter wraps a writer for tests. Close is a no-op.
func NewWithWriter(w io.Writer) *Logger {
	return &Logger{out: w, now: time.Now}
}

// Close releases the underlying file handle if one was opened. Safe on nil.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	l.out = io.Discard
	return err
}

// writer returns the destination, opening the file lazily if needed.
// On open failure, falls back to io.Discard with a stderr warning (once).
func (l *Logger) writer() io.Writer {
	if l.out != nil {
		return l.out
	}
	if !l.enabled || l.failed {
		l.out = io.Discard
		return l.out
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		l.failOpen(err)
		return l.out
	}
	// 0o600: URL history is sensitive. O_APPEND for atomic writes <PIPE_BUF.
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		l.failOpen(err)
		return l.out
	}
	l.file = f
	l.out = f
	return l.out
}

func (l *Logger) failOpen(err error) {
	l.failed = true
	l.out = io.Discard
	fmt.Fprintf(os.Stderr, "warning: logging disabled: %v\n", err)
}

// Routed records a routing decision.
//
//	2026-05-05 14:23:45  routed   https://github.com/foo  →  Firefox  (rule 0: prefix=https://github.com/)  source=Slack
func (l *Logger) Routed(url, browser, profile, ruleDesc, srcLabel string, ruleIndex int) {
	if l == nil {
		return
	}
	target := browser
	if profile != "" {
		target = browser + " [profile=" + profile + "]"
	}
	where := "default"
	if ruleIndex >= 0 {
		where = fmt.Sprintf("rule %d: %s", ruleIndex, ruleDesc)
	}
	line := fmt.Sprintf("%s  routed   %s  →  %s  (%s)", l.timestamp(), url, target, where)
	if srcLabel != "" {
		line += "  source=" + srcLabel
	}
	fmt.Fprintln(l.writer(), line)
}

// Rewritten records a URL transformation.
//
//	2026-05-05 14:23:45  rewrite  http://github.com/foo  →  https://github.com/foo
func (l *Logger) Rewritten(from, to string) {
	if l == nil {
		return
	}
	fmt.Fprintf(l.writer(), "%s  rewrite  %s  →  %s\n", l.timestamp(), from, to)
}

// Error records a failure with formatted args.
//
//	2026-05-05 14:23:45  error    open "Firefox": exit status 1
func (l *Logger) Error(format string, args ...any) {
	if l == nil {
		return
	}
	w := l.writer()
	fmt.Fprintf(w, "%s  error    ", l.timestamp())
	fmt.Fprintf(w, format, args...)
	fmt.Fprintln(w)
}

func (l *Logger) timestamp() string {
	return l.now().Format("2006-01-02 15:04:05")
}

// DefaultPath returns the platform-conventional log path.
//
//	macOS: ~/Library/Logs/browser-proxy.log
//	Linux: $XDG_STATE_HOME/browser-proxy/browser-proxy.log
//	       (default ~/.local/state/browser-proxy/browser-proxy.log)
func DefaultPath() string {
	h, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(h, "Library", "Logs", "browser-proxy.log")
	}
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "browser-proxy", "browser-proxy.log")
	}
	return filepath.Join(h, ".local", "state", "browser-proxy", "browser-proxy.log")
}

func expandHome(p string) string {
	if p == "~" {
		h, _ := os.UserHomeDir()
		return h
	}
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, p[2:])
	}
	return p
}
