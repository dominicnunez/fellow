# PRD 004: Baselines

Status: Draft

## Goal

Allow teams to adopt `fellow` without failing CI on known existing findings.

## Users

- CI maintainers.
- Developers working in legacy repos.
- Teams planning staged cleanup.

## Requirements

- Save current findings to a baseline file.
- Load a baseline and suppress matching findings.
- Report active and suppressed counts.
- Preserve normal analyzer errors.

## Non-Functional Requirements

- Baselines must be plain JSON.
- Matching should be deterministic.
- Malformed files must fail clearly.

## Acceptance Criteria

- Reusing a saved baseline suppresses unchanged findings.
- New findings are still reported.
- JSON includes suppressed counts.
- Invalid baseline JSON returns a useful error.
