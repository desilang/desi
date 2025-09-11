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

## Building Desi Programs

The main workflow is through the `desic build` command, which compiles a `.desi` source file into C, then (optionally) invokes a C compiler to produce a native executable.

### Basic usage

```bash
# Compile a program (emits C + binary)
go run ./compiler/cmd/desic build examples/lex_demo.desi
```

This writes:

* `gen/out/lex_demo.c` — generated C
* `gen/out/lex_demo` (or `lex_demo.exe` on Windows) — executable

### Flags

* `--no-cc`
  Stop after emitting C (no native binary produced).

* `--cc-bin=<compiler>`
  Override C compiler (default: `clang`/`cc` depending on system).

* `--cc-arg=<flag>`
  Pass extra arguments to the C compiler.
  Repeatable, e.g. `--cc-arg=-O2 --cc-arg=-g`.

* `--out=<name>`
  Set custom output binary name (default: basename of `.desi` file).

* `--use-desi-lexer`
  Use the self-hosted Desi lexer via the bridge adapter instead of the Go lexer.

* `--verbose`
  Show detailed bridge/lexer debug output.

* `--Werror`
  Treat warnings as errors.

### Examples

```bash
# Generate only the C code (no native binary)
go run ./compiler/cmd/desic build --no-cc examples/parallel_demo.desi

# Compile with optimization flags
go run ./compiler/cmd/desic build --cc-bin=clang --cc-arg=-O2 examples/parallel_demo.desi

# Use the Desi lexer for parsing
go run ./compiler/cmd/desic build --use-desi-lexer examples/lex_demo.desi
```

On all platforms, runtime support (`runtime/c/desi_std.c` + `desi_std.h`) is automatically included and packaged — no manual copying is required.

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
* `desic build` will automatically pass `-I runtime/c` and link against `runtime/c/desi_std.c`.

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
  `.gitattributes` forces LF for source files. If you see EOL diffs, re-checkout with
  `git config core.autocrlf false` and clean.
