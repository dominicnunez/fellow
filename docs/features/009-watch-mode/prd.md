# PRD 009: Watch Mode

Status: Permission-Gated

## Goal

Define future product requirements for continuous local feedback while preventing accidental implementation.

## Users

- Developers refactoring code.
- Developers cleaning findings interactively.
- Agents running iterative edits.

## Requirements

- Do not implement without explicit user permission.
- Keep normal `gallow` commands as single-run processes.
- If approved later, add `gallow watch`.
- If approved later, watch `.go`, `go.mod`, and `go.sum` files.
- If approved later, debounce rapid changes and exit cleanly on interrupt.

## Non-Functional Requirements

- Avoid vendor and ignored directories.
- Keep repeated output readable.
- Support major development platforms where feasible.

## Acceptance Criteria

- Current codebase has no watch command implementation unless explicitly approved later.
- Planning docs state the permission gate.
- Any future implementation requires portability review before dependency changes.
- If approved later, interrupt handling and debounce behavior must be tested.
