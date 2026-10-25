// Package gitlog reads commit history from a local git repository.
package gitlog

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Commit is one non-merge commit with its change statistics.
type Commit struct {
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	When      time.Time `json:"when"`
	Subject   string    `json:"subject"`
	Files     []string  `json:"files"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
}

const (
	recordSep = "\x1e"
	unitSep   = "\x1f"
	// Format emits: RS hash US author US ISO-date US subject, then numstat lines.
	logFormat = "--pretty=format:%x1e%H%x1f%an%x1f%aI%x1f%s"
)

// Read runs git log in repo for commits newer than since (any value git's
// --since accepts, e.g. "4 weeks ago" or "2026-01-01") and parses the result.
func Read(repo, since string) ([]Commit, error) {
	args := []string{"-C", repo, "log", "--no-merges", "--numstat", logFormat}
	if since != "" {
		args = append(args, "--since="+since)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return Parse(stdout.String())
}

// Parse converts the output of git log in the package's format into commits.
func Parse(out string) ([]Commit, error) {
	var commits []Commit
	for _, rec := range strings.Split(out, recordSep) {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		lines := strings.Split(rec, "\n")
		fields := strings.Split(lines[0], unitSep)
		if len(fields) != 4 {
			return nil, fmt.Errorf("malformed commit header %q", lines[0])
		}
		when, err := time.Parse(time.RFC3339, fields[2])
		if err != nil {
			return nil, fmt.Errorf("bad commit date %q: %w", fields[2], err)
		}
		c := Commit{Hash: fields[0], Author: fields[1], When: when.UTC(), Subject: fields[3]}
		for _, l := range lines[1:] {
			if strings.TrimSpace(l) == "" {
				continue
			}
			parts := strings.SplitN(l, "\t", 3)
			if len(parts) != 3 {
				return nil, fmt.Errorf("malformed numstat line %q", l)
			}
			// Binary files report "-" for both counts.
			if n, err := strconv.Atoi(parts[0]); err == nil {
				c.Additions += n
			}
			if n, err := strconv.Atoi(parts[1]); err == nil {
				c.Deletions += n
			}
			c.Files = append(c.Files, parts[2])
		}
		commits = append(commits, c)
	}
	return commits, nil
}
