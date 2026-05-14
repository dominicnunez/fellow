# ADR 005: Auto Fix

Status: Permission-Gated

## Context

Auto-fix can mutate user files. Even safe-looking dependency edits can interact with module resolution, generated files, local replacements, and user changes.

## Decision

Do not implement auto-fix without explicit user permission. Planning documents may exist, but code that changes files automatically is out of scope until separately approved.

## Consequences

- Normal analysis remains read-only.
- Any future implementation must start with dry-run behavior.
- Auto-fix work requires a fresh approval checkpoint before code changes begin.

## Verification

Before implementation is allowed, confirm explicit user approval and add tests proving normal analysis never mutates files.
