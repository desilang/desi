# desic Command-Line Tool

The `desic` command-line tool is your primary interface for working with Desi code.

## Commands

### `desic watch` - Development Mode 🔥

Watch for file changes and automatically rebuild/run.

!!! warning "macOS and Linux only"
    `desic watch` is not available on Windows. It relies on Unix signals
    (`SIGUSR1`) and shared-library (`.so`) reloading, neither of which has a
    Windows implementation yet. Running it there prints a message and exits
    rather than watching anything. Every other `desic` command — `run`,
    `build`, `test`, `check`, `fmt`, `doc` — works on all three platforms.

```bash
desic watch           # Watch and type-check only
desic watch --run .   # Watch, build, and run with auto-restart
desic watch --hot .   # True hot reload (no process restart)
desic watch -v .      # Verbose mode
```

**Modes:**

| Flag | Description |
|------|-------------|
| (none) | Type-check only on file change |
| `--run` | Rebuild and restart process on change |
| `--hot` | Recompile `.so`, reload via `desi-host` (no restart) |

**State Serialization Builtins (for `--run` mode):**

```desi
if is_reload():
    let saved = read_state()
    if saved is Some(json):
        restore_from_json(json)
        delete_state()
else:
    print("First run")
```

| Function | Description |
|----------|-------------|
| `is_reload()` | `true` if this is a reload |
| `reload_count()` | Number of reloads (0 = first) |
| `write_state(json)` | Save state to temp file |
| `read_state()` | Load state from temp file |

Press `Ctrl+C` to stop watching.

---

### `desic build` - Compile to an Executable

```bash
desic build hello.desi              # -> build/output/hello
desic build hello.desi -o hello     # -> ./hello
desic build hello.desi -o dist/app  # -> dist/app
desic build hello.desi -o hello -O2 # optimized
```

`-o` is a path, as it is for `cc`, `go` and `rustc`: the executable is
written exactly where you name it, and any directories in the path are
created. Without `-o`, the output goes to `build/output/` named after the
source file.

On Windows a `.exe` extension is appended when the path has none, since
Windows will not run a file without it.

Run without a file argument to build the project described by `desi.mod`.

### `desic run` - Compile and Run

```bash
desic run hello.desi
desic run hello.desi -- arg1 arg2   # arguments after -- go to the program
```

Compiles to a temporary location and executes it; nothing is left behind.

### `desic check` - Type Check

Verify your code without compiling:

```bash
desic check main.desi
```

---

### `desic fmt` - Format Code

Auto-format your Desi code with consistent style:

```bash
desic fmt -w main.desi     # Write in place
desic fmt -l ./src         # List files to change
desifmt main.desi          # Standalone formatter (stdout)
desifmt -w main.desi       # Standalone formatter (in-place)
```

**What the formatter does:**

- Uses **tabs** for indentation
- Preserves **standalone comments** and **trailing comments**
- Preserves **string literals** exactly (quotes, escapes, f-strings)
- Normalizes spacing around operators and keywords

---

### `desic doc` - Generate Docs

Create Markdown documentation:

```bash
desic doc main.desi        # Public declarations
desic doc --all main.desi  # All declarations
```

---

## Quick Start

```bash
# Create a file
echo 'print("Hello!")' > hello.desi

# Check it
desic check hello.desi

# Start developing with auto-reload
desic watch .
```

---

## See Also

- [Getting Started](../getting-started/install.md)
- [Language Reference](../language/types.md)
