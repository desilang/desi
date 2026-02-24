# Assert Statement

## Overview

The `assert` keyword provides runtime assertion checking. When the condition evaluates to `false`, the program prints a diagnostic message and exits with code 1.

```
assert <condition>
assert <condition>, "optional message"
```

## Implementation

### Compiler Pipeline

| Layer | File | What |
|-------|------|------|
| Token | `token/token.go` | `KW_assert` constant |
| Keywords | `token/keywords.go` | `"assert": KW_assert` map entry |
| Lexer | `lex/scanner.go` | `keywordToken()` switch case |
| AST | `ast/nodes.go` | `AssertStmt{Cond, Msg, Span}` |
| Parser | `parse/stmt.go` | `parseAssert()` — dispatched from `parseStmt()` |
| Lowering | `lower/lower_stmt.go` | Emits `if !cond { __desi_assert_fail("line N: msg") }` |
| C Runtime | `runtime/builtins.c` | `__desi_assert_fail()` — prints to stderr, exits(1) |

### Lowering Strategy

Assert is lowered as a conditional branch to a failure block:

```
cond = <lower condition expression>
neg  = icmp eq i1 %cond, false
br i1 %neg, label %assert_fail, label %continue

assert_fail:
  call void @__desi_assert_fail(ptr @"line N: message")
  ; block is unreachable after exit(1)

continue:
  ; rest of program
```

The failure message includes the line number and optional user message, baked in as a compile-time string constant.

### AST Node

```go
type AssertStmt struct {
    Cond Expr      // condition (required)
    Msg  Expr      // optional message (typically StrLit)
    Span diag.Span
}
```

### Parser

`parseAssert()` consumes `KW_assert`, parses the condition expression, optionally consumes a comma and message expression, then expects a newline.

## Testing

- `examples/147_testing_assert.desi` — basic assertions
- `examples/148_testing_test_decorator.desi` — asserts inside `@test` functions  
- `examples/408_assert.desi` — comprehensive assert tests
