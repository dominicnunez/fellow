# RFC 008: CI Report Formats

Status: Draft

## Summary

Add CI-native output formats such as SARIF, CodeClimate, GitLab Code Quality, and GitHub annotations.

## Problem

Human and JSON output are useful, but CI platforms can display findings inline when reports follow standard formats. Native formats reduce friction for pull request review.

## Required Behavior

- Add `--format sarif` for GitHub code scanning.
- Add `--format codeclimate` or `--format gitlab-codequality` for GitLab-compatible reports.
- Add `--format annotations` for GitHub Actions log annotations.
- Preserve current `human` and `json` formats.
- Include stable rule IDs and severity mappings.

## Constraints

- Output must conform to each target schema.
- File paths should be relative to the project root when required by the target format.
- CI formats should not require network access.

## Out of Scope

- Uploading reports to GitHub or GitLab.
- Posting pull request comments.
- Architecture boundary enforcement.

## Acceptance Criteria

- SARIF output validates against the SARIF schema.
- GitLab Code Quality output includes file, line, severity, and fingerprint.
- GitHub annotations output can be consumed by a GitHub Actions step.
- Existing JSON output remains backward compatible for current fields.
