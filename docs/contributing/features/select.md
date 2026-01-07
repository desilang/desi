# Select Statement Implementation

This document describes how the `select` statement is implemented in the Desi compiler.

## Overview

The `select` statement provides channel multiplexing - waiting on multiple channel operations and executing the first one that succeeds.

```desi
select:
    case msg = rx1.try_recv():
        print("Got:", msg)
    case rx2.try_recv():
        print("Got from ch2")
    default:
        print("Nothing ready")
```

## Semantics

1. Each case is evaluated in order (non-blocking)
2. First case that succeeds executes its body
3. Remaining cases are skipped
4. Default runs only if no case succeeded

## Implementation Files

| File | Purpose |
|------|---------|
| `ast/stmt_select_node.go` | AST nodes: SelectStmt, SelectCase |
| `parse/stmt_select.go` | Parser for select syntax |
| `check/check_select.go` | Type checker for case operations |
| `lower/select_lower.go` | Lowers to matched-flag if-chain |

## AST Structure

```go
type SelectStmt struct {
    Cases   []SelectCase  // case clauses
    Default []Stmt        // default body (nil if none)
}

type SelectCase struct {
    Binding *Ident  // variable to bind (nil for send)
    Op      Expr    // channel operation (try_recv/try_send)
    Body    []Stmt  // statements to execute
}
```

## Lowering Strategy

Select is lowered to a series of if-checks with a "matched" flag:

```
// Pseudo-code:
matched = false
if !matched && rx1.try_recv() != nil:
    matched = true
    // case 1 body
if !matched && rx2.try_recv() != nil:
    matched = true
    // case 2 body
if !matched:
    // default body
```

This ensures:
- Only the first ready case executes
- Short-circuit evaluation (remaining cases skipped)
- Default runs only if nothing matched

## Type Requirements

- Case operations must return `Option<T>` (try_recv) or `bool` (try_send)
- Binding variable type is inferred from inner Option type
- Body statements type-checked normally

## Generics Support

Select works with any channel type `Channel<T>`:
- `try_recv()` returns `Option<T>`
- `try_send(value: T)` returns `bool`
- Binding variable gets type `T`

## Memory Safety

- No blocking operations (uses try_* variants)
- No data races (evaluated sequentially on single thread)
- Channels are reference-counted

## Test Files

- `examples/220_select_test.desi` - basic select with multiple channels
