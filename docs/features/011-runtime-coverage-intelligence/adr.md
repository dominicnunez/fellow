# ADR 011: Runtime Coverage Intelligence

Status: Proposed

## Context

Static analysis cannot tell whether reachable code executes in tests or production-like runs. Coverage evidence can prioritize findings and identify gaps.

## Decision

Accept local Go coverage profiles as optional input. Map coverage to files and functions, then annotate existing findings and health reports without changing static results by default.

## Consequences

- Static analysis remains useful without coverage.
- Coverage path normalization becomes important.
- Production telemetry ingestion is deferred.

## Verification

Add coverage profile fixtures for covered functions, uncovered functions, stale paths, and mixed package coverage.
