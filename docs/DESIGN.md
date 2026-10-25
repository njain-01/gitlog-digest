# Design: gitlog-digest

## Problem
Raw `git log` is noisy and chronological. Teams want a weekly changelog grouped by
kind of change (features, fixes, ...) that a human can skim in a minute. Existing
tools mostly require strict Conventional Commits or an LLM API key.

## Approach
A pipeline of four small packages, standard library only:

1. `gitlog` runs `git log` with a machine-friendly format and parses it into `Commit` values (hash, author, time, subject, line stats, files).
2. `classify` assigns each commit a category and optional scope. It trusts Conventional Commit prefixes first, then falls back to subject keywords, then to which files changed.
3. `digest` groups commits by ISO week (UTC) and category and renders Markdown or JSON.
4. `summarize` writes a one-paragraph headline per week. The default heuristic summarizer is deterministic and offline. An optional OpenAI-compatible client can be enabled with `-llm`.

## Key decisions
- **Heuristics first, LLM optional.** The tool must be useful and testable with no network or keys. The LLM only rewrites the weekly headline; classification stays deterministic so output is reproducible and auditable.
- **Standard library only.** No dependency to vet, easy `go run`, trivial CI.
- **Parse via separators, not regexes on pretty output.** ASCII record/unit separators (0x1e/0x1f) are safe against odd subjects. Cost: slightly less readable git invocation.
- **LLM failure degrades gracefully.** On any error the heuristic headline is used and a warning goes to stderr, so a flaky endpoint never breaks a report.
- **ISO weeks in UTC.** Stable across machines and time zones; trade-off is that a commit near midnight local time may land in the neighbouring week.

## Out of scope
Merge-commit/PR title resolution via a forge API, release tagging, config files, and per-author filtering.
