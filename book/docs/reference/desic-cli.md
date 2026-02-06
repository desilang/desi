# desic Command-Line Tool

The `desic` command-line tool is your primary interface for working with Desi code.

## Commands

### `desic watch` - Development Mode 🔥

Watch for file changes and automatically rebuild/run.

```bash
desic watch           # Watch and type-check only
desic watch --run .   # Watch, build, and run with hot reload!
desic watch -v .      # Verbose mode
```

**Hot Reload Example:**

```
🔥 Watching for changes in: /my-project
📦 Building: main.desi
✅ Build succeeded (0.31s)
🚀 Running...
─────────────────────────────────────
Hello from my Desi program!

📝 Changed: main.desi
✅ Build succeeded (0.09s)
🚀 Running...
─────────────────────────────────────
Hello from v2 - hot reloaded!
```

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
