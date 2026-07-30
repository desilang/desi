# Contributing to Desi

Thanks for taking an interest. This file covers how to build the project, how
to test a change, and the few conventions worth knowing before you open a pull
request.

## Building

```sh
# macOS / Linux — installs any missing prerequisites, then builds
./bootstrap.sh

# Windows
.\bootstrap.ps1
```

If you already have Go, LLVM/Clang, OpenSSL and Make, plain `make` is enough.
On Windows use `.\build.ps1`, which needs the MSVC toolchain from the Visual
Studio Build Tools "Desktop development with C++" workload.

The compiler is Go; the runtime (`compiler/runtime/`) is C. `make runtime`
rebuilds only the C side, `make compiler` only the Go side.

> The standard library in `compiler/lib/*.desi` is **embedded into the compiler
> binary** with `go:embed`. Editing it requires `make compiler`, not
> `make runtime` — a rebuild of the runtime alone will appear to do nothing.

## Testing

Three suites, all of which should pass before you push:

```sh
go test ./...          # compiler unit tests
./test_examples.sh     # every program in examples/, against expected output
DESI_RELEASE=1 ./test_examples.sh   # the same, at -O2
```

On Windows use `.\test_examples_parallel.ps1`, which shards across cores and
honours `$env:DESI_RELEASE`.

Both optimization levels matter. `-O2` exploits undefined behaviour that `-O0`
tolerates, so the release leg catches miscompiles the debug leg hides — several
real bugs were found exactly that way.

Only run one suite at a time in a given checkout. They share `build/program.ll`
and will overwrite each other.

### Database tests

Ten examples exercise the ORM against a real server. They skip unless you opt
in:

```sh
export PG_HOST=127.0.0.1 PG_USER=postgres PG_PASS=... PG_DB=postgres
export MYSQL_HOST=127.0.0.1 MYSQL_USER=root MYSQL_PASS=... MYSQL_DB=desi_test
DESI_DB_TESTS=1 ./test_examples.sh
```

## Adding an example

Examples are the main regression suite. Each is a program in `examples/`
numbered in sequence, with its expectation in a header comment:

```desi
# EXPECTED_OUTPUT:
# 42
def main() -> int:
	print(str(42))
	return 0
```

Other headers: `# EXPECTED: COMPILE_ERROR`, `# EXPECTED: RUNTIME_ERROR`, and
`# REQUIRES: database` for the opt-in set above.

When you fix a bug, add an example that fails without the fix. Say in the
header what used to go wrong — a future reader needs to know what the guard is
guarding.

## Conventions

- **Comments explain why, not what.** The line above already says what it does.
- **Platform-specific code is additive and guarded.** Never remove or degrade a
  working path for another platform; use `#ifdef _WIN32`, `runtime.GOOS`, or a
  `uname` check and leave the existing branch alone.
- **Don't commit build artifacts** — `bin/`, `build/`, `*.o`, `*.obj`, `*.a`,
  `*.lib`, `*.exe`.
- **Commit messages** explain the problem being solved, not just the change.
  If a fix has a subtlety, or you tried an approach that did not work, write
  that down; it saves the next person from repeating it.

## Documentation is code too

The Desi blocks in `book/docs` are compiled, not eyeballed. `tools/docaudit`
extracts every one and checks it — see its README. A block that is illustrative
rather than runnable (a placeholder, an API signature listing) should be tagged
` ```text ` so it is not presented as something that compiles.

Use that tooling rather than looping over `desic` yourself: it is a native
binary, Git Bash's `timeout` does not reliably kill one, and a pathological
input can otherwise take the machine down.

## Reporting a bug

Include the program, the command you ran, what happened, and what you expected.
The smallest program that still shows the problem is far more useful than a
large one — most bugs in this repo were fixed the moment they were reduced.
