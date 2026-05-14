# RFC 014: Build Tag Matrixing

Status: Draft

## Summary

Add analysis across multiple `GOOS`, `GOARCH`, and custom build tag combinations.

## Problem

Go files can be included or excluded by build constraints. An analyzer that only sees the current environment can miss platform-specific code or incorrectly report it as unused.

## Required Behavior

- Accept explicit build tags through CLI or config.
- Support analyzing multiple `GOOS` and `GOARCH` combinations.
- Record which build context produced each finding.
- Merge duplicate findings across contexts when appropriate.
- Respect `--production` test exclusion consistently across build contexts.

## Constraints

- Matrix size must be bounded to avoid unexpectedly expensive runs.
- Defaults should match the current Go environment unless configured otherwise.
- Findings must clearly distinguish context-specific and all-context issues.

## Out of Scope

- Cross-compilation builds.
- Running tests for every build context.
- Architecture boundary enforcement.

## Acceptance Criteria

- A fixture with OS-specific files is analyzed correctly for configured targets.
- A finding present only on one platform includes that platform context.
- Default behavior remains equivalent to current-environment analysis.
- JSON output includes build context metadata.
