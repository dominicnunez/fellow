# ADR 015: Module Hygiene

Status: Proposed

## Context

Go module state is maintained by the Go command, especially `go mod tidy`. `fellow` should expose stale or risky module state in reports, but it should not mutate files during normal analysis. Mutation is covered separately by the gated auto-fix feature.

Current dependency checks are based on direct requirements and imports. That catches the main Fallow-inspired dependency drift cases, but it does not explain all module hygiene concerns users encounter in CI:

- tidy would edit `go.mod` or `go.sum`;
- a `replace` directive is stale;
- a local replacement was accidentally committed;
- tool dependencies are intentionally present even without production imports;
- imports are hidden behind build tags or platform constraints.

## Decision

Add one module hygiene feature instead of six separate features.

The feature will extend dependency analysis with read-only module maintenance checks:

- tidy drift detection;
- `go.sum` integrity drift reporting;
- unused and suspicious `replace` directive reporting;
- Go tool dependency awareness;
- build-context-aware dependency findings integrated with RFC 014.

This keeps related module concerns in one implementation and one user-facing configuration surface while preserving the separation between reporting and mutation.

## Rationale

- `go mod tidy`, `go.sum`, `replace`, and tool directives are all aspects of module graph hygiene.
- Users want one answer for “is this module state clean?” rather than several unrelated commands.
- Build tags affect dependency visibility, but the general matrix mechanism already belongs to RFC 014.
- Read-only reporting is safer for CI and local review than automatic cleanup.

## Consequences

- The analyzer must distinguish “unused dependency” from “intentionally retained for a tool”.
- Some checks may depend on Go command behavior and should prefer non-mutating modes such as diff output.
- JSON and CI serializers need new finding types.
- Module hygiene findings should be suppressible through existing rule and targeted ignore mechanisms.
- Auto-fix can later consume these findings, but only if RFC 005 is explicitly implemented.

## Alternatives Considered

- Six independent features: rejected because they would fragment one module-hygiene workflow.
- Implementing only tidy drift: rejected because `replace` and tool dependencies affect the same user decisions.
- Running `go mod tidy` and applying edits: rejected because normal analysis must be read-only.
- Adding dependency allow/block policy: rejected because that is a policy-linting problem, not dependency drift.

## Verification

- Add fixtures for tidy drift, `go.sum` drift, stale `replace`, local `replace`, sibling-module replacement, tool directives, and build-tag-only imports.
- Verify module files are unchanged after analysis.
- Verify existing dependency findings remain unchanged when no module hygiene issues exist.
- Verify all new findings round-trip through JSON, SARIF, CodeClimate, GitLab Code Quality, and annotations output.
