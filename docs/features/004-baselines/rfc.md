# RFC 004: Baselines

Status: Draft

## Summary

Add baseline files so teams can adopt `gallow` incrementally and fail only on new or regressed findings.

## Problem

Existing projects may have many findings. Blocking CI on all findings immediately makes adoption difficult. Baselines let teams ratchet quality without stopping current work.

## Required Behavior

- Support `--save-baseline <path>` for current findings.
- Support `--baseline <path>` to suppress matching existing findings.
- Include enough stable finding identity to match across runs.
- Report baseline misses or malformed baseline files clearly.
- Support JSON and human summaries of suppressed and active findings.

## Constraints

- Baseline matching should survive unrelated line shifts when possible.
- Baselines should be plain JSON.
- Baselines must not hide analyzer errors.

## Out of Scope

- Cloud-hosted baseline storage.
- Automatic deletion of resolved baseline entries.
- Architecture boundary enforcement.

## Acceptance Criteria

- Saving and reusing a baseline suppresses unchanged findings.
- A newly introduced finding is still reported.
- Malformed baseline JSON returns a clear error.
- JSON output includes active and baseline-suppressed counts.
