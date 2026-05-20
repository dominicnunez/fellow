# RFC 007: Config File

Status: Draft

## Summary

Add a project configuration file for rule settings, ignore patterns, thresholds, and output defaults.

## Problem

CLI flags are sufficient for one-off usage, but teams need repeatable project policy. A config file lets CI, local development, and agents share the same analyzer behavior.

## Required Behavior

- Discover `.gallowrc.json` from the project root by default.
- Support `--config <path>` to load an explicit file.
- Configure rule severity, ignore patterns, complexity thresholds, and output defaults.
- Validate unknown fields and invalid values with clear errors.
- Document precedence between CLI flags and config values.

## Constraints

- JSON is the initial supported format.
- CLI flags should override config values.
- Defaults must preserve current behavior when no config exists.

## Out of Scope

- TOML or YAML config support in the first version.
- Interactive config generation.
- Architecture boundary enforcement.

## Acceptance Criteria

- A config file can disable one rule without disabling others.
- CLI flags override conflicting config settings.
- Invalid config exits with a clear diagnostic.
- README documents the config file path and precedence model.
