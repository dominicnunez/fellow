# ADR 007: Config File

Status: Proposed

## Context

CLI flags are useful for one-off runs, but teams need shared policy for CI and local development.

## Decision

Introduce `.gallowrc.json` as the default config format. CLI flags override config values. Unknown fields should fail validation so mistakes are visible.

## Consequences

- Project policy becomes repeatable.
- JSON schema or validation code is needed.
- Additional formats can be considered after the JSON contract stabilizes.

## Verification

Add tests for discovery, explicit config path, CLI precedence, invalid fields, and default behavior without config.
