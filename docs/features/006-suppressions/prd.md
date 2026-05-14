# PRD 006: Suppressions

Status: Draft

## Goal

Let users intentionally hide specific findings while keeping analysis useful and auditable.

## Users

- Developers handling reflection-heavy code.
- Reviewers who need visible exceptions.
- CI maintainers avoiding broad rule disables.

## Requirements

- Support line-level suppressions.
- Support file-level suppressions.
- Require explicit rule IDs.
- Report stale suppressions when requested.
- Include suppressed counts in JSON.

## Non-Functional Requirements

- Suppression syntax must be stable.
- Suppressions should be easy to review in code.
- Stale suppression checks should be deterministic.

## Acceptance Criteria

- A targeted suppression hides one matching finding.
- A file suppression hides findings in that file.
- An unused suppression can be reported as stale.
- Suppressed finding counts appear in machine-readable output.
