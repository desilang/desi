# Import System - Internals

> **For Contributors**: How Desi's module resolution and import system works.

---

## Overview

Desi uses a file-based module system with dotted paths (like Python) and explicit exports (like Rust).

## Import Resolution Flow

```
1. Parse import statement → ast.ImportStmt / ast.FromImportStmt
2. resolve.Resolve() walks __top__ block
3. FSLoader.Load(dotted_path) finds and parses file
4. CollectExports() gathers exportable items
5. Graph.AddEdge() tracks dependencies
6. Graph.Cycles() detects circular imports → DME0008
```

## Loader Roots (FSLoaderMulti)

Search order:
1. **File's directory** - for relative imports (e.g., `from bar import greet`)
2. **User -I paths** - explicit include directories
3. **Stdlib** - `compiler/lib/`

## Circular Import Detection

### Implementation

```go
// resolve/resolve.go
func resolveImportsRecursive(mod, srcModule, ldr, info, diags, visited) {
    if visited[srcModule] { return }  // cycle detected
    visited[srcModule] = true
    // ... process imports, add edges to Graph
}
```

### Error Code: DME0008

```
error[DME0008] module: circular import detected
  = help: Circular imports are not allowed.
```

## Export Visibility

| Declaration | Exported? |
|-------------|-----------|
| `pub def foo()` | ✓ Yes |
| `def foo()` | ✗ No |
| `pub class Foo` | ✓ Yes |
| `class Foo` (top-level) | ✓ Yes (classes public by default) |
| Nested `class Inner` | ✗ No |
| `pub class Inner` | ✓ Yes |

## Key Files

| File | Purpose |
|------|---------|
| `resolve/resolve.go` | Recursive import resolution |
| `resolve/graph.go` | Dependency graph, cycle detection |
| `resolve/exports.go` | Collect exportable items |
| `resolve/loader_fs.go` | File system loader |
| `check/imports.go` | Inject imports into scope |
