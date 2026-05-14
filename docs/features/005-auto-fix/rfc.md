# RFC 005: Auto Fix

Status: Draft

Implementation note: do not implement this RFC without explicit user permission. Auto-fix changes mutate project files and must remain a separately approved scope.

## Summary

Add safe automatic fixes for selected findings, starting with unused `go.mod` requirements.

## Problem

Findings are more useful when safe cleanup can be previewed and applied. Dependency drift is a strong candidate because unused direct requirements can often be removed mechanically.

## Required Behavior

- Add `fellow fix --dry-run` to preview changes.
- Add `fellow fix` to apply supported fixes.
- Start with unused direct `require` removal from `go.mod`.
- Run or recommend `go mod tidy` after dependency edits.
- Emit machine-readable actions in JSON output.

## Constraints

- Fixes must be opt-in and never run during normal analysis.
- Dry-run output must show exact files and edits.
- Fixes must avoid generated files.

## Out of Scope

- Automatic deletion of Go source files.
- Semantic refactoring.
- Architecture boundary enforcement.

## Acceptance Criteria

- Dry run reports the unused requirement that would be removed.
- Apply mode removes the selected requirement from `go.mod`.
- Supported fixes are represented in JSON actions.
- Unsupported findings remain report-only.
