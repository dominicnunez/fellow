# PRD 010: Workspace Filtering

Status: Draft

## Goal

Let users scope analysis to selected modules in large Go workspaces.

## Users

- Monorepo developers.
- CI maintainers optimizing runtime.
- Reviewers focusing on changed modules.

## Requirements

- Parse `go.work` when present.
- Support `--workspace <pattern>`.
- Support changed-module detection from a Git base ref.
- Preserve `used_in_modules` context.
- Report skipped modules when requested.

## Non-Functional Requirements

- Filtering must be deterministic.
- Git failures must fail clearly.
- Child modules must remain isolated from parent attribution.

## Acceptance Criteria

- A `go.work` fixture discovers listed modules.
- Workspace filters limit reported findings.
- Changed-module mode scopes analysis to touched modules.
- JSON includes selected and skipped modules.
