// Package classify assigns a change category and scope to a commit.
package classify

import (
	"path"
	"regexp"
	"strings"

	"github.com/namanjain/gitlog-digest/internal/gitlog"
)

// Category is the kind of change a commit represents.
type Category string

const (
	Features    Category = "Features"
	Fixes       Category = "Fixes"
	Performance Category = "Performance"
	Refactoring Category = "Refactoring"
	Docs        Category = "Documentation"
	Tests       Category = "Tests"
	Build       Category = "Build & CI"
	Chores      Category = "Chores"
	Other       Category = "Other"
)

// Order lists categories in the order they are rendered.
var Order = []Category{Features, Fixes, Performance, Refactoring, Docs, Tests, Build, Chores, Other}

// Result is the classification of a single commit.
type Result struct {
	Category Category
	Scope    string
	Title    string // subject with any conventional-commit prefix removed
	Breaking bool
}

var conventional = regexp.MustCompile(`^(\w+)(?:\(([^)]*)\))?(!)?:\s*(.+)$`)

var prefixes = map[string]Category{
	"feat": Features, "feature": Features,
	"fix": Fixes, "bugfix": Fixes, "hotfix": Fixes,
	"perf":     Performance,
	"refactor": Refactoring,
	"docs":     Docs, "doc": Docs,
	"test": Tests, "tests": Tests,
	"build": Build, "ci": Build,
	"chore": Chores, "style": Chores, "revert": Chores,
}

var keywords = []struct {
	words []string
	cat   Category
}{
	{[]string{"fix", "fixes", "fixed", "resolve", "resolves", "patch"}, Fixes},
	{[]string{"add", "adds", "added", "implement", "introduce", "support", "create"}, Features},
	{[]string{"speed", "optimize", "optimise", "faster", "cache"}, Performance},
	{[]string{"refactor", "restructure", "simplify", "rename", "extract", "cleanup", "clean"}, Refactoring},
	{[]string{"document", "docs", "readme"}, Docs},
	{[]string{"test", "tests"}, Tests},
	{[]string{"bump", "upgrade", "update", "ci", "build", "release"}, Build},
}

// Commit classifies c. Precedence: conventional-commit prefix, then the first
// word of the subject, then the files touched.
func Commit(c gitlog.Commit) Result {
	subject := strings.TrimSpace(c.Subject)
	if m := conventional.FindStringSubmatch(subject); m != nil {
		if cat, ok := prefixes[strings.ToLower(m[1])]; ok {
			return Result{Category: cat, Scope: m[2], Title: m[4], Breaking: m[3] == "!"}
		}
	}
	first := strings.ToLower(strings.Fields(subject + " x")[0])
	first = strings.Trim(first, ":.,")
	for _, k := range keywords {
		for _, w := range k.words {
			if first == w {
				return Result{Category: k.cat, Title: subject}
			}
		}
	}
	return Result{Category: fromFiles(c.Files), Title: subject}
}

// fromFiles infers a category when every changed file belongs to one kind.
func fromFiles(files []string) Category {
	if len(files) == 0 {
		return Other
	}
	var cat Category
	for _, f := range files {
		var k Category
		base := strings.ToLower(path.Base(f))
		switch {
		case strings.HasSuffix(base, "_test.go"), strings.Contains(base, ".test."), strings.Contains(base, ".spec."):
			k = Tests
		case strings.HasSuffix(base, ".md"), strings.HasPrefix(f, "docs/"):
			k = Docs
		case strings.HasPrefix(f, ".github/"), base == "makefile", base == "dockerfile", base == "go.mod", base == "go.sum":
			k = Build
		default:
			return Other
		}
		if cat != "" && cat != k {
			return Other
		}
		cat = k
	}
	return cat
}
