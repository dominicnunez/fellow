# PRD 012: Editor, LSP, And MCP Integration

Status: Draft

## Goal

Make `gallow` usable from editors and coding agents without scraping human output.

## Users

- Developers using editor diagnostics.
- Coding agents invoking deterministic tools.
- Integrators building workflow automation.

## Requirements

- Version machine-readable diagnostic schemas.
- Include file, line, rule, severity, message, and action metadata.
- Define LSP diagnostic mapping.
- Define MCP tool inputs and outputs.
- Respect project config.

## Non-Functional Requirements

- Servers must reuse CLI analyzer behavior.
- Schema changes must be intentional.
- Long-running integrations should avoid stale config state.

## Acceptance Criteria

- JSON diagnostics include schema version.
- LSP diagnostics map directly from existing findings.
- MCP tool contracts are documented.
- A simple adapter can consume CLI JSON diagnostics.
