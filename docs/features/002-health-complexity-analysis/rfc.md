# RFC 002: Health And Complexity Analysis

Status: Draft

## Summary

Add maintainability analysis for Go functions and methods, including complexity metrics and module-level health summaries.

## Problem

Dead code is not the only review risk. Large or highly branched functions deserve attention even when they are used. `fellow` should help reviewers identify risky code paths and refactor candidates.

## Required Behavior

- Compute cyclomatic complexity for Go functions and methods.
- Compute a cognitive complexity score or clearly documented equivalent.
- Report functions above configurable thresholds.
- Provide summary counts and top offenders.
- Respect `--production` and `--ignore-generated`.

## Constraints

- Metrics must be stable and documented.
- The initial implementation should not require external services.
- Threshold defaults should be conservative enough for existing projects.

## Out of Scope

- Runtime hot-path analysis.
- Automatic function splitting.
- Architecture boundary enforcement.

## Acceptance Criteria

- A fixture with deeply nested control flow is reported.
- A simple function is not reported.
- JSON output includes metric values and thresholds.
- Human summary includes analyzed function count and above-threshold count.
