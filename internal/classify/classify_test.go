package classify

import (
	"testing"

	"github.com/namanjain/gitlog-digest/internal/gitlog"
)

func TestCommit(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		files    []string
		want     Category
		scope    string
		title    string
		breaking bool
	}{
		{"conventional feat with scope", "feat(api): add search", nil, Features, "api", "add search", false},
		{"breaking marker", "fix!: drop v1 endpoint", nil, Fixes, "", "drop v1 endpoint", true},
		{"uppercase prefix", "FIX: crash on empty input", nil, Fixes, "", "crash on empty input", false},
		{"unknown prefix falls through to keyword", "wip: add thing", nil, Other, "", "wip: add thing", false},
		{"keyword fix", "Fix nil pointer in parser", nil, Fixes, "", "Fix nil pointer in parser", false},
		{"keyword add", "Add retry to client", nil, Features, "", "Add retry to client", false},
		{"capitalised prefix", "Refactor: split handler", nil, Refactoring, "", "split handler", false},
		{"keyword with trailing punctuation", "Simplify, then ship", nil, Refactoring, "", "Simplify, then ship", false},
		{"files: only tests", "more cases", []string{"a/b_test.go"}, Tests, "", "more cases", false},
		{"files: only docs", "words", []string{"README.md", "docs/x.txt"}, Docs, "", "words", false},
		{"files: mixed kinds", "stuff", []string{"README.md", "main.go"}, Other, "", "stuff", false},
		{"files: tests and docs mixed", "stuff", []string{"a_test.go", "README.md"}, Other, "", "stuff", false},
		{"no signal", "misc", nil, Other, "", "misc", false},
		{"empty subject", "", nil, Other, "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Commit(gitlog.Commit{Subject: tc.subject, Files: tc.files})
			if got.Category != tc.want || got.Scope != tc.scope || got.Title != tc.title || got.Breaking != tc.breaking {
				t.Errorf("got %+v, want cat=%s scope=%q title=%q breaking=%v", got, tc.want, tc.scope, tc.title, tc.breaking)
			}
		})
	}
}
