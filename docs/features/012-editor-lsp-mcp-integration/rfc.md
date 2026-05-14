# RFC 012: Editor, LSP, And MCP Integration

Status: Draft

## Summary

Add integration surfaces for editors and coding agents, starting with stable machine-readable commands and eventually LSP or MCP support.

## Problem

CLI output is useful, but developers and agents benefit from structured diagnostics, code actions, and direct tool calls. Integration reduces context switching and makes findings actionable during editing.

## Required Behavior

- Define stable JSON contracts for diagnostics and actions.
- Provide enough metadata for editor diagnostics: file, line, rule, severity, message, and fix availability.
- Consider an LSP server for real-time diagnostics.
- Consider an MCP server for agent workflows.
- Version output schemas to protect integrations.

## Constraints

- The CLI JSON contract should be stable before adding servers.
- Long-running servers must reuse analyzer code rather than fork behavior.
- Editor integration should respect project config.

## Out of Scope

- Publishing editor extensions in the first iteration.
- Cloud services.
- Architecture boundary enforcement.

## Acceptance Criteria

- JSON diagnostics can be consumed by a simple editor adapter.
- Schema version appears in machine-readable output.
- Proposed LSP diagnostics map directly to existing findings.
- MCP tool design documents input and output contracts.
