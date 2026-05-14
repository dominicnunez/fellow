# PRD 001: Duplicate Code Detection

Status: Draft

## Goal

Help users find duplicated Go logic that should be reviewed or refactored.

## Users

- Developers cleaning up a repo.
- Reviewers checking generated or AI-assisted changes.
- CI users tracking duplicate code growth.

## Requirements

- Report duplicate clone groups with file and line ranges.
- Include duplicated line counts and a stable group identifier.
- Respect `--production` and `--ignore-generated`.
- Support human and JSON output.

## Non-Functional Requirements

- Deterministic output.
- Practical performance on large repos.
- No dependency on successful type checking.

## Acceptance Criteria

- Copied blocks across two files produce one clone group.
- Repeated import blocks do not produce a clone group.
- JSON output includes all clone locations.
- Human output is concise enough for PR review.
