# ADR 010: Workspace Filtering

Status: Proposed

## Context

Multi-module Go repos need scoped analysis without losing cross-module context. `gallow` already handles nested modules but does not fully model `go.work` selection.

## Decision

Parse `go.work` files and add module filters that select analyzed modules while preserving context for dependency usage in other modules.

## Consequences

- Large repos can run narrower checks.
- Output needs selected and skipped module metadata.
- Changed-module detection depends on Git diff behavior.

## Verification

Add fixtures for `go.work`, explicit workspace filters, changed-module selection, and nested module boundaries.
