# ADR 002: Health And Complexity Analysis

Status: Proposed

## Context

Used code can still be risky when functions are large, deeply nested, or highly branched. `fellow` needs a static health surface that works without coverage or runtime data.

## Decision

Implement cyclomatic complexity first, then add cognitive complexity as a documented extension. Analyze functions and methods from parsed Go ASTs and report findings above configurable thresholds.

## Consequences

- The first version has a simple, explainable metric.
- Health output can run without `go/types`.
- Maintainability scores and hotspots can be layered later.

## Verification

Add fixtures for branch-heavy functions, simple functions, methods, anonymous functions if supported, and threshold configuration.
