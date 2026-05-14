# ADR 012: Editor, LSP, And MCP Integration

Status: Proposed

## Context

Editors and agents need stable machine-readable diagnostics before server integrations can be reliable.

## Decision

Stabilize the JSON diagnostic contract first. Treat LSP and MCP servers as later adapters over the same analyzer and schema rather than separate implementations.

## Consequences

- CLI JSON schema quality becomes a prerequisite.
- Server implementations can reuse analyzer code.
- Publishing editor extensions is deferred.

## Verification

Add schema-version tests, diagnostic contract snapshots, and adapter-level tests when LSP or MCP support begins.
