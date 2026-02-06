# CLI Reference

This document describes the `desic` command-line interface.

## Commands

### `desic watch` - Hot Reload Development Mode

Watches for `.desi` file changes and automatically rebuilds/runs.

```bash
# Watch and type-check only
desic watch

# Watch and run the program (hot reload!)
desic watch --run ./src

# Verbose mode (shows compile steps)
desic watch --run -v ./src
```

**With `--run` flag (full hot reload):**

```
🔥 Watching for changes in: /path/to/project
   Press Ctrl+C to stop

📦 Building: main.desi
  📝 Emitting IR...
  🔨 Compiling to object code...
  🔗 Linking...
✅ Build succeeded (0.31s)
🚀 Running...
─────────────────────────────────────
Server started on port 8080

📝 Changed: handler.desi
  🔄 Stopping previous process...
✅ Build succeeded (0.09s)
🚀 Running...
─────────────────────────────────────
Server started on port 8080 (v2 - hot reloaded!)
```

**Features:**
- Recursive directory watching
- 100ms debounce for rapid saves
- Skips hidden directories (`.git`, `.vscode`, etc.)
- Skips `node_modules` and `__pycache__`
- Graceful Ctrl+C handling
- Auto-detects `main.desi` in target directory
- **`--run`**: Full compile + link + execute with auto-restart

---

### `desic check` - Type Check

Parses and type-checks a Desi file without compiling.

```bash
desic check main.desi
desic check -I examples:lib main.desi
```

---

### `desic fmt` - Format Code

Formats Desi source files.

```bash
# Print formatted code to stdout
desic fmt main.desi

# Write formatted code in place
desic fmt -w main.desi

# List files that would be changed
desic fmt -l ./src

# Quiet mode (no output on success)
desic fmt -w -q ./src
```

---

### `desic doc` - Generate Documentation

Generates Markdown documentation from Desi source files.

```bash
# Generate docs for public declarations
desic doc main.desi

# Include private declarations
desic doc --all main.desi
```

---

## Global Flags

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Verbose output |
| `--version` | Print version and exit |
| `--error-format=human\|json` | Error output format |
| `--color=auto\|always\|never` | Color output mode |

---

## Getting Started

```bash
# Create a new Desi file
echo 'print("Hello, Desi!")' > hello.desi

# Check syntax
desic check hello.desi

# Start development with auto-reload
desic watch .
```

---

## See Also

- [Hot Reload Design](../roadmap/hot_reload_design.md) - Architecture and roadmap
- [Language Reference](../language/) - Desi language guide
