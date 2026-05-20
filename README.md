# Gallow

Fallow-style codebase intelligence for Go.

[![CI](https://github.com/dominicnunez/gallow/actions/workflows/ci.yml/badge.svg)](https://github.com/dominicnunez/gallow/actions/workflows/ci.yml)
![Go version](https://img.shields.io/github/go-mod/go-version/dominicnunez/gallow)
![License](https://img.shields.io/github/license/dominicnunez/gallow)

Gallow is a Fallow-style codebase intelligence tool for Go. It analyzes Go modules for dependency drift, unused code, module hygiene issues, complexity hotspots, duplicate code, and CI-ready quality signals.

Gallow is inspired by Fallow's codebase-intelligence model. It is not the official Fallow project, and it is built specifically for Go modules.

## Why Gallow?

Go already has excellent quality tools. Gallow is intended to sit beside them by combining repo-wide hygiene signals into one workflow, especially for module drift, dead code, baselines, audit output, and CI adoption.

| Tool | Primary job | Where Gallow fits |
| --- | --- | --- |
| `go vet` | Correctness and suspicious code | Gallow adds repo-wide dependency, dead-code, hygiene, and reporting checks. |
| `staticcheck` | Go static analysis | Gallow focuses on codebase-level drift and adoption workflows rather than individual diagnostics. |
| `go mod tidy` | Module cleanup | Gallow runs read-only `go mod tidy -diff` and reports drift without mutating files. |
| `golangci-lint` | Lint aggregation | Gallow is a focused analyzer with baselines, audit mode, and CI output formats. |
| Gallow | Repo-wide dependency drift, dead code, module hygiene, baselines, CI and audit reporting | Use it when you want one conservative pass over module and codebase health. |

## Features

- Unused direct `require` entries in `go.mod`.
- External imports used in code but missing from `go.mod`.
- Dependencies used only from `*_test.go` files.
- Unused internal packages, files, functions, methods, structs, fields, interfaces, type aliases, vars, and consts.
- Complexity findings above configured cyclomatic or cognitive thresholds.
- Duplicate code windows across Go files.
- Read-only `go mod tidy -diff` drift detection.
- Suspicious local `replace` directives that do not point at a discovered sibling module.
- Conventional Go `tool` directives and `tools.go` imports.
- Nested Go module analysis with workspace filters and build tags.
- Baselines, inline suppressions, coverage annotation, audit mode, and CI formats.

## Quickstart

Install the CLI:

```bash
go install github.com/dominicnunez/gallow/cmd/gallow@latest
```

Run it from a Go module:

```bash
gallow --root .
gallow --summary
gallow --format json
gallow --ci
gallow audit --base origin/main --format sarif
```

When working from a checkout of this repository, use:

```bash
go run ./cmd/gallow --root . --summary
```

## Usage

```bash
gallow [dead-code|audit] [flags] [root]
```

Common flags:

- `--root`, `-r`: root directory to scan, default `.`.
- `--config`, `-c`: config file path.
- `--format`, `-f`: `human`, `json`, `sarif`, `codeclimate`, `gitlab-codequality`, or `annotations`, default `human`.
- `--production`: exclude `*_test.go` files.
- `--all-requires`: also check `// indirect` requirements for unused status.
- `--ignore-generated`: skip files with generated-code headers.
- `--summary`: print only counts in human output.
- `--max-cyclomatic`: enable cyclomatic complexity findings above this threshold.
- `--max-cognitive`: enable cognitive complexity findings above this threshold.
- `--workspace`: comma-separated module path or directory filters.
- `--tags`: comma-separated Go build tags.
- `--coverage`: Go coverage profile to annotate findings.
- `--base`, `--changed-since`: base ref for `audit`.
- `--fail-on-issues`: exit with status 1 when findings exist.
- `--ci`: equivalent to `--fail-on-issues`.
- `--baseline`: suppress findings recorded in a baseline file.
- `--save-baseline`: write current findings to a baseline file.

## Configuration

Gallow loads `.gallowrc.json` from the selected root when present. Use `--config <path>` to load a different file. CLI flags override config values.

```json
{
  "format": "json",
  "production": true,
  "workspace": ["github.com/acme/service"],
  "buildTags": ["integration"],
  "ignorePatterns": ["internal/generated/**"],
  "ignoreFindings": [
    {"type": "duplicate-code", "file": "**/*_test.go"},
    {"type": "complexity", "file": "internal/parser.go", "symbol": "parseArchiveFormat"}
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

## Example Output

Human output is compact:

```text
example.com/app (.)
  unused dependency github.com/acme/old v0.4.0 at go.mod:12
  go mod tidy drift at go.mod:1
  complex function example.com/app/internal/api.Handle at internal/api/handler.go:42 (cyclomatic 22, cognitive 31)

Summary
  modules: 1
  findings: 3
  unused dependencies: 1
  complexity findings: 1
```

GitHub annotation output includes warning severity:

```text
::warning file=go.mod,line=12::unused-dependency: github.com/acme/old
::warning file=go.mod,line=1::tidy-drift
::warning file=internal/api/handler.go,line=42::complexity: Handle
```

Suggested actions are currently documented by finding type rather than emitted as a separate machine-readable field.

| Finding type | Suggested action |
| --- | --- |
| `unused-dependency` | Remove the unused requirement or confirm it is intentionally kept. |
| `unlisted-dependency` | Add the dependency with `go get` or remove the import. |
| `tidy-drift` | Run `go mod tidy` and review the diff. |
| `local-replace` | Remove the local replace or point it at a checked-in sibling module. |
| `complexity` | Split or simplify the function if the threshold reflects your team policy. |
| `duplicate-code` | Consider extracting shared logic when the duplicated block is intentional code, not test setup. |

## CI Usage

Gallow supports `json`, `sarif`, `codeclimate`, `gitlab-codequality`, and GitHub `annotations` output.

Run the same command locally and in CI:

```bash
gallow --root . --summary --ci
```

Use audit mode to scope findings to changed files:

```bash
gallow audit --base origin/main --format sarif
```

Coverage profiles from `go test -coverprofile=coverage.out ./...` can annotate matching findings:

```bash
gallow --coverage coverage.out --format json
```

## Baselines And Suppressions

Use inline comments for intentional exceptions:

```go
// gallow-ignore-next-line unused-function
func RuntimeHook() {}

// gallow-ignore-file
```

Use baselines to adopt Gallow incrementally:

```bash
gallow --save-baseline gallow-baseline.json
gallow --baseline gallow-baseline.json --fail-on-issues
```

## Known Limitations

Dead-code analysis in Go is conservative and difficult. Gallow prefers conservative findings over noisy false positives.

Areas that can affect precision include reflection, generated code, build tags, exported APIs, framework conventions, interface usage, test-only references, and `//go:linkname`.

## Roadmap

- Richer SARIF and code scanning integration.
- Improved dead-code confidence levels.
- Better monorepo and workspace support.
- LSP and MCP integration.
- More precise generated-code handling.
- Release binaries through GitHub Releases.

## Contributing

Before opening a change, run:

```bash
go fmt ./...
go test ./...
go vet ./...
go run ./cmd/gallow --root . --summary --ci
```

Keep changes small and verify README examples against implemented flags.

## License

Gallow is available under the MIT License. See `LICENSE`.
