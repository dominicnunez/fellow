# ADR 001: Duplicate Code Detection

Status: Proposed

## Context

`fellow` currently detects dependency drift and dead code, but it does not identify repeated code blocks. Duplicate detection should be useful during review without requiring type checking or full compilation.

## Decision

Use a token-normalized clone detector as the initial implementation. Parse Go files, normalize syntax into comparable token windows, and index repeated windows with deterministic hashing before expanding reports into clone groups.

## Consequences

- The first version can run on incomplete projects.
- Formatting-only differences should not affect results.
- Semantic clones with renamed logic are deferred until the simpler detector is proven useful.

## Verification

Add fixtures for copied blocks, near-misses, generated files, and test-file inclusion or exclusion.
