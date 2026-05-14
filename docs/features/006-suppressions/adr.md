# ADR 006: Suppressions

Status: Proposed

## Context

Some static findings are intentional because of runtime hooks, reflection, external APIs, or migration windows. Users need precise suppressions without disabling whole rules.

## Decision

Support explicit rule-scoped comments and config ignores. Prefer narrow suppressions and track stale suppressions as a separate reportable finding.

## Consequences

- Findings can be documented where exceptions occur.
- Suppression parsing becomes part of rule evaluation.
- Stale suppression detection requires retaining suppressed finding metadata.

## Verification

Add tests for next-line suppression, file suppression, multi-rule comments, stale suppressions, and JSON suppression metadata.
