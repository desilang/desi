# desic Command-Line Tool

The `desic` command-line tool is your primary interface for working with Desi code.

## Commands

### `desic watch` - Development Mode 🔥

Watch for file changes and automatically rebuild/run.

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
    match read_state():
        case Some(json):
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

### `desic check` - Type Check

Verify your code without compiling:

```bash
desic check main.desi
```

---

### `desic fmt` - Format Code

Auto-format your Desi code:

```bash
desic fmt -w main.desi     # Write in place
desic fmt -l ./src         # List files to change
```

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

- [Getting Started](../getting-started/installation.md)
- [Language Reference](../language/syntax.md)
