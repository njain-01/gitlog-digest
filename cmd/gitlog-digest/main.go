// Command gitlog-digest turns git history into a weekly changelog.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/namanjain/gitlog-digest/internal/digest"
	"github.com/namanjain/gitlog-digest/internal/gitlog"
	"github.com/namanjain/gitlog-digest/internal/summarize"
)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "gitlog-digest:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gitlog-digest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "path to the git repository")
	since := fs.String("since", "4 weeks ago", `how far back to read, any value git --since accepts`)
	format := fs.String("format", "md", "output format: md or json")
	useLLM := fs.Bool("llm", false, "write weekly headlines with an OpenAI-compatible endpoint (see README)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *format != "md" && *format != "json" {
		return fmt.Errorf("unknown -format %q (want md or json)", *format)
	}

	commits, err := gitlog.Read(*repo, *since)
	if err != nil {
		return err
	}
	weeks := digest.Build(commits)

	var s summarize.Summarizer = summarize.Heuristic{}
	if *useLLM {
		base := getenv("GITLOG_LLM_BASE_URL")
		model := getenv("GITLOG_LLM_MODEL")
		if base == "" || model == "" {
			return fmt.Errorf("-llm needs GITLOG_LLM_BASE_URL and GITLOG_LLM_MODEL (GITLOG_LLM_API_KEY is optional)")
		}
		s = summarize.WithFallback{
			Primary: summarize.OpenAI{BaseURL: base, APIKey: getenv("GITLOG_LLM_API_KEY"), Model: model},
			Warn:    func(f string, a ...any) { fmt.Fprintf(stderr, "warning: "+f+"\n", a...) },
		}
	}
	summarize.Apply(context.Background(), s, weeks)

	if *format == "json" {
		return digest.RenderJSON(stdout, weeks)
	}
	return digest.RenderMarkdown(stdout, weeks)
}
