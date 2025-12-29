# Import System - Internals

> **For Contributors**: How Desi's module resolution and import system works.

---

## Overview

Desi uses a file-based module system with dotted paths (like Python) and explicit exports (like Rust).

## Import Resolution Flow

```
1. Parse import statement → ast.ImportStmt / ast.FromImportStmt
2. resolve.Resolve() walks __top__ block
3. Check for std.* prefix → LoadStdlib() if present
4. Check for relative import (.) → resolve from file dir
5. FSLoader.Load(dotted_path) finds and parses file
6. CollectExports() gathers exportable items
7. Graph.AddEdge() tracks dependencies
8. Graph.Cycles() detects circular imports → DME0008
```

## Loader Roots (FSLoaderMulti)

Search order:
1. **File's directory** - for relative imports (e.g., `from .bar import greet`)
2. **User -I paths** - explicit include directories
3. **Stdlib** - `compiler/lib/` (stdlibIdx marks where stdlib starts)

## Syntax

| Import | Meaning |
|--------|---------|
| `from math import add` | Local-first (file dir → project → stdlib) |
| `from std.math import add` | **Always stdlib**, skips local lookup |
| `from .math import add` | **Relative** to current file |

## Error Codes

| Code | Message | When |
|------|---------|------|
| DME0008 | Circular import detected | A → B → A |
| DME0009 | Reserved namespace | `import std` |
| DME0010 | Module shadows stdlib | Local `math.desi` + stdlib `math` |
| DME0011 | Ambiguous module | Both `bar.desi` and `bar/` exist |

## Key Files

| File | Purpose |
|------|---------|
| `resolve/resolve.go` | Recursive import resolution, loadModule() |
| `resolve/loader.go` | StdlibLoader interface, multiLoader |
| `resolve/loader_fs.go` | Filesystem loader, ambiguity check |
| `resolve/graph.go` | Dependency graph, cycle detection |
| `resolve/exports.go` | Collect exportable items |

## Known Limitations

- **Stdlib path is relative** (`compiler/lib`) - only works from project directory
- Future: embed stdlib or resolve relative to binary
