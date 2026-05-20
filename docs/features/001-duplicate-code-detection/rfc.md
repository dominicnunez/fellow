# RFC 001: Duplicate Code Detection

Status: Draft

## Summary

Add clone detection for Go source so `gallow` can report repeated logic across files and packages.

## Problem

Dead-code analysis identifies unused code, but it does not identify copied logic that increases maintenance risk. Teams need a deterministic way to find duplicated blocks before refactoring or reviewing AI-generated changes.

## Required Behavior

- Analyze Go source files under each module.
- Report clone groups with file paths, line ranges, and duplicated line counts.
- Support at least one conservative mode that ignores formatting-only differences.
- Exclude generated files when `--ignore-generated` is set.
- Include test files unless `--production` is set.

## Constraints

- The first version should avoid type-checking requirements.
- Results must be deterministic across runs.
- Large repos should not require pairwise file comparison.

## Out of Scope

- Architecture boundary enforcement.
- Automatic refactoring of duplicate code.
- Semantic equivalence beyond syntactic clone detection.

## Acceptance Criteria

- A fixture with copied Go blocks reports one clone group.
- A fixture with only shared imports or declarations does not report a clone.
- JSON output includes clone family locations and line ranges.
- Human output includes enough context to find each duplicate block.
