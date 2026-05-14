# ADR 008: CI Report Formats

Status: Proposed

## Context

CI platforms can surface diagnostics inline when tools emit supported formats. `fellow` currently supports human and JSON output only.

## Decision

Add report serializers behind `--format`, starting with SARIF and GitLab Code Quality compatible JSON. Keep upload or API posting out of the CLI.

## Consequences

- CI integrations remain network-free.
- Rule IDs and fingerprints must be stable.
- Schema validation should be part of tests.

## Verification

Add fixtures validating SARIF structure, GitLab Code Quality fields, annotation formatting, and unchanged existing JSON output.
