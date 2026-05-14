# ADR 004: Baselines

Status: Proposed

## Context

Existing projects may have enough findings that immediate CI blocking is impractical. Baselines allow incremental adoption.

## Decision

Store baselines as JSON snapshots of stable finding fingerprints. Matching findings are suppressed from active output while counts remain visible.

## Consequences

- Teams can ratchet without fixing all findings first.
- Fingerprint stability becomes important for every rule.
- Resolved baseline cleanup can be a separate feature.

## Verification

Add tests for save, load, newly introduced findings, malformed baseline files, and line-shift tolerance where possible.
