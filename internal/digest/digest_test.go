package digest

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/namanjain/gitlog-digest/internal/classify"
	"github.com/namanjain/gitlog-digest/internal/gitlog"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func sample() []gitlog.Commit {
	return []gitlog.Commit{
		{Hash: "aaaaaaaa1", Author: "Ada", When: at("2026-10-25T10:00:00Z"), Subject: "feat(cli)!: new flags", Additions: 10, Deletions: 1}, // Sunday, W43
		{Hash: "bbbbbbbb2", Author: "Bob", When: at("2026-10-19T00:00:00Z"), Subject: "fix: crash", Additions: 2, Deletions: 2},             // Monday, W43
		{Hash: "cccccccc3", Author: "Ada", When: at("2026-10-18T23:59:59Z"), Subject: "docs: readme", Additions: 5},                         // Sunday, W42
	}
}

func TestBuildGroupsByISOWeek(t *testing.T) {
	weeks := Build(sample())
	if len(weeks) != 2 {
		t.Fatalf("got %d weeks, want 2", len(weeks))
	}
	w := weeks[0]
	if w.Label != "2026-W43" || w.Start != "2026-10-19" || w.End != "2026-10-25" {
		t.Errorf("week 0 header wrong: %+v", w)
	}
	if w.Commits != 2 || w.Authors != 2 || w.Additions != 12 || w.Deletions != 3 {
		t.Errorf("week 0 stats wrong: %+v", w)
	}
	if weeks[1].Label != "2026-W42" || weeks[1].Commits != 1 {
		t.Errorf("week 1 wrong: %+v", weeks[1])
	}
	if w.Entries[0].Hash != "aaaaaaa" || !w.Entries[0].Breaking {
		t.Errorf("entry wrong: %+v", w.Entries[0])
	}
}

func TestBuildOrdersNewestWeekFirstRegardlessOfInput(t *testing.T) {
	cs := sample()
	cs[0], cs[2] = cs[2], cs[0]
	weeks := Build(cs)
	if weeks[0].Label != "2026-W43" {
		t.Errorf("newest week should be first, got %s", weeks[0].Label)
	}
}

func TestISOWeekAcrossYearBoundary(t *testing.T) {
	weeks := Build([]gitlog.Commit{{Hash: "d", Author: "A", When: at("2027-01-01T12:00:00Z"), Subject: "x"}})
	if weeks[0].Label != "2026-W53" || weeks[0].Start != "2026-12-28" {
		t.Errorf("year boundary wrong: %+v", weeks[0])
	}
}

func TestRenderMarkdown(t *testing.T) {
	weeks := Build(sample())
	weeks[0].Summary = "A busy week."
	var buf bytes.Buffer
	if err := RenderMarkdown(&buf, weeks); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"## 2026-W43 (2026-10-19 to 2026-10-25)",
		"A busy week.",
		"_2 commits, 2 authors, +12/-3 lines_",
		"### Features",
		"- **BREAKING** **cli:** new flags (`aaaaaaa`, Ada)",
		"### Fixes",
		"### Documentation",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q\n%s", want, out)
		}
	}
	if strings.Index(out, "### Features") > strings.Index(out, "### Fixes") {
		t.Error("categories out of order")
	}
}

func TestRenderEmpty(t *testing.T) {
	var md, js bytes.Buffer
	if err := RenderMarkdown(&md, nil); err != nil || !strings.Contains(md.String(), "No commits") {
		t.Errorf("empty markdown wrong: %v %q", err, md.String())
	}
	if err := RenderJSON(&js, nil); err != nil || strings.TrimSpace(js.String()) != "[]" {
		t.Errorf("empty json wrong: %v %q", err, js.String())
	}
}

func TestRenderJSONRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderJSON(&buf, Build(sample())); err != nil {
		t.Fatal(err)
	}
	var back []Week
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 2 || back[0].Counts()[classify.Features] != 1 {
		t.Errorf("round trip wrong: %+v", back)
	}
}
