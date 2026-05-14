# RFC 003: Audit Mode

Status: Draft

## Summary

Add a change-focused audit command that compares the current working tree against a base ref and reports findings relevant to changed files.

## Problem

Full-repo findings are useful for cleanup, but they can overwhelm pull request review. Reviewers need a mode that focuses on issues introduced or touched by the current change set.

## Required Behavior

- Add `fellow audit` with a configurable base ref.
- Detect changed files using Git.
- Run relevant analyses and filter findings to changed files.
- Exit with pass, warn, or fail semantics suitable for CI.
- Support JSON output with introduced or inherited attribution when available.

## Constraints

- Git errors in CI should fail clearly instead of silently widening scope.
- The command must not mutate the working tree.
- The default base detection must be documented.

## Out of Scope

- Pull request comment posting.
- Remote Git provider API integration.
- Architecture boundary enforcement.

## Acceptance Criteria

- A changed file with a new finding causes `fellow audit --fail-on-issues` to exit non-zero.
- Unchanged files are not shown in the default audit output.
- JSON output includes base ref, changed files, and verdict.
- Audit works when the working tree has unstaged changes.
