package logging

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedClock(t *testing.T) func() time.Time {
	t.Helper()
	frozen := time.Date(2026, 5, 5, 14, 23, 45, 0, time.UTC)
	return func() time.Time { return frozen }
}

func TestNilLoggerIsSafe(t *testing.T) {
	var l *Logger
	// must not panic
	l.Routed("u", "b", "", "", "", -1)
	l.Rewritten("a", "b")
	l.Error("x: %v", errors.New("y"))
	if err := l.Close(); err != nil {
		t.Errorf("Close on nil: %v", err)
	}
}

func TestDisabledLoggerWritesNothing(t *testing.T) {
	l := New(false, "")
	defer l.Close()
	if l.out != io.Discard {
		t.Errorf("disabled logger should write to io.Discard, got %T", l.out)
	}
	// Calling methods must not panic and must not change the writer.
	l.Routed("u", "b", "", "", "", -1)
	if l.out != io.Discard {
		t.Errorf("disabled logger writer changed after method call: %T", l.out)
	}
}

func TestLazyFileCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.log")

	l := New(true, path)
	defer l.Close()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected log file NOT to exist before first write, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("expected log dir NOT to exist before first write, stat err=%v", err)
	}

	l.Routed("u", "b", "", "", "", -1)

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected log file to exist after first write, got err=%v", err)
	}
}

func TestRoutedFormat(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf)
	l.now = fixedClock(t)

	l.Routed("https://github.com/foo", "Firefox", "Work", "prefix=https://github.com/", "Slack", 0)

	got := buf.String()
	for _, want := range []string{
		"2026-05-05 14:23:45",
		"routed",
		"https://github.com/foo",
		"Firefox [profile=Work]",
		"rule 0: prefix=https://github.com/",
		"source=Slack",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull: %s", want, got)
		}
	}
}

func TestRoutedWithoutProfile(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf)
	l.now = fixedClock(t)

	l.Routed("https://x.com/", "Chrome", "", "host=*.x.com", "", 2)
	got := buf.String()

	if !strings.Contains(got, "Chrome  ") || strings.Contains(got, "[profile=") {
		t.Errorf("expected bare 'Chrome' without profile suffix, got %q", got)
	}
	if strings.Contains(got, "source=") {
		t.Errorf("no source means no source= field, got %q", got)
	}
}

func TestRoutedDefault(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf)
	l.now = fixedClock(t)

	l.Routed("https://x.com/", "Safari", "", "", "", -1)
	if !strings.Contains(buf.String(), "(default)") {
		t.Errorf("expected '(default)' marker, got %q", buf.String())
	}
}

func TestRewritten(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf)
	l.now = fixedClock(t)

	l.Rewritten("http://x.com/", "https://x.com/")
	got := buf.String()
	if !strings.HasPrefix(got, "2026-05-05 14:23:45  rewrite  http://x.com/  →  https://x.com/\n") {
		t.Errorf("unexpected rewrite line: %q", got)
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf)
	l.now = fixedClock(t)

	l.Error("open %q: %v", "Firefox", errors.New("boom"))
	got := buf.String()
	if !strings.Contains(got, `open "Firefox": boom`) {
		t.Errorf("expected formatted error message, got %q", got)
	}
}

func TestFileModeIs0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(true, path)
	l.Routed("u", "b", "", "", "", -1)
	l.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("expected mode 0o600, got %o", mode)
	}
}

func TestAppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte("preexisting\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	l := New(true, path)
	l.Rewritten("a", "b")
	l.Close()

	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "preexisting\n") {
		t.Errorf("expected pre-existing line preserved, got %q", data)
	}
	if !strings.Contains(string(data), "rewrite") {
		t.Errorf("expected new rewrite line appended, got %q", data)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		"~":         home,
		"~/foo":     filepath.Join(home, "foo"),
		"~/foo/bar": filepath.Join(home, "foo", "bar"),
		"/abs/path": "/abs/path",
		"relative":  "relative",
		"":          "",
		"~user/foo": "~user/foo", // not supported, passthrough
	}
	for in, want := range cases {
		if got := expandHome(in); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	l := New(true, filepath.Join(dir, "x.log"))
	l.Routed("u", "b", "", "", "", -1) // open file via lazy path
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("second Close errored: %v", err)
	}
	// Methods after Close: writer() returns Discard
	l.Routed("u", "b", "", "", "", -1)
}
