# RFC 006: Suppressions

Status: Draft

## Summary

Add inline and config-based suppressions for intentional findings.

## Problem

Static analysis can identify valid exceptions, especially for runtime hooks and reflection-heavy code. Users need a visible, reviewable way to suppress findings without disabling whole analyses.

## Required Behavior

- Support line-level suppression comments for specific rule IDs.
- Support file-level suppression comments.
- Support config-based ignore patterns and ignored symbols.
- Report stale suppressions when requested.
- Include suppression metadata in JSON output.

## Constraints

- Suppressions must be explicit about the rule being ignored.
- Suppression syntax should be stable before broad use.
- Stale suppression detection should not require auto-fix support.

## Out of Scope

- IDE code actions for adding suppressions.
- Organization-wide suppression policy.
- Architecture boundary enforcement.

## Acceptance Criteria

- A line suppression hides only the targeted finding.
- A file suppression hides all supported findings in that file.
- An unused suppression can be reported as stale.
- JSON output includes suppressed counts when suppressions are present.
