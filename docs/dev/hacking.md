# Hacking on Desi (Stage-0)

This doc is a quick start for contributors on macOS, Windows, and Linux.

## Prerequisites
- Go **1.25.x**
- Git + GitHub account
- (Later) LLVM/Clang 17+ — only needed once we start compiling emitted C
- Python 3.12 (optional; future docs tooling)

### Verify Go
```bash
go version
go env GOPATH
```

## Build & Test

From the repo root:

```bash
# build the CLI
go build ./compiler/cmd/desic

# run it
go run ./compiler/cmd/desic --help

# run tests
go test ./...
```

## GoLand (JetBrains)

1. **Open** the repo folder.
2. **Settings → Go → GOROOT**: select Go 1.25.x.
3. **Run configuration**:

  * Kind: *Go Build / Run*
  * Package path: `compiler/cmd/desic`
  * Program args: `--help`
4. Run ▶️.

## Windows notes

* When we start emitting C, install **LLVM for Windows** and ensure:

  ```powershell
  clang --version
  ```
* We’ll call `clang` from `desic build` to link emitted C with the runtime.

## Coding workflow

* Small, focused PRs.
* Update **docs/spec** with any user-visible change.
* Add/adjust tests under `tests/*`.

## Make targets (coming soon)

We’ll add a Makefile for convenience:

```
make build   # builds desic + runtime
make test    # runs all tests
make fmt     # formatting/lint
make docs    # serve docs locally
```

## Troubleshooting

* **`go: module github.com/... not found`**
  Ensure `go.mod` module path matches your GitHub repo and imports use the same prefix.
* **Line endings on Windows**
  `.gitattributes` forces LF for source files. If you see EOL diffs, re-checkout with `git config core.autocrlf false` and clean.
