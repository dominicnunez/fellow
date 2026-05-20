# PRD 007: Config File

Status: Draft

## Goal

Provide a project-level configuration file for repeatable analyzer behavior.

## Users

- Teams running `gallow` in CI.
- Developers sharing local defaults.
- Agents needing stable project policy.

## Requirements

- Discover `.gallowrc.json` by default.
- Support `--config <path>`.
- Configure rules, thresholds, ignore patterns, and output defaults.
- Validate unknown fields and invalid values.
- Document precedence rules.

## Non-Functional Requirements

- Defaults must preserve current behavior.
- Validation errors must be actionable.
- CLI overrides must be deterministic.

## Acceptance Criteria

- A config can disable one rule.
- CLI flags override config values.
- Invalid config exits with a clear error.
- README documents config usage and precedence.
