# RFC 011: Runtime Coverage Intelligence

Status: Draft

## Summary

Add optional runtime and coverage evidence so findings can distinguish hot, cold, and unexecuted code.

## Problem

Static reachability shows what code can be referenced, not what actually runs. Runtime evidence helps reviewers prioritize risky hot paths and gain confidence when deleting cold code.

## Required Behavior

- Accept Go coverage profiles as input.
- Map coverage data back to packages, files, and functions.
- Annotate dead-code and health findings with coverage status when available.
- Report uncovered reachable functions as coverage gaps.
- Keep static analysis usable without coverage input.

## Constraints

- Coverage paths must be normalized relative to the analysis root.
- Missing or stale coverage should produce warnings, not false precision.
- Runtime evidence must not hide static findings by default.

## Out of Scope

- Hosted runtime collection.
- Production telemetry ingestion.
- Architecture boundary enforcement.

## Acceptance Criteria

- A Go coverage profile marks covered and uncovered functions.
- JSON output includes coverage metadata for mapped findings.
- Human output summarizes coverage input quality.
- Static-only behavior remains unchanged when no coverage is provided.
