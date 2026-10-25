package summarize

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/namanjain/gitlog-digest/internal/classify"
	"github.com/namanjain/gitlog-digest/internal/digest"
)

func week() digest.Week {
	return digest.Week{Label: "2026-W43", Commits: 3, Authors: 2, Entries: []digest.Entry{
		{Category: classify.Fixes, Title: "crash on empty input"},
		{Category: classify.Features, Title: "add search"},
		{Category: classify.Docs, Title: "readme"},
	}}
}

func TestHeuristic(t *testing.T) {
	got, err := Heuristic{}.Summarize(context.Background(), week())
	if err != nil {
		t.Fatal(err)
	}
	want := "3 commits by 2 authors: 1 features, 1 fixes, 1 documentation. Highlight: add search."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHeuristicBreakingAndEmpty(t *testing.T) {
	w := week()
	w.Entries[0].Breaking = true
	got, _ := Heuristic{}.Summarize(context.Background(), w)
	if !strings.Contains(got, "breaking change: crash on empty input") {
		t.Errorf("breaking not highlighted: %q", got)
	}
	if _, err := (Heuristic{}).Summarize(context.Background(), digest.Week{}); err == nil {
		t.Error("expected error for empty week")
	}
}

func TestOpenAISuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("bad request: %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct{ Content string }
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "m" || !strings.Contains(req.Messages[1].Content, "add search") {
			t.Errorf("unexpected payload: %+v", req)
		}
		_, _ = rw.Write([]byte(`{"choices":[{"message":{"content":"  Search landed.  "}}]}`))
	}))
	defer srv.Close()
	got, err := OpenAI{BaseURL: srv.URL + "/v1/", APIKey: "k", Model: "m"}.Summarize(context.Background(), week())
	if err != nil || got != "Search landed." {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestOpenAIFailures(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"http error": func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(500) },
		"bad json":   func(rw http.ResponseWriter, _ *http.Request) { _, _ = rw.Write([]byte("nope")) },
		"no choices": func(rw http.ResponseWriter, _ *http.Request) { _, _ = rw.Write([]byte(`{"choices":[]}`)) },
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			if _, err := (OpenAI{BaseURL: srv.URL, Model: "m"}).Summarize(context.Background(), week()); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestFallbackAndApply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(503) }))
	defer srv.Close()
	var warned []string
	s := WithFallback{
		Primary: OpenAI{BaseURL: srv.URL, Model: "m"},
		Warn:    func(f string, a ...any) { warned = append(warned, f) },
	}
	weeks := []digest.Week{week()}
	Apply(context.Background(), s, weeks)
	if !strings.HasPrefix(weeks[0].Summary, "3 commits by 2 authors") {
		t.Errorf("fallback not used: %q", weeks[0].Summary)
	}
	if len(warned) != 1 {
		t.Errorf("expected one warning, got %d", len(warned))
	}
}
