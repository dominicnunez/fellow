# RFC 015: Module Hygiene

Status: Draft

## Summary

Add a low-risk, read-only Go module hygiene slice that reports tidy drift, suspicious local replacements, and tool dependency intent without mutating `go.mod` or `go.sum`.

## Problem

`fellow` already reports unused direct requirements, unlisted imports, test-only dependencies, and nested module drift. Go module maintenance has a few high-value hygiene issues that are not captured by import-to-require comparison alone:

- `go mod tidy` may change `go.mod` or `go.sum`.
- `replace` directives may point at local development paths that should not be merged accidentally.
- Go tool dependencies may be intentionally present without production imports.

These issues should be visible in CI and audit reports, but normal analysis must stay read-only.

## Required Behavior

- Run `go mod tidy -diff` per selected module and report when it would change module files.
- Never run a mutating tidy command during normal analysis.
- Report tidy drift as a `tidy-drift` finding, including the module and which module files would change when that can be determined from the diff.
- Report suspicious local `replace` directives that point outside the module unless they resolve to a known sibling module or workspace module.
- Treat Go `tool` directives as intentional dependency usage so tool requirements are not reported as unused application dependencies.
- Treat conventional tool dependency patterns, such as build-tagged `tools.go` imports, as intentional usage where the repo has not migrated to Go tool directives.
- Report malformed tool directives only when they can be identified from `go.mod` parsing without invoking mutating commands.

## Finding Types

- `tidy-drift`: `go mod tidy` would edit module files.
- `local-replace`: a local filesystem replacement may be an accidental development override.
- `tool-dependency`: a malformed tool directive or unresolved tool declaration needs attention.

## Constraints

- Analysis must never write `go.mod` or `go.sum`.
- Do not run `go get`, `go mod tidy` without `-diff`, or any command that mutates module files.
- Do not add dependency allow/block policy; that belongs to dedicated policy tools.
- Do not duplicate `go mod tidy` as an editor. `fellow` reports drift and can recommend the command.
- Avoid network-dependent behavior where possible; tidy failures caused by unavailable downloads should be reported as analysis errors, not guessed hygiene findings.
- Findings must identify the module, file, line, and directive or dependency involved.

## Out of Scope

- Auto-fixing module files. Auto-fix remains gated by RFC 005.
- Dependency license, vulnerability, or organization policy enforcement.
- Replacing `go mod tidy` as the canonical module normalization command.
- Detailed `missing-sum` and `stale-sum` classifications. The first slice reports `tidy-drift` rather than parsing every diff hunk into semantic sum-entry findings.
- Unused `replace` directive analysis. The first slice only reports suspicious local replacements.
- Build-tag or platform matrix metadata. That belongs to RFC 014.

## Acceptance Criteria

- A fixture where `go mod tidy -diff` would edit `go.mod` reports `tidy-drift` without modifying files.
- A fixture where `go mod tidy -diff` would edit `go.sum` reports `tidy-drift` without modifying files.
- A local `replace` pointing to a non-sibling path is reported as `local-replace`.
- A local `replace` pointing to a sibling module in the workspace is not reported as suspicious.
- A declared tool dependency is not reported as an unused application dependency.
- A conventional `tools.go` dependency is not reported as an unused application dependency.
- A malformed tool declaration produces a `tool-dependency` finding when parseable from module file state.
- Configured self-analysis remains zero findings when repository policy ignores intentional known findings.
