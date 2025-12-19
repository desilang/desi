# Numeric Types Implementation

This document describes the implementation of Desi's numeric type system, including sized integers, the `as` cast operator, and the `decimal` type.

## Type System Overview

### Kind Constants

All numeric types are defined in `compiler/internal/types/types.go` with distinct `Kind` constants:

```go
const (
    IntKind Kind = iota
    FloatKind
    I8Kind, I16Kind, I32Kind, I64Kind, I128Kind
    U8Kind, U16Kind, U32Kind, U64Kind, U128Kind
    F32Kind, F64Kind
    CharKind
    DecimalKind
    // ...
)
```

### Type Singletons

Sized types are declared in `compiler/internal/types/numeric.go`:

```go
var (
    I8   = &basic{kind: IntKind, name: "i8"}    // Uses IntKind for compatibility
    I16  = &basic{kind: IntKind, name: "i16"}
    // ...
    F32  = &basic{kind: FloatKind, name: "f32"}
    F64  = &basic{kind: FloatKind, name: "f64"}
)
```

> **Note**: Sized types currently use base `IntKind`/`FloatKind` for backward compatibility with literal assignment. The specific kind constants exist for future stricter type checking.

## The `as` Cast Operator

### AST Node

`ast.CastExpr` in `compiler/internal/ast/nodes.go`:

```go
type CastExpr struct {
    X    Expr      // expression to cast
    Type *TypeName // target type
    Span diag.Span
}
```

### Parser

In `compiler/internal/parse/expr.go`, `as` is parsed as a postfix operator at the same precedence level as `is`:

```go
case token.KW_as:
    opStart := p.cur
    p.next()
    targetType := p.parseTypeName()
    e = &ast.CastExpr{
        X:    e,
        Type: targetType,
        Span: joinTok(p.file, opStart, p.cur),
    }
```

### Type Checking

In `compiler/internal/check/expr.go`, the type checker validates that both source and target types are numeric:

```go
case *ast.CastExpr:
    fromType := c.typ(x.X)
    toType := c.resolveType(x.Type)
    
    fromNumeric := types.Equal(fromType, types.Int) || 
                   types.Equal(fromType, types.Float) || ...
    toNumeric := types.Equal(toType, types.Int) || ...
    
    if !fromNumeric || !toNumeric {
        c.add(diagAt("DTE0004", e.SpanOf(), 
            fmt.Sprintf("cannot cast %s to %s", fromType, toType)))
    }
    
    c.info.Types[e] = toType
    return toType
```

**Error Code**: `DTE0004` - "cannot cast X to Y"

### LLVM Lowering

In `compiler/internal/lower/lower_expr.go`:

```go
case *ast.CastExpr:
    src := ls.lowerExpr(x.X)
    dstType := ls.info.Types[e]
    dstLLVMType := lowerType(dstType)
    
    dst := ls.b.FreshTemp("cast")
    ls.b.Emit(&hir.Cast{Dst: dst, Src: src, Type: dstLLVMType})
    return dst
```

### IR Emission

In `compiler/internal/backend/llvm/emit_func.go`, `intTypeBits()` determines the appropriate LLVM opcode:

| Conversion | Opcode |
|------------|--------|
| Large int → Small int | `trunc` |
| Small int → Large int | `sext` |
| Float → Int | `fptosi` |
| Int → Float | `sitofp` |
| Double → Float | `fptrunc` |
| Float → Double | `fpext` |

```go
func intTypeBits(ty string) int {
    switch ty {
    case "i8":  return 8
    case "i16": return 16
    case "i32": return 32
    case "i64": return 64
    default:    return 0
    }
}
```

## Decimal Type

### Library: libmpdec

Desi uses [libmpdec](https://www.bytereef.org/mpdecimal/) 4.0.1 (BSD license) for arbitrary-precision decimal arithmetic.

**File Location**: `compiler/runtime/decimal/`

### Build Integration

In `Makefile`:

```makefile
decimal-lib: $(DECIMAL_LIB)

$(DECIMAL_LIB):
    cd $(DECIMAL_SRC)/mpdecimal-4.0.1 && ./configure --quiet && make -C libmpdec -s
    cp $(DECIMAL_SRC)/mpdecimal-4.0.1/libmpdec/libmpdec.a $(DECIMAL_SRC)/lib/
```

### C Wrapper Functions

`compiler/runtime/decimal/desi_decimal.c` provides simplified FFI:

| Function | Description |
|----------|-------------|
| `__decimal_new(const char* str)` | Create from string |
| `__decimal_from_int(int64_t value)` | Create from integer |
| `__decimal_add(a, b)` | Addition |
| `__decimal_sub(a, b)` | Subtraction |
| `__decimal_mul(a, b)` | Multiplication |
| `__decimal_div(a, b)` | Division |
| `__decimal_cmp(a, b)` | Compare (-1, 0, 1) |
| `__decimal_to_str(d)` | Convert to string |
| `__decimal_free(d)` | Free memory |

**Precision**: 28 digits (Python's default)  
**Rounding**: ROUND_HALF_EVEN (Banker's rounding)

### Memory Safety

Decimal values are heap-allocated (`mpd_t*`). Memory management is ensured by:

1. **Scope-based Cleanup**: Compiler emits `__decimal_free()` at block exit
2. **No Implicit Copies**: Assignment moves ownership (Desi's move semantics)
3. **Wrapper Knows Allocation**: All allocation via `__decimal_new`/`__decimal_from_int`

### Decimal Literals

Decimal literals use the `d`/`D` suffix:

```desi
let price: decimal = 19.99d    # Float with d suffix
let count: decimal = 42d       # Integer with d suffix  
let sci: decimal = 1.5e3d      # Scientific with d suffix
```

**Scanner**: `compiler/internal/lex/scanner.go` checks for `d`/`D` suffix after parsing any numeric literal and emits `DECIMAL_LIT` token.

**Type Checking**: `compiler/internal/check/expr_binary.go` allows `decimal + decimal`, `decimal - decimal`, etc.

### Cross-Platform Support

libmpdec source is bundled and built on first compile:

| Platform | Build Status |
|----------|-------------|
| macOS (ARM64) | ✅ Built & tested |
| macOS (x86) | Builds from source |
| Linux | Builds from source |
| Windows | Use `vcbuild64.bat` |

See `compiler/runtime/decimal/README.md` for build instructions.
