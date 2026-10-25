package gitlog

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	out := "\x1eabc123\x1fAda\x1f2026-10-21T10:00:00+02:00\x1ffeat(api): add search\n\n10\t2\tapi/search.go\n-\t-\tlogo.png\n" +
		"\x1edef456\x1fBob\x1f2026-10-20T09:00:00Z\x1fWIP\n"
	cs, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("got %d commits, want 2", len(cs))
	}
	c := cs[0]
	if c.Additions != 10 || c.Deletions != 2 || len(c.Files) != 2 {
		t.Errorf("stats wrong: %+v", c)
	}
	if c.When.Hour() != 8 { // converted to UTC
		t.Errorf("expected UTC conversion, got %v", c.When)
	}
	if len(cs[1].Files) != 0 || cs[1].Subject != "WIP" {
		t.Errorf("second commit wrong: %+v", cs[1])
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse("\x1eonlyhash"); err == nil {
		t.Error("expected error for malformed header")
	}
	if _, err := Parse("\x1eh\x1fa\x1fnot-a-date\x1fs"); err == nil {
		t.Error("expected error for bad date")
	}
}

func TestReadRealRepo(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "feat: first")
	cs, err := Read(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Subject != "feat: first" || cs[0].Additions != 2 {
		t.Fatalf("unexpected commits: %+v", cs)
	}
	if _, err := Read(filepath.Join(dir, "missing"), ""); err == nil {
		t.Error("expected error for missing repo")
	}
}
