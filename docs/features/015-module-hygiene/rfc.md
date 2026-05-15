# RFC 015: Module Hygiene

Status: Draft

## Summary

Add read-only Go module hygiene checks that report what module maintenance commands and module directives imply without mutating `go.mod` or `go.sum`.

## Problem

`fellow` already reports unused direct requirements, unlisted imports, test-only dependencies, and nested module drift. Go module maintenance has additional sources of stale or risky state that are not captured by import-to-require comparison alone:

- `go mod tidy` may change `go.mod` or `go.sum`.
- `go.sum` may be missing needed integrity entries or contain stale entries.
- `replace` directives may be unused or point at local development paths.
- Go tool dependencies may be intentionally present without production imports.
- Dependencies may only be visible under specific build tags or platform files.

These issues should be visible in CI and audit reports, but normal analysis must stay read-only.

## Required Behavior

- Add a module hygiene analysis surface that is report-only by default.
- Report when `go mod tidy` would change `go.mod` or `go.sum`.
- Report unused `replace` directives when the replaced module is not required, imported, used by tools, or otherwise resolved.
- Report suspicious local `replace` directives that point outside the module unless they resolve to a known sibling module or workspace module.
- Report missing `go.sum` entries needed for the current module graph.
- Report stale `go.sum` entries when tidy would remove them.
- Treat Go `tool` directives and conventional tool dependencies as intentional dependency usage.
- Report malformed or unresolved tool declarations as module hygiene findings.
- Carry build-context metadata when a dependency finding only appears under specific tags or platforms.
- Reuse RFC 014 build tag matrixing for multi-context dependency coverage instead of adding a second matrix mechanism.

## Finding Types

- `tidy-drift`: `go mod tidy` would edit module files.
- `missing-sum`: a needed `go.sum` entry is absent.
- `stale-sum`: a `go.sum` entry is no longer needed.
- `unused-replace`: a `replace` directive has no effect on the resolved graph.
- `local-replace`: a local filesystem replacement may be an accidental development override.
- `tool-dependency`: a tool directive or conventional tool dependency is malformed, unresolved, or intentionally keeping a requirement live.
- Existing dependency finding types may gain build-context metadata.

## Constraints

- Analysis must never write `go.mod` or `go.sum`.
- Do not run `go get`, `go mod tidy` without `-diff`, or any command that mutates module files.
- Do not add dependency allow/block policy; that belongs to dedicated policy tools.
- Do not duplicate `go mod tidy` as an editor. `fellow` reports drift and can recommend the command.
- Avoid network-dependent behavior where possible; prefer local module graph information and Go command read-only/diff modes.
- Findings must identify the module, file, line, and directive or dependency involved.

## Out of Scope

- Auto-fixing module files. Auto-fix remains gated by RFC 005.
- Dependency license, vulnerability, or organization policy enforcement.
- Replacing `go mod tidy` as the canonical module normalization command.
- Exhaustive platform matrixing beyond RFC 014.

## Acceptance Criteria

- A fixture where `go mod tidy -diff` would edit `go.mod` reports `tidy-drift` without modifying files.
- A fixture where `go mod tidy -diff` would edit `go.sum` reports missing or stale sum findings without modifying files.
- An unused `replace` directive is reported as `unused-replace`.
- A local `replace` pointing to a non-sibling path is reported as `local-replace`.
- A local `replace` pointing to a sibling module in the workspace is not reported as suspicious.
- A declared tool dependency is not reported as an unused application dependency.
- A malformed or unresolved tool declaration produces a `tool-dependency` finding.
- A dependency used only under configured build tags includes build-context metadata.
- Configured self-analysis remains zero findings when repository policy ignores intentional known findings.
