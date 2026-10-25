package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func makeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL=a@example.com",
			"GIT_COMMITTER_NAME=Ada", "GIT_COMMITTER_EMAIL=a@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	for i, msg := range []string{"feat(core): add parser", "fix: handle empty file"} {
		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(strings.Repeat("x\n", i+1)), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", ".")
		git("commit", "-q", "-m", msg)
	}
	return dir
}

func TestRunMarkdown(t *testing.T) {
	var out, errb bytes.Buffer
	if err := run([]string{"-repo", makeRepo(t), "-since", "1 year ago"}, noEnv, &out, &errb); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Weekly digest", "### Features", "**core:** add parser", "### Fixes", "2 commits by 1 author"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunJSON(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-repo", makeRepo(t), "-format", "json"}, noEnv, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"category": "Features"`) {
		t.Errorf("json missing category:\n%s", out.String())
	}
}

func TestRunErrors(t *testing.T) {
	repo := makeRepo(t)
	cases := map[string][]string{
		"bad format":    {"-repo", repo, "-format", "xml"},
		"not a repo":    {"-repo", t.TempDir()},
		"llm needs env": {"-repo", repo, "-llm"},
		"unknown flag":  {"-nope"},
	}
	for name, args := range cases {
		if err := run(args, noEnv, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestRunLLMFallsBackWithWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(500) }))
	defer srv.Close()
	env := map[string]string{"GITLOG_LLM_BASE_URL": srv.URL, "GITLOG_LLM_MODEL": "m"}
	var out, errb bytes.Buffer
	err := run([]string{"-repo", makeRepo(t), "-llm"}, func(k string) string { return env[k] }, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "warning: llm summary") || !strings.Contains(out.String(), "2 commits by 1 author") {
		t.Errorf("fallback behaviour wrong:\nstderr=%s\nstdout=%s", errb.String(), out.String())
	}
}
