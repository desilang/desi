# CLI Reference

This document describes the `desic` command-line interface.

## Commands

### `desic watch` - Hot Reload Development Mode

Watches for `.desi` file changes and automatically rebuilds.

```bash
# Watch current directory
desic watch

# Watch specific directory
desic watch ./src

# Verbose mode (show directory count)
desic watch -v ./src

# Run program after successful build (future feature)
desic watch --run ./src
```

**Output Example:**

```
🔥 Watching for changes in: /path/to/project
   Press Ctrl+C to stop

📦 Building: main.desi
✅ Build succeeded (0.12s)

📝 Changed: server.desi
✅ Build succeeded (0.08s)

👋 Stopping watch...
```

**Features:**
- Recursive directory watching
- 100ms debounce for rapid saves
- Skips hidden directories (`.git`, `.vscode`, etc.)
- Skips `node_modules` and `__pycache__`
- Graceful Ctrl+C handling
- Auto-detects `main.desi` in target directory

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
