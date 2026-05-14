# PRD 013: Additional Go Symbol Kinds

Status: Draft

## Goal

Expand dead-code reporting to more Go declaration kinds.

## Users

- Developers cleaning stale APIs.
- Reviewers looking for unused constants and interfaces.
- Teams tracking dead-code totals.

## Requirements

- Report unused interfaces.
- Report unused type aliases and non-struct types.
- Report unused package vars and consts.
- Report safe unused enum-like `iota` constants.
- Preserve side-effect safety for vars.

## Non-Functional Requirements

- Use `go/types` object identity when available.
- Avoid reporting local variables already handled by the compiler.
- Keep grouped declarations readable.

## Acceptance Criteria

- Fixtures cover each new symbol kind.
- Side-effectful vars are not reported as safely removable.
- JSON identifies symbol kind and location.
- Existing symbol tests still pass.
