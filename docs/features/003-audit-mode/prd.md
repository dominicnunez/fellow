# PRD 003: Audit Mode

Status: Draft

## Goal

Provide a PR-friendly command that focuses `gallow` output on files touched by the current change set.

## Users

- Pull request authors.
- Reviewers.
- CI maintainers.

## Requirements

- Add `gallow audit`.
- Accept an explicit base ref.
- Detect changed files from Git.
- Filter findings to changed files.
- Emit a pass or fail verdict.

## Non-Functional Requirements

- Git failures must be explicit.
- The command must not mutate the worktree.
- Output should be concise by default.

## Acceptance Criteria

- A new finding in a changed file fails when `--fail-on-issues` is set.
- Findings in unchanged files are hidden by default.
- JSON includes base ref, changed files, and verdict.
- Unstaged changes are included in the audit scope.
