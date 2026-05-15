# PRD 015: Module Hygiene

Status: Draft

## Goal

Make `fellow` report the highest-value Go module hygiene issues that go beyond import-to-require drift while keeping normal analysis read-only.

## Users

- Go developers who want CI to catch stale module files before review.
- Maintainers of multi-module repositories with local replacements and workspace modules.
- Teams that track tool dependencies in `go.mod`.
- Reviewers auditing dependency changes across build tags or platforms.

## Requirements

- Report whether module files are out of sync with tidy semantics by running `go mod tidy -diff` read-only.
- Report suspicious local `replace` directives.
- Do not report local replacements that target known sibling or workspace modules.
- Treat Go `tool` directives as valid dependency usage.
- Support conventional tool dependency patterns where Go version support is not available or a repo has not migrated yet.
- Support rule severity configuration for all new finding types.
- Support targeted `ignoreFindings` for all new finding types.

## Non-Functional Requirements

- Analysis must be read-only by default.
- Analysis must not edit `go.mod` or `go.sum`.
- Checks should be deterministic in CI.
- Network-bound tidy failures should fail analysis clearly instead of being guessed as hygiene findings.
- Error messages should recommend the responsible command or directive to inspect.
- Performance should remain acceptable for normal single-context analysis.

## User Stories

- As a developer, I want `fellow` to tell me when `go mod tidy` would change files, so CI can catch stale module state.
- As a reviewer, I want local `replace` directives called out, so development overrides are not accidentally merged.
- As a tool maintainer, I want tool dependencies to be recognized, so they are not misclassified as unused application dependencies.

## Acceptance Criteria

- Running analysis on a tidy-drift fixture reports `tidy-drift` and leaves files unchanged.
- Running analysis on a `go.sum` tidy-drift fixture reports `tidy-drift` and leaves files unchanged.
- Running analysis on an accidental local replacement fixture reports `local-replace`.
- Running analysis on a sibling-module replacement fixture reports no suspicious local replacement.
- Running analysis on a tool directive fixture keeps the tool dependency live.
- Running analysis on a conventional `tools.go` fixture keeps the tool dependency live.
- Running analysis on a malformed parseable tool directive fixture reports `tool-dependency`.
- All new findings are emitted in human and JSON formats.
- CI serializers include equivalent diagnostics for all new findings.

## Out of Scope

- Applying `go mod tidy` edits.
- Removing or rewriting directives automatically.
- License, vulnerability, or organization dependency policy.
- General architecture boundary enforcement.
- Detailed `missing-sum` and `stale-sum` classifications.
- Unused non-local `replace` directive analysis.
- Build-tag or platform matrix metadata beyond existing dependency checks.

## Open Questions

- Should `tidy-drift` be one finding per module or one finding per changed file?
- Should tidy diff excerpts be included in JSON output, or only file-level metadata?
- Which conventional tool dependency patterns should be supported before relying solely on Go `tool` directives?
- Should local replacements default to warning severity while tidy drift defaults to error severity?
