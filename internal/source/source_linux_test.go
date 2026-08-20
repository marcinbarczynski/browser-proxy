//go:build linux

package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParsePPIDFromStat(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{
			name: "init line",
			in:   "1 (systemd) S 0 1 1 0 -1 4194560 ...",
			want: 0,
		},
		{
			name: "normal child",
			in:   "1234 (slack) R 1000 1234 1000 ...",
			want: 1000,
		},
		{
			name: "comm with spaces",
			in:   "5678 (Web Content) S 4321 5678 4321 ...",
			want: 4321,
		},
		{
			name: "comm with parens inside",
			in:   "9999 (some (nested) name) S 100 9999 100 ...",
			want: 100,
		},
		{
			name: "malformed empty",
			in:   "",
			want: 0,
		},
		{
			name: "malformed no paren",
			in:   "1234 nope nothing",
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePPIDFromStat(tc.in); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestIsLauncher(t *testing.T) {
	mustBe := []string{"xdg-open", "gio", "gio-launch-deskto", "browser-proxy", "bash"}
	mustNot := []string{"slack", "Slack", "firefox", "code", "thunderbird"}
	for _, n := range mustBe {
		if !isLauncher(n) {
			t.Errorf("%q should be a launcher", n)
		}
	}
	for _, n := range mustNot {
		if isLauncher(n) {
			t.Errorf("%q must NOT be classified as launcher", n)
		}
	}
}

// --- fixture ---------------------------------------------------------------

// fixture builds a fake /proc tree so the whole staged Detect() chain can be
// exercised without reading the real system.
type fixture struct {
	t    *testing.T
	proc string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{t: t, proc: filepath.Join(root, "proc")}
	f.mkdir(f.proc)

	oldProc, oldPPID := procRoot, getppid
	procRoot = f.proc
	getppid = func() int { return 0 } // no ancestry unless a test adds one
	t.Cleanup(func() {
		procRoot, getppid = oldProc, oldPPID
	})
	return f
}

func (f *fixture) mkdir(p string) {
	f.t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) write(p, content string) {
	f.t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// addProc registers a process. exe == "" omits the symlink, as for a process
// whose image we are not allowed to read.
func (f *fixture) addProc(pid int, comm, exe string, ppid int) {
	f.t.Helper()
	dir := filepath.Join(f.proc, strconv.Itoa(pid))
	f.mkdir(dir)
	f.write(filepath.Join(dir, "comm"), comm+"\n")
	f.write(filepath.Join(dir, "stat"), fmt.Sprintf("%d (%s) S %d 1 1 0 -1 0", pid, comm, ppid))
	if exe != "" {
		if err := os.Symlink(exe, filepath.Join(dir, "exe")); err != nil {
			f.t.Fatal(err)
		}
	}
}

// setParent makes pid our parent process.
func (f *fixture) setParent(pid int) { getppid = func() int { return pid } }

func (f *fixture) setSelfCgroup(lines ...string) {
	f.t.Helper()
	dir := filepath.Join(f.proc, "self")
	f.mkdir(dir)
	f.write(filepath.Join(dir, "cgroup"), strings.Join(lines, "\n")+"\n")
}

const slackScope = "user.slice/user@1000.service/app.slice/app-slack-4711.scope"

// --- staged detection ------------------------------------------------------

func TestDetectAncestorStage(t *testing.T) {
	f := newFixture(t)
	f.addProc(200, "xdg-open", "/usr/bin/xdg-open", 300)
	f.addProc(300, "slack", "/usr/lib/slack/slack", 1)
	f.setParent(200)

	got := Detect()
	if got.Name != "slack" || got.Via != viaAncestor {
		t.Fatalf("got %+v, want slack via ancestor", got)
	}
	if !hasCandidate(got, "slack") {
		t.Errorf("candidates %v missing slack", got.Candidates)
	}
}

// comm is truncated to 15 chars by the kernel, so the exe basename has to be a
// candidate too — that's the only form long binary names survive in.
func TestDetectAncestorAddsExeCandidate(t *testing.T) {
	f := newFixture(t)
	f.addProc(200, "teams-for-linux", "/opt/teams-for-linux/teams-for-linux-bin", 1)
	f.setParent(200)

	got := Detect()
	if got.Name != "teams-for-linux" {
		t.Fatalf("Name = %q, want teams-for-linux", got.Name)
	}
	if !hasCandidate(got, "teams-for-linux-bin") {
		t.Errorf("candidates %v missing the exe basename", got.Candidates)
	}
	if got.Candidates[0] != got.Name {
		t.Errorf("invariant broken: Candidates[0]=%q Name=%q", got.Candidates[0], got.Name)
	}
}

// The case the whole change exists for: gio double-forks the handler, so only
// the app scope name still identifies the caller.
func TestDetectScopeStage(t *testing.T) {
	f := newFixture(t)
	f.setSelfCgroup("0::/" + slackScope)

	got := Detect()
	if got.Via != viaScope {
		t.Fatalf("Via = %q, want scope (got %+v)", got.Via, got)
	}
	if want := []string{"slack"}; !equal(got.Candidates, want) {
		t.Errorf("candidates = %v, want %v", got.Candidates, want)
	}
	if got.Name != "slack" {
		t.Errorf("Name = %q, want slack", got.Name)
	}
}

func TestDetectNothing(t *testing.T) {
	f := newFixture(t)
	f.setSelfCgroup("0::/user.slice/user@1000.service/session.scope")

	if got := Detect(); !got.Empty() || got.Via != "" {
		t.Fatalf("got %+v, want empty Info", got)
	}
}

// A shell's own scope must not be reported as the source app.
func TestDetectIgnoresLauncherScopeName(t *testing.T) {
	f := newFixture(t)
	f.setSelfCgroup("0::/user.slice/user@1000.service/app.slice/app-bash-123.scope")

	if got := Detect(); !got.Empty() {
		t.Fatalf("got %+v, want empty Info", got)
	}
}

func TestDetectMissingProcIsNotFatal(t *testing.T) {
	newFixture(t) // empty tree: no /proc/self/cgroup at all
	if got := Detect(); !got.Empty() {
		t.Fatalf("got %+v, want empty Info", got)
	}
}

func TestReadExeBase(t *testing.T) {
	f := newFixture(t)
	f.addProc(10, "slack", "/usr/lib/slack/slack", 1)
	f.addProc(11, "upgraded", "/usr/bin/upgraded (deleted)", 1)
	f.addProc(12, "noexe", "", 1)

	cases := map[int]string{10: "slack", 11: "upgraded", 12: ""}
	for pid, want := range cases {
		if got := readExeBase(pid); got != want {
			t.Errorf("readExeBase(%d) = %q, want %q", pid, got, want)
		}
	}
}

func hasCandidate(i Info, want string) bool {
	for _, c := range i.Candidates {
		if c == want {
			return true
		}
	}
	return false
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
