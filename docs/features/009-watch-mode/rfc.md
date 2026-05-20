# RFC 009: Watch Mode

Status: Permission-Gated

Implementation note: do not implement this RFC without explicit user permission. Watch mode introduces long-running process behavior and file-watching dependencies that must remain a separately approved scope.

## Summary

Add a watch command that reruns analysis when relevant project files change.

## Problem

Running the analyzer manually after each edit slows feedback. Watch mode gives developers fast local feedback while they refactor or clean up findings.

## Required Behavior

- Add `gallow watch`.
- Watch `.go`, `go.mod`, and `go.sum` files under the selected root.
- Debounce rapid file changes.
- Reuse config and CLI flags from normal analysis.
- Print changed summaries without flooding the terminal.

## Constraints

- Watch mode should avoid analyzing vendor and ignored directories.
- The command must exit cleanly on interrupt.
- File watching should work on Linux, macOS, and Windows when feasible.

## Out of Scope

- Browser UI.
- Background daemon mode.
- Architecture boundary enforcement.

## Acceptance Criteria

- Editing a Go file triggers a re-analysis.
- Editing `go.mod` triggers dependency analysis.
- Rapid repeated writes are debounced into a single run.
- Interrupting the process exits without stack traces.
