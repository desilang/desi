# sys Module Implementation

The `sys` module provides access to system streams (stdout, stderr).

## Architecture

Like other stdlib modules, sys requires `import sys`:

```
import sys
    ↓
StdlibImports["sys"] = true  (check/stmt.go)
    ↓
Type check: sys.stderr → ptr  (check/expr_call.go)
    ↓
Lower: __get_stderr()  (lower/lower_expr.go)
    ↓
LLVM emit: call ptr @__get_stderr()
```

## Files

| File | Purpose |
|------|---------|
| `compiler/lib/sys/sys.desi` | Module definition with pub extern |
| `compiler/internal/check/stmt.go:839` | Registers sys in StdlibImports |
| `compiler/internal/check/expr_call.go:475-489` | Import check + usage tracking |
| `compiler/internal/lower/lower_expr.go:1281-1291` | Lowering to __get_stdout/__get_stderr |

## Import Check

In `check/expr_call.go`, sys.stdout/stderr are handled specially:

```go
if id, ok := fe.X.(*ast.Ident); ok && id.Name == "sys" {
    if fe.Name.Name == "stdout" || fe.Name.Name == "stderr" {
        // Require import sys
        if !c.info.StdlibImports["sys"] {
            c.add(diagAt("DTE0200", ...))
            continue
        }
        // Mark as typed so import tracker sees usage
        c.info.Types[fe] = types.Any
        continue
    }
}
```

## Adding New sys Members

1. **Add to sys.desi**: `pub extern newmember: type`
2. **Update type check**: Add case in expr_call.go
3. **Update lowering**: Add case in lower_expr.go  
4. **Update C runtime**: If needed, add __get_newmember()
5. **Update docs**: Add to learner and contributor docs
