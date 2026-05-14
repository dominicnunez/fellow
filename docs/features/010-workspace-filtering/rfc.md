# RFC 010: Workspace Filtering

Status: Draft

## Summary

Add richer multi-module filtering for `go.work` workspaces and changed modules.

## Problem

Large Go repos often contain many modules. Teams need to analyze selected modules or only modules affected by a change set without losing cross-module context.

## Required Behavior

- Parse `go.work` files when present.
- Support `--workspace <pattern>` to include selected modules.
- Support changed-module detection from a Git base ref.
- Preserve `used_in_modules` context for dependency findings.
- Clearly report skipped modules when requested.

## Constraints

- Filtering must not attribute child module imports to parent modules.
- Git failures in changed-module mode should fail clearly.
- Patterns should be deterministic and documented.

## Out of Scope

- Creating or editing `go.work` files.
- Remote workspace discovery.
- Architecture boundary enforcement.

## Acceptance Criteria

- A `go.work` fixture discovers all listed modules.
- `--workspace` limits findings to matching modules.
- Changed-module mode scopes analysis to modules touched by a diff.
- JSON output includes selected and skipped module lists.
