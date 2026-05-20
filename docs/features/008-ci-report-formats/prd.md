# PRD 008: CI Report Formats

Status: Draft

## Goal

Make `gallow` findings visible in CI review surfaces through standard report formats.

## Users

- CI maintainers.
- Pull request reviewers.
- Developers consuming inline diagnostics.

## Requirements

- Add SARIF output.
- Add GitLab Code Quality or CodeClimate output.
- Add GitHub Actions annotation output.
- Include rule IDs, severity, file, line, message, and fingerprint.

## Non-Functional Requirements

- Formats must validate against target schemas.
- Output must be deterministic.
- No network access required.

## Acceptance Criteria

- SARIF validates against the schema.
- GitLab output includes required fields.
- Annotation output is accepted by GitHub Actions logs.
- Current JSON output remains compatible.
