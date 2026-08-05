# `desi.mod`

The project manifest. `desic init` writes one, and `desic build`, `desic run`
and `desic test` read it to decide what to compile and how.

A single `.desi` file does not need a manifest — `desic run hello.desi` works on
its own. You want one as soon as a project has more than one file, because the
manifest is what tells the compiler where your modules live.

## What `desic init` generates

```toml
[package]
name    = "myapp"
version = "0.1.0"
edition = "2025"
entry   = "src/main.desi"
roots   = ["src"]

[build]
mode    = "debug"
out_dir = "build"

[target]
triple  = "native"

[diagnostics]
error_format = "human"
color        = "auto"
max_errors   = "100"

[ffi]
libs   = []
search = []
```

alongside this layout:

```
myapp/
├── desi.mod
├── src/
│   └── main.desi
├── tests/
│   └── main_test.desi
└── .gitignore
```

## `[package]`

| Key | Meaning |
|---|---|
| `name` | Project name. Also the default executable name. |
| `version` | Your project's version. Not the compiler's. |
| `edition` | Language edition the code targets. |
| `entry` | The file whose `main` runs. Changing it changes what `desic run` builds. |
| `roots` | **The one that matters most.** Directories searched to resolve `import`. |

`roots` is why a project needs a manifest at all. With `roots = ["src"]`, a file
at `src/util/math.desi` is reachable as `import util.math` from anywhere in the
project. Add a directory here and its contents become importable; leave one out
and its modules cannot be found, whatever the path on disk.

## `[build]`

| Key | Values | Meaning |
|---|---|---|
| `mode` | `debug`, `release` | `release` turns on `-O2` and link-time optimisation. Debug compiles faster and keeps diagnostics readable. |
| `out_dir` | path | Where executables and intermediates land. |

`desic build --release` overrides `mode` for one invocation, so the manifest
sets the default rather than the only option.

## `[target]`

| Key | Meaning |
|---|---|
| `triple` | `native` builds for the machine you are on. An explicit LLVM target triple cross-compiles. |

## `[diagnostics]`

| Key | Values | Meaning |
|---|---|---|
| `error_format` | `human`, `json` | `json` is for editors and CI that parse diagnostics. |
| `color` | `auto`, `always`, `never` | `auto` colours a terminal and stays plain when piped. |
| `max_errors` | number | Stops after this many, so one bad line does not print a thousand follow-on errors. |

## `[ffi]`

Links C libraries into your program. Empty unless you need it.

| Key | Meaning |
|---|---|
| `libs` | Libraries to link against, named without the `lib` prefix or file extension. |
| `search` | Extra directories to look for them in, relative to the project root or absolute. |

```toml
[ffi]
libs   = ["greet", "m"]
search = ["vendor"]
```

That links `greet` from `vendor/`, and the system math library. Declare the
functions with `@extern` and call them:

```desi
@extern("C", safe=true, c_name="greet_answer")
pub def answer() -> i32

def main() -> int:
	let v = answer()
	print(str(v))
	return 0
```

!!! note "The file your library has to be called"
    A name in `libs` is resolved by the platform's own convention, because
    `clang` is the linker on every target:

    - **Linux, macOS** — `greet` finds `libgreet.a` or `libgreet.so`
    - **Windows** — `greet` finds `greet.lib`

    A library built with Unix naming will not be found by a Windows build and
    the error names the file it wanted: `could not open 'greet.lib'`.

Search paths are emitted before library names, so a `search` entry applies to
every library in `libs`.

See [Safe FFI](../language/safe-ffi.md) for the language side — `@extern`,
`safe=true`, and what the compiler will and will not check for you.

## Editing it by hand

The manifest is plain TOML and meant to be edited. The changes worth knowing:

- **Adding a source directory** — add it to `roots`, or its modules will not be
  importable.
- **Renaming the entry point** — change `entry`; the file's location is not
  assumed.
- **Shipping a build** — set `mode = "release"`, or pass `--release`.
- **Linking a C library** — add it to `libs`, and its directory to `search` if
  it is somewhere the linker does not already look.
