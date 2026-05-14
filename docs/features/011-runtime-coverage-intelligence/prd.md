# PRD 011: Runtime Coverage Intelligence

Status: Draft

## Goal

Use coverage evidence to distinguish covered, uncovered, hot, or cold code when data is available.

## Users

- Reviewers prioritizing risky changes.
- Developers deciding what to delete.
- Teams tracking coverage gaps.

## Requirements

- Accept Go coverage profiles.
- Map coverage to packages, files, and functions.
- Annotate findings with coverage metadata.
- Report coverage gaps for reachable code.
- Preserve static-only behavior when no coverage is supplied.

## Non-Functional Requirements

- Missing coverage should warn, not imply certainty.
- Path mapping must be explicit and deterministic.
- Coverage should not hide static findings by default.

## Acceptance Criteria

- Covered and uncovered functions are identified from a profile.
- JSON includes coverage metadata.
- Human output summarizes coverage quality.
- No-coverage runs behave as they do today.
