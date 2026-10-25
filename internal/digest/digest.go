// Package digest groups classified commits by ISO week and renders them.
package digest

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/namanjain/gitlog-digest/internal/classify"
	"github.com/namanjain/gitlog-digest/internal/gitlog"
)

// Entry is a classified commit ready for display.
type Entry struct {
	Hash     string            `json:"hash"`
	Author   string            `json:"author"`
	Category classify.Category `json:"category"`
	Scope    string            `json:"scope,omitempty"`
	Title    string            `json:"title"`
	Breaking bool              `json:"breaking,omitempty"`
}

// Week is all activity for one ISO week (Monday start, UTC).
type Week struct {
	Label     string  `json:"week"` // e.g. 2026-W43
	Start     string  `json:"start"`
	End       string  `json:"end"`
	Commits   int     `json:"commits"`
	Authors   int     `json:"authors"`
	Additions int     `json:"additions"`
	Deletions int     `json:"deletions"`
	Summary   string  `json:"summary,omitempty"`
	Entries   []Entry `json:"entries"`
}

// Build classifies commits and groups them by ISO week, newest week first.
// Within a week, entries keep input order (git log emits newest first).
func Build(commits []gitlog.Commit) []Week {
	type acc struct {
		week    Week
		authors map[string]bool
		monday  time.Time
	}
	byLabel := map[string]*acc{}
	var order []*acc
	for _, c := range commits {
		year, wk := c.When.ISOWeek()
		label := fmt.Sprintf("%d-W%02d", year, wk)
		a, ok := byLabel[label]
		if !ok {
			monday := time.Date(c.When.Year(), c.When.Month(), c.When.Day(), 0, 0, 0, 0, time.UTC).
				AddDate(0, 0, -((int(c.When.Weekday()) + 6) % 7))
			a = &acc{authors: map[string]bool{}, monday: monday, week: Week{
				Label: label,
				Start: monday.Format("2006-01-02"),
				End:   monday.AddDate(0, 0, 6).Format("2006-01-02"),
			}}
			byLabel[label] = a
			order = append(order, a)
		}
		r := classify.Commit(c)
		short := c.Hash
		if len(short) > 7 {
			short = short[:7]
		}
		a.week.Entries = append(a.week.Entries, Entry{Hash: short, Author: c.Author,
			Category: r.Category, Scope: r.Scope, Title: r.Title, Breaking: r.Breaking})
		a.week.Commits++
		a.week.Additions += c.Additions
		a.week.Deletions += c.Deletions
		a.authors[c.Author] = true
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].monday.After(order[j].monday) })
	weeks := make([]Week, 0, len(order))
	for _, a := range order {
		a.week.Authors = len(a.authors)
		weeks = append(weeks, a.week)
	}
	return weeks
}

// Counts returns the number of entries per category in w.
func (w Week) Counts() map[classify.Category]int {
	m := map[classify.Category]int{}
	for _, e := range w.Entries {
		m[e.Category]++
	}
	return m
}

// RenderMarkdown writes weeks as a Markdown changelog.
func RenderMarkdown(out io.Writer, weeks []Week) error {
	var b strings.Builder
	b.WriteString("# Weekly digest\n")
	if len(weeks) == 0 {
		b.WriteString("\nNo commits in the selected range.\n")
	}
	for _, w := range weeks {
		fmt.Fprintf(&b, "\n## %s (%s to %s)\n\n", w.Label, w.Start, w.End)
		if w.Summary != "" {
			b.WriteString(w.Summary + "\n\n")
		}
		fmt.Fprintf(&b, "_%d commits, %d authors, +%d/-%d lines_\n", w.Commits, w.Authors, w.Additions, w.Deletions)
		for _, cat := range classify.Order {
			var lines []string
			for _, e := range w.Entries {
				if e.Category != cat {
					continue
				}
				line := "- "
				if e.Breaking {
					line += "**BREAKING** "
				}
				if e.Scope != "" {
					line += "**" + e.Scope + ":** "
				}
				line += fmt.Sprintf("%s (`%s`, %s)", e.Title, e.Hash, e.Author)
				lines = append(lines, line)
			}
			if len(lines) > 0 {
				fmt.Fprintf(&b, "\n### %s\n\n%s\n", cat, strings.Join(lines, "\n"))
			}
		}
	}
	_, err := io.WriteString(out, b.String())
	return err
}

// RenderJSON writes weeks as indented JSON.
func RenderJSON(out io.Writer, weeks []Week) error {
	if weeks == nil {
		weeks = []Week{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(weeks)
}
