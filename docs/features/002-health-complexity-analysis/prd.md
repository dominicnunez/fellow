# PRD 002: Health And Complexity Analysis

Status: Draft

## Goal

Identify Go functions and methods that deserve review because of complexity.

## Users

- Developers planning refactors.
- Reviewers triaging risky changes.
- CI maintainers enforcing complexity thresholds.

## Requirements

- Count analyzed functions and methods.
- Report functions above default thresholds.
- Include metric values in human and JSON output.
- Allow thresholds through config once config support exists.

## Non-Functional Requirements

- Metric behavior must be documented.
- Results must be stable across runs.
- Analysis should remain fast enough for local use.

## Acceptance Criteria

- A nested fixture reports a complexity finding.
- A simple function does not report a finding.
- JSON includes metric names, values, and thresholds.
- Summary includes analyzed and above-threshold counts.
