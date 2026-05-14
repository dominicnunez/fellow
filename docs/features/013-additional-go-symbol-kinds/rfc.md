# RFC 013: Additional Go Symbol Kinds

Status: Draft

## Summary

Extend dead-code detection beyond functions, methods, structs, and fields to cover interfaces, type aliases, vars, consts, and enum-like groups.

## Problem

The current analyzer misses several symbol kinds that can accumulate as code evolves. Expanding symbol coverage makes dead-code reports more complete and closer to Fallow-style codebase intelligence.

## Required Behavior

- Report unused interfaces.
- Report unused type aliases and non-struct type declarations.
- Report unused package-level vars and consts.
- Report unused enum-like `iota` constants where safe.
- Preserve special handling for exported APIs, reflection, and tests where needed.

## Constraints

- Symbol liveness should use `go/types` object identity when packages type-check.
- Package-level vars with initialization side effects must not be reported as safely removable.
- Const and var grouping should produce readable findings.

## Out of Scope

- Local variable unused checks already handled by the Go compiler.
- Automatic source deletion.
- Architecture boundary enforcement.

## Acceptance Criteria

- Fixtures cover unused interface, alias, var, const, and `iota` cases.
- Vars with side-effectful initializers are not reported as removable.
- JSON output identifies symbol kind and declaration location.
- Existing function, method, struct, and field tests still pass.
