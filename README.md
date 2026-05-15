# fellow

`fellow` is a Fallow-inspired static analyzer for Go modules.

It builds a repo-wide view of `go.mod` requirements, Go packages, and Go source references, then reports dependency drift and dead code that file-local tools miss.

## What It Finds

- Unused direct `require` entries in `go.mod`
- External imports used in code but not declared in `go.mod`
- Dependencies used only from `*_test.go` files
- The same dependency being unused in one module but used by another module in the same repo
- Unused internal packages that are not reachable from module roots, `main` packages, or tests
- Unused files whose declarations are all unused
- Unused functions, methods, structs, and struct fields in reachable packages
- Unused interfaces, type aliases, package-level vars, and consts
- Complex functions and methods above cyclomatic or cognitive thresholds
- Duplicate code windows across Go files
- Read-only `go mod tidy -diff` drift
- Suspicious local `replace` directives that do not point at a discovered sibling module

Nested Go modules are analyzed independently, and imports from child modules are not attributed to parent modules.

## Usage

```bash
go run ./cmd/fellow --root .
go run ./cmd/fellow dead-code --format json
go run ./cmd/fellow audit --base main
go run ./cmd/fellow --ci
```

Flags:

- `--root`, `-r`: root directory to scan, default `.`
- `--config`, `-c`: config file path, default `<root>/.fellowrc.json` when present
- `--format`, `-f`: `human`, `json`, `sarif`, `codeclimate`, `gitlab-codequality`, or `annotations`, default `human`
- `--production`: exclude `*_test.go` files
- `--all-requires`: also check `// indirect` requirements for unused status
- `--ignore-generated`: skip files with generated-code headers
- `--summary`: print only counts in human output
- `--max-cyclomatic`: enable cyclomatic complexity findings above this threshold
- `--max-cognitive`: enable cognitive complexity findings above this threshold
- `--workspace`: comma-separated module path or directory filters
- `--tags`: comma-separated Go build tags
- `--coverage`: Go coverage profile to annotate findings
- `--base`, `--changed-since`: base ref for `audit`
- `--fail-on-issues`: exit with status 1 when findings exist
- `--ci`: equivalent to `--fail-on-issues`
- `--baseline`: suppress findings recorded in a baseline file
- `--save-baseline`: write current findings to a baseline file

## Notes

Go has no direct equivalent to JavaScript `dependencies` and `devDependencies`, so `fellow` treats direct `require` entries as the primary dependency contract. Indirect requirements are ignored for unused dependency findings unless `--all-requires` is set.

Module hygiene checks are read-only. `fellow` runs `go mod tidy -diff` and reports `tidy-drift` when Go would update `go.mod` or `go.sum`; it never runs mutating `go mod tidy`. Go `tool` directives and conventional `tools.go` imports count as intentional dependency usage.

Dead-code detection is a closed-world static analysis of the scanned repo. When packages type-check, `fellow` uses `go/packages` and `go/types` object identity for exact function, method, type, and field liveness. Tagged struct fields are treated as externally used to avoid obvious false positives with JSON/database/reflection-based code.

Complexity checks are opt-in. Set `--max-cyclomatic`, `--max-cognitive`, or the matching `health` config values to enable them.

## Configuration

`fellow` loads `.fellowrc.json` from the selected root when present. Use `--config <path>` to load a different file. CLI flags override config values.

```json
{
  "format": "json",
  "production": true,
  "workspace": ["github.com/acme/service"],
  "buildTags": ["integration"],
  "ignorePatterns": ["internal/generated/**"],
  "ignoreFindings": [
    {"type": "duplicate-code", "file": "**/*_test.go"},
    {"type": "complexity", "file": "internal/parser.go", "symbol": "parseLegacyFormat"}
  ],
  "health": {
    "maxCyclomatic": 20,
    "maxCognitive": 15
  },
  "rules": {
    "unused-function": "off",
    "unused-field": "warn"
  }
}
```

Rule severities are `off`, `warn`, and `error`. The current CLI treats `warn` and `error` as reportable findings; `off` disables matching findings.

Use `ignoreFindings` for known acceptable findings without ignoring every finding in a file. Each entry matches only the fields provided. `file` supports the same glob patterns as `ignorePatterns`; other fields match exactly, including `type`, `symbol`, `package`, `module`, `importPath`, `receiver`, `struct`, and `fingerprint`.

## Suppressions And Baselines

Use inline comments for intentional exceptions:

```go
// fellow-ignore-next-line unused-function
func RuntimeHook() {}

// fellow-ignore-file
```

Use baselines to adopt `fellow` incrementally:

```bash
go run ./cmd/fellow --save-baseline fellow-baseline.json
go run ./cmd/fellow --baseline fellow-baseline.json --fail-on-issues
```

## CI And Audit

Use `audit` to scope findings to files changed from a base ref:

```bash
go run ./cmd/fellow audit --base origin/main --format sarif
```

CI-oriented formats are available with `--format sarif`, `--format codeclimate`, `--format gitlab-codequality`, and `--format annotations`.

Coverage profiles from `go test -coverprofile=coverage.out ./...` can annotate matching findings:

```bash
go run ./cmd/fellow --coverage coverage.out --format json
```
