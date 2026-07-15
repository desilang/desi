# Building Desi from Source

## Quick Start (Windows)

Run the one-file bootstrap — it installs anything missing, then builds:

```powershell
.\bootstrap.ps1
```

Flags: `-Test` (run examples), `-SkipBuild` (just set up tools), `-Clean` (fresh build), `-NoElevate` (skip UAC; admin only needed for Build Tools install).

## Quick Start (macOS / Linux)

Run the one-file bootstrap — it installs anything missing, then builds:

```sh
./bootstrap.sh
```

Flags: `--test` / `-t` (run examples), `--skip-build` / `-s` (just set up tools), `--clean` / `-c` (fresh build), `--no-elevate` / `-n` (skip sudo).

Alternatively, if you already have all prerequisites installed:

```sh
make
```

## Prerequisites

| Tool | Version | macOS / Linux | Windows |
|------|---------|---------------|---------|
| **Go** | 1.21+ | `brew install go` / distro package | `winget install GoLang.Go` or [go.dev/dl](https://go.dev/dl/) |
| **LLVM / Clang** | 15+ | `brew install llvm` / distro package | `winget install LLVM.LLVM` or [releases.llvm.org](https://releases.llvm.org/) |
| **C compiler** | — | Clang (comes with LLVM) | Visual Studio 2022 Build Tools — *Desktop development with C++* workload (`cl.exe`, `lib.exe`) |
| **Make** | — | Pre-installed | Not needed (`build.ps1` replaces it) |

`bootstrap.ps1` detects what you already have and only installs what's missing (winget first, direct download as fallback).

## Build Commands

### Windows

```powershell
.\build.ps1              # build runtime + all Go tools
.\build.ps1 -Clean       # remove artifacts first
.\build.ps1 -SkipRuntime # rebuild only the Go binaries
```

### macOS / Linux

```sh
make                     # build runtime + all Go tools
make clean && make       # from scratch
```

### Output

```
bin/desic(.exe)           # compiler
bin/desifmt(.exe)         # formatter
bin/desirepl(.exe)        # REPL
bin/desilsp(.exe)         # language server
build/libdesi.a (.lib)    # C runtime library
```

## Compiling a Desi Program

```powershell
# Windows
.\build-desi.ps1 examples\000_hello_world.desi

# macOS / Linux
./build-desi.sh examples/000_hello_world.desi
```

This runs the compiler pipeline: Desi source → LLVM IR → object code (via clang) → linked binary.

## Running the Test Suite

```powershell
# Go unit tests (all platforms)
go test ./...

# Example programs (Windows)
.\test_examples.ps1

# Example programs (macOS / Linux)
./test_examples.sh
```

The example suite compiles and runs ~480 programs, comparing output against expected baselines.

## How the Build Works

**Runtime library** — `compiler/runtime/*.c` compiled to object files, merged with the bundled mpdecimal library into a single static archive (`libdesi.a` on Unix, `libdesi.lib` on Windows).

**Go binaries** — Standard `go build` for each tool (`desic`, `desifmt`, `desirepl`, `desilsp`).

**User programs** — The compiler emits LLVM IR, then shells out to `clang` to assemble and link against the runtime library.

### Windows-specific notes

- MSVC `cl.exe` compiles the C runtime (not clang) because the Windows SDK headers expect it.
- Two files are excluded on Windows: `desi_host.c` (Mach-O introspection) and `signal_handler.c` (POSIX signals; will be ported to `SetConsoleCtrlHandler`).
- The `compiler/runtime/db/` directory is excluded on all platforms from `build.ps1` and is a separate make target on Unix. It requires database client libraries.

## Detailed Platform Docs

- [Windows build internals](docs/contributing/windows-build.md) — platform portability rules, MSVC shims, ABI gotchas
- [Drop/ownership implementation](docs/contributing/runtime/drops-implementation.md) — hybrid memory management pipeline
- [Contributing guide](CONTRIBUTING.md)
