# PRD 005: Auto Fix

Status: Permission-Gated

## Goal

Define future product requirements for safe automatic fixes while preventing accidental implementation.

## Users

- Developers who want mechanical cleanup after review.
- CI users who want machine-readable fix suggestions.

## Requirements

- Do not implement without explicit user permission.
- Keep normal `gallow` commands read-only.
- If approved later, start with `gallow fix --dry-run`.
- If approved later, represent supported fixes as JSON actions.

## Non-Functional Requirements

- File mutation must be opt-in.
- Dry-run output must show exact planned edits.
- Fix support must avoid generated files.

## Acceptance Criteria

- Current codebase has no auto-fix command implementation unless explicitly approved later.
- Planning docs state the permission gate.
- Any future implementation requires dry-run tests before apply-mode tests.
- Unsupported findings remain report-only.
