# ADR 009: Watch Mode

Status: Permission-Gated

## Context

Developers need faster feedback while cleaning up findings. Re-running the CLI manually after each edit is inefficient.

## Decision

Do not implement watch mode without explicit user permission. Planning documents may exist, but long-running watcher behavior and file-watching dependencies are out of scope until separately approved.

## Consequences

- Normal `gallow` commands remain single-run processes.
- Any future implementation must start with dependency and portability review.
- Watch mode work requires a fresh approval checkpoint before code changes begin.

## Verification

Before implementation is allowed, confirm explicit user approval and add tests or integration fixtures for debounce behavior, watched file types, ignored directories, and interrupt handling.
