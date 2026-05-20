# Releasing

Gallow currently uses Go's module proxy and Git tags for releases.

Before tagging:

```bash
go fmt ./...
go test ./...
go vet ./...
go run ./cmd/gallow --root . --summary --ci
```

Tag the first release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

After the tag is available, users can install the released CLI with:

```bash
go install github.com/dominicnunez/go-fallow/cmd/gallow@v0.1.0
```

Future release work can add GitHub Release binaries and GoReleaser once the project needs packaged artifacts beyond `go install`.
