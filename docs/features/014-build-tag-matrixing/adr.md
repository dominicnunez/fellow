# ADR 014: Build Tag Matrixing

Status: Proposed

## Context

Go build constraints can hide or reveal files based on `GOOS`, `GOARCH`, and custom tags. Current analysis follows the active environment.

## Decision

Add explicit build contexts as configuration. Default to the current environment, and allow bounded matrices for users who need platform coverage.

## Consequences

- Default behavior stays familiar.
- Matrix analysis can be expensive and must be bounded.
- Findings need build-context metadata.

## Verification

Add fixtures for OS-specific files, architecture-specific files, custom tags, and merged findings across contexts.
