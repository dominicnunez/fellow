# ADR 003: Audit Mode

Status: Proposed

## Context

Full-repo analysis can overwhelm pull request review. Audit mode should focus on changed files and produce a clear verdict for CI.

## Decision

Use Git diff information to scope findings to changed files. Keep inherited-vs-introduced attribution as a later enhancement unless baseline comparison is already available.

## Consequences

- Initial audit mode can be implemented without temporary worktrees.
- CI behavior is predictable and easy to explain.
- Precise introduced attribution requires follow-up work.

## Verification

Add Git fixture tests for changed files, unstaged changes, missing base refs, and clean working trees.
