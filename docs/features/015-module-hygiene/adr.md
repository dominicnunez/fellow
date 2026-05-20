# ADR 015: Module Hygiene

Status: Proposed

## Context

Go module state is maintained by the Go command, especially `go mod tidy`. `gallow` should expose stale or risky module state in reports, but it should not mutate files during normal analysis. Mutation is covered separately by the gated auto-fix feature.

Current dependency checks are based on direct requirements and imports. That catches the main Fallow-style dependency drift cases, but it does not explain all module hygiene concerns users encounter in CI:

- tidy would edit `go.mod` or `go.sum`;
- a local replacement was accidentally committed;
- tool dependencies are intentionally present even without production imports;
- conventional tool-import patterns should not be misclassified as application dependency usage.

## Decision

Add a focused first slice of module hygiene instead of a broad module-maintenance suite.

The feature will extend dependency analysis with read-only checks for:

- tidy drift detection via `go mod tidy -diff`;
- suspicious local `replace` directive reporting;
- Go tool directive and conventional tool dependency awareness.

This keeps the first implementation immediately useful and low-risk while preserving the separation between reporting and mutation.

## Rationale

- `go mod tidy -diff` gives a read-only answer for the highest-value module-file drift check.
- Local `replace` directives are common accidental-commit risks and can be detected from `go.mod` without graph-heavy analysis.
- Tool directives and `tools.go` patterns affect whether a requirement is intentionally present.
- Detailed `go.sum`, unused replacement, and build-context matrixing can be added later if the first slice proves useful.
- Read-only reporting is safer for CI and local review than automatic cleanup.

## Consequences

- The analyzer must distinguish “unused dependency” from “intentionally retained for a tool”.
- Tidy drift depends on Go command behavior and should use non-mutating diff output.
- JSON and CI serializers need new finding types.
- Module hygiene findings should be suppressible through existing rule and targeted ignore mechanisms.
- Auto-fix can later consume these findings, but only if RFC 005 is explicitly implemented.
- The first slice intentionally does not classify every `go.sum` hunk or every unused replacement case.

## Alternatives Considered

- Broad module hygiene suite: rejected for the first slice because it would require fragile tidy-diff parsing and graph-heavy replacement analysis before the highest-value checks are proven.
- Implementing only tidy drift: rejected because local `replace` and tool dependencies are low-risk additions that affect the same user decisions.
- Running `go mod tidy` and applying edits: rejected because normal analysis must be read-only.
- Adding dependency allow/block policy: rejected because that is a policy-linting problem, not dependency drift.

## Verification

- Add fixtures for tidy drift in `go.mod`, tidy drift in `go.sum`, local replacement, sibling-module replacement, Go tool directives, and conventional `tools.go` imports.
- Verify module files are unchanged after analysis.
- Verify existing dependency findings remain unchanged when no module hygiene issues exist.
- Verify all new findings round-trip through JSON, SARIF, CodeClimate, GitLab Code Quality, and annotations output.
