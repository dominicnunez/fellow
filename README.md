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

Nested Go modules are analyzed independently, and imports from child modules are not attributed to parent modules.

## Usage

```bash
go run ./cmd/fellow --root .
go run ./cmd/fellow dead-code --format json
go run ./cmd/fellow --ci
```

Flags:

- `--root`, `-r`: root directory to scan, default `.`
- `--config`, `-c`: config file path, default `<root>/.fellowrc.json` when present
- `--format`, `-f`: `human` or `json`, default `human`
- `--production`: exclude `*_test.go` files
- `--all-requires`: also check `// indirect` requirements for unused status
- `--ignore-generated`: skip files with generated-code headers
- `--summary`: print only counts in human output
- `--fail-on-issues`: exit with status 1 when findings exist
- `--ci`: equivalent to `--fail-on-issues`

## Notes

Go has no direct equivalent to JavaScript `dependencies` and `devDependencies`, so `fellow` treats direct `require` entries as the primary dependency contract. Indirect requirements are ignored for unused dependency findings unless `--all-requires` is set.

Dead-code detection is a closed-world static analysis of the scanned repo. When packages type-check, `fellow` uses `go/packages` and `go/types` object identity for exact function, method, type, and field liveness. Tagged struct fields are treated as externally used to avoid obvious false positives with JSON/database/reflection-based code.

## Configuration

`fellow` loads `.fellowrc.json` from the selected root when present. Use `--config <path>` to load a different file. CLI flags override config values.

```json
{
  "format": "json",
  "production": true,
  "ignorePatterns": ["internal/generated/**"],
  "rules": {
    "unused-function": "off",
    "unused-field": "warn"
  }
}
```

Rule severities are `off`, `warn`, and `error`. The current CLI treats `warn` and `error` as reportable findings; `off` disables matching findings.
