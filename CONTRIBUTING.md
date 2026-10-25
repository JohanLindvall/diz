# Contributing to diz

Thanks for helping make Docker image archives easier to use. Documentation fixes, bug reproductions, compatibility reports, and code contributions are welcome.

## Choose a starting point

- Improve an example after trying it with your Docker setup.
- Reproduce a bug with a small public image or a synthetic archive fixture.
- Add coverage for archive layouts and error paths.
- Share measurements before proposing compression or memory-use changes.
- Browse [open issues](https://github.com/JohanLindvall/diz/issues) or [good first issues](https://github.com/JohanLindvall/diz/labels/good%20first%20issue).

For a substantial change, open an issue describing the use case and proposed behavior first. Small, focused fixes can go straight to a pull request.

## Development setup

Install Go 1.25 or newer, fork the repository, and clone your fork:

```sh
git clone https://github.com/YOUR-USERNAME/diz.git
cd diz
git switch -c describe-your-change
go build -o ./bin/diz .
```

Docker is needed for live image workflows, but **the unit tests do not require a Docker daemon**.

## Checks

```sh
make check
make race
```

`make check` verifies formatting and module tidiness, runs `go vet`, builds all packages, and runs the tests. `make race` runs the tests with Go's race detector. `make cover` reports coverage.

Without Make, use:

```sh
go mod tidy -diff
go vet ./...
go build ./...
go test ./...
go test -race ./...
```

Format changed Go files with `gofmt`. For documentation changes, check examples and links; Go tests are only needed if behavior changes. Markdown is checked in CI. An optional local equivalent uses Node.js 24 or newer and npm:

```sh
npx --yes markdownlint-cli2@0.23.2
```

## Find your way around

| Path | Responsibility |
| --- | --- |
| `main.go` | Command-line flags and dispatch |
| `app/` | List, create, update, restore, serve, and tag-file coordination |
| `diz/` | Image manifests, archive selection, and tar/ZIP conversion |
| `imagesource/` | Docker and ZIP image sources |
| `hashzip/` | ZIP entries, content hashes, and parallel compression |
| `registry/` | HTTP registry and archive-download endpoints |
| `dockerref/` | Image-reference parsing and normalization |
| `tagfile/` | Reference-file parsing and rewriting |
| `docs/` | User-facing command and format documentation |

## Send a pull request

Describe the problem, the resulting behavior, and how you checked the change. For a bug fix, add a regression test when practical. Use synthetic fixtures for unit tests so contributors can run them without Docker or registry access.

Keep performance claims tied to reproducible measurements. Compression changes should preserve raw DEFLATE compatibility and propagate failures to the caller. Changes to CLI behavior should update the [command reference](docs/cli.md).

The project uses the [MIT license](LICENSE). Participation follows the [Contributor Covenant](CODE_OF_CONDUCT.md). For vulnerabilities, use [private reporting](SECURITY.md) instead of a public issue.
