# F-String Format Specifiers - Implementation

## Overview

F-string format specifiers allow users to control how values are formatted.

**Syntax**: `f"{value:spec}"`

**Examples**:
- `f"{pi:.2f}"` → `3.14`
- `f"{count:05d}"` → `00042`
- `f"{num:x}"` → `2a`

## Implementation

### AST Changes (`ast/nodes.go`)

Added `FStringExpr` to wrap expressions with format specs:

```go
type FStringExpr struct {
    X    Expr   // The expression to format
    Spec string // Format spec (e.g., ".2f", "05d")
    Span diag.Span
}
```

`FString.Parts` can now contain `StrLit`, `FStringExpr`, or raw expressions.

### Parser Changes (`parse/expr.go`)

Modified `parseFString()` to check for `:` after expressions:

```go
case token.LBRACE:
    expr := p.parseExpr()
    var spec string
    if p.cur.Tok == token.COLON {
        p.next() // consume :
        spec = p.parseFStringSpec()
    }
    // Wrap in FStringExpr if spec present
```

Added `parseFStringSpec()` to collect spec tokens until `}`.

### Lowering Changes (`lower/lower_expr.go`)

Added `FStringExpr` case and `specToPrintf()` helper:

```go
case *ast.FStringExpr:
    val := ls.lowerExpr(p.X)
    fmtSpec := ls.specToPrintf(p.Spec, p.X)
    fmtBuilder.WriteString(fmtSpec)
```

`specToPrintf()` converts Desi specs to printf format:
- `.2f` → `%.2f`
- `05d` → `%05lld`
- `x` → `%x`

## Files Modified

| File | Change |
|------|--------|
| `ast/nodes.go` | Added `FStringExpr` struct |
| `parse/expr.go` | Parse `:spec` in f-strings |
| `lower/lower_expr.go` | `specToPrintf()` conversion |
| `book/docs/language/strings.md` | Added format specifiers docs |

## Test

`examples/231_fstring_format.desi` covers:
- Float precision (`.2f`)
- Zero-padding (`05d`)
- Hex formatting (`x`, `X`)
- Width (`10d`)
