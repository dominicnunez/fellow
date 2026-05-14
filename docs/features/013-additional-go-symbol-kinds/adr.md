# ADR 013: Additional Go Symbol Kinds

Status: Proposed

## Context

`fellow` currently reports several declaration kinds but omits interfaces, type aliases, package vars, consts, and enum-like groups.

## Decision

Extend the existing typed liveness model to more package-level declarations. Treat package vars with side-effectful initializers conservatively.

## Consequences

- Dead-code coverage becomes more complete.
- More rules require careful false-positive handling.
- Source deletion remains out of scope.

## Verification

Add fixtures for interfaces, aliases, vars, consts, iota groups, and side-effectful initializers.
