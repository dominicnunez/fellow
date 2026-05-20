# Plan 015: Module Hygiene

Status: Draft

## Goal

Implement the first low-risk module hygiene slice from RFC 015:

- Run `go mod tidy -diff` read-only and report `tidy-drift`.
- Report suspicious local `replace` directives as `local-replace`.
- Treat Go `tool` directives and conventional `tools.go` imports as intentional dependency usage.

## Non-Goals

- Do not mutate `go.mod` or `go.sum`.
- Do not classify every tidy diff hunk as `missing-sum` or `stale-sum`.
- Do not implement unused non-local `replace` detection.
- Do not add build-tag/platform matrixing beyond existing dependency checks.
- Do not add auto-fix behavior.

## Files To Change

- `internal/analyzer/analyzer.go`
  - Add new finding type constants: `tidy-drift`, `local-replace`, and `tool-dependency`.
  - Add summary counters if we want per-type counts, or keep them under total findings only for the first slice.
  - Call module hygiene analysis during `analyzeModule` after module metadata is loaded.

- `internal/analyzer/dependencies.go` or new `internal/analyzer/module_hygiene.go`
  - Prefer a new `module_hygiene.go` file to keep module-maintenance logic separate from existing import/require drift logic.
  - Implement tidy drift detection.
  - Implement local replace detection.
  - Implement tool dependency extraction/classification helpers.

- `internal/analyzer/deadcode.go`
  - Update package/import usage collection only if conventional `tools.go` handling needs source-file classification beyond existing test/build-tag metadata.

- `internal/analyzer/baseline.go`
  - No expected schema change because new findings use existing `Finding` fields and fingerprints.
  - Verify baselines suppress new finding types correctly.

- `internal/settings/settings.go`
  - No new config fields expected for the first slice.
  - Existing `rules` and `ignoreFindings` should handle new finding types.

- `cmd/gallow/main.go`
  - Add human output writers for `tidy-drift`, `local-replace`, and `tool-dependency`.
  - Verify JSON, SARIF, CodeClimate, GitLab Code Quality, and annotations work through generic finding serialization.

- `internal/analyzer/analyzer_test.go`
  - Add fixtures for tidy drift, local replace, sibling module replace, Go tool directive usage, and `tools.go` usage.

- `cmd/gallow/main_test.go`
  - Add or update serializer tests if new human output or CI formatting needs explicit coverage.

- `README.md`
  - Document module hygiene finding types and read-only tidy behavior after implementation.

## Data Model

Use existing `Finding` fields where possible.

### `tidy-drift`

- `Type`: `tidy-drift`
- `Module`: module path
- `File`: `go.mod`, `go.sum`, or `go.mod` if changed-file parsing fails
- `Line`: `1`
- `Locations`: optional changed files from tidy diff

### `local-replace`

- `Type`: `local-replace`
- `Module`: module path being replaced
- `File`: module-relative `go.mod`
- `Line`: replace directive line
- `ImportPath`: replacement module path if useful

### `tool-dependency`

- `Type`: `tool-dependency`
- `Module`: module path or tool package path involved
- `File`: module-relative `go.mod` or `tools.go`
- `Line`: directive/import line when available

Tool dependencies that are valid should not produce findings. They should only mark requirements as intentionally used.

## Tidy Drift Algorithm

1. For each selected module, run `go mod tidy -diff` with working directory set to the module directory.
2. Capture stdout and stderr.
3. If exit code is `0`, emit no finding.
4. If exit code is non-zero and stdout contains unified diff content, emit `tidy-drift`.
5. Parse changed module files from diff headers when possible:
   - `+++ b/go.mod`
   - `+++ b/go.sum`
6. If changed files are parsed, add them as `Locations`.
7. If no changed file can be parsed, report one module-level `tidy-drift` at `go.mod:1`.
8. If the command fails without diff output, return an analysis error rather than guessing a hygiene finding.

Implementation notes:

- Use `exec.Command("go", "mod", "tidy", "-diff")`.
- Set `cmd.Dir` to the module directory.
- Do not pass commands through a shell.
- Do not run `go mod tidy` without `-diff`.
- Keep stdout diff content out of the first finding model unless needed later.

## Local Replace Algorithm

1. Parse `go.mod` using `golang.org/x/mod/modfile`.
2. For each `replace` directive, inspect `Replace.New.Path`.
3. Treat a replacement as local when it is an absolute path, a relative path, or begins with `.` / `..`.
4. Resolve relative replacements against the module directory.
5. Compare the resolved path against known module directories discovered under the selected root.
6. If the replacement points to a known sibling or workspace module, emit no finding.
7. Otherwise emit `local-replace` at the directive line.

Open implementation detail:

- `modfile.Replace` has syntax position data. Use it for line numbers if available; otherwise report `go.mod:1`.

## Tool Dependency Algorithm

### Go `tool` Directives

1. Parse `go.mod`.
2. Collect declared tool package paths from tool directives when supported by the `x/mod` version in use.
3. Map tool package paths to their module roots where possible using existing module-path matching or resolved requirements.
4. Mark matching requirements as intentionally used so they do not produce `unused-dependency` findings.
5. Emit `tool-dependency` only for malformed or unresolved tool declarations that can be detected from parsed module state.

### Conventional `tools.go`

1. During source scanning, detect files conventionally used for tool dependencies:
   - filename `tools.go`, or
   - build tags containing `tools`.
2. Treat blank imports from those files as intentional dependency usage.
3. Do not count those imports as production application usage.
4. Ensure requirements used only by tool files are not reported as unused or test-only application dependencies.

## Test Plan

- Tidy drift in `go.mod`:
  - Fixture starts with untidy `go.mod`.
  - Analysis reports `tidy-drift`.
  - Test verifies `go.mod` contents are unchanged after analysis.

- Tidy drift in `go.sum`:
  - Fixture starts with stale or missing sum state.
  - Analysis reports `tidy-drift`.
  - Test verifies `go.sum` contents are unchanged after analysis.

- Suspicious local replace:
  - `replace example.com/lib => ../scratch/lib`
  - Analysis reports `local-replace`.

- Known sibling replace:
  - Root contains `app/go.mod` and `lib/go.mod`.
  - `app` replaces `example.com/lib => ../lib`.
  - Analysis does not report `local-replace`.

- Go tool directive usage:
  - Requirement exists only for a declared tool.
  - Analysis does not report `unused-dependency`.

- Conventional `tools.go` usage:
  - Requirement exists only through blank import in `tools.go`.
  - Analysis does not report `unused-dependency`.

- Serialization:
  - New finding types appear in human output.
  - New finding types serialize through JSON and CI formats.

## Verification Commands

Run before committing implementation:

```bash
go test ./...
go vet ./...
golangci-lint run
go run ./cmd/gallow --root . --summary
go run ./cmd/gallow audit --root . --base origin/main --summary
```

## Rollout Notes

- Keep the feature read-only from the start.
- If tidy drift introduces environment-dependent failures, prefer returning a clear analysis error over emitting partial findings.
- Do not add auto-fix hooks until RFC 005 is explicitly implemented.
