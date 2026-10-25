// Package summarize produces a one-paragraph headline for a week of commits.
package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/namanjain/gitlog-digest/internal/classify"
	"github.com/namanjain/gitlog-digest/internal/digest"
)

// Summarizer turns a week of entries into a short human-readable headline.
type Summarizer interface {
	Summarize(ctx context.Context, w digest.Week) (string, error)
}

// Heuristic is a deterministic, offline summarizer.
type Heuristic struct{}

// Summarize lists the busiest categories and highlights the first feature or fix.
func (Heuristic) Summarize(_ context.Context, w digest.Week) (string, error) {
	counts := w.Counts()
	var parts []string
	for _, cat := range classify.Order {
		if n := counts[cat]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, strings.ToLower(string(cat))))
		}
	}
	if len(parts) == 0 {
		return "", errors.New("week has no entries")
	}
	s := fmt.Sprintf("%d commits by %d %s: %s.", w.Commits, w.Authors, plural(w.Authors, "author", "authors"), strings.Join(parts, ", "))
	for _, e := range w.Entries {
		if e.Breaking {
			return s + " Includes a breaking change: " + e.Title + ".", nil
		}
	}
	for _, cat := range []classify.Category{classify.Features, classify.Fixes} {
		for _, e := range w.Entries {
			if e.Category == cat {
				return s + " Highlight: " + e.Title + ".", nil
			}
		}
	}
	return s, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// OpenAI calls an OpenAI-compatible /chat/completions endpoint (OpenAI, Ollama,
// llama.cpp server, vLLM, ...).
type OpenAI struct {
	BaseURL string // e.g. http://localhost:11434/v1
	APIKey  string // optional for local servers
	Model   string
	Client  *http.Client
}

// Summarize asks the model for a two-sentence headline.
func (o OpenAI) Summarize(ctx context.Context, w digest.Week) (string, error) {
	var list strings.Builder
	for _, e := range w.Entries {
		fmt.Fprintf(&list, "- [%s] %s\n", e.Category, e.Title)
	}
	body, err := json.Marshal(map[string]any{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You write terse weekly changelog headlines. Reply with at most two plain sentences."},
			{"role": "user", "content": fmt.Sprintf("Summarize week %s:\n%s", w.Label, list.String())},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm endpoint returned %s", resp.Status)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("llm returned no content")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// WithFallback tries primary and falls back to Heuristic, reporting failures
// through warn so a flaky endpoint never breaks a report.
type WithFallback struct {
	Primary Summarizer
	Warn    func(format string, args ...any)
}

// Summarize implements Summarizer.
func (f WithFallback) Summarize(ctx context.Context, w digest.Week) (string, error) {
	s, err := f.Primary.Summarize(ctx, w)
	if err == nil {
		return s, nil
	}
	if f.Warn != nil {
		f.Warn("llm summary for %s failed (%v); using heuristic", w.Label, err)
	}
	return Heuristic{}.Summarize(ctx, w)
}

// Apply fills in the Summary of every week using s. Weeks that fail are left blank.
func Apply(ctx context.Context, s Summarizer, weeks []digest.Week) {
	for i := range weeks {
		if text, err := s.Summarize(ctx, weeks[i]); err == nil {
			weeks[i].Summary = text
		}
	}
}
