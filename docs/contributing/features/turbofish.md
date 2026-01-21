# Turbofish Syntax Design Decision

This document explains the design rationale behind Desi's turbofish (`::<>`) syntax for explicit generic instantiation.

## The Problem

When calling generic functions or instantiating generic types, users sometimes need to specify type arguments explicitly:

```python
identity<int>(42)    # What we might want
```

However, this creates **parser-level ambiguity** with comparison operators:

```python
if x < y > z:        # Chained comparison (Python-style)
foo < bar > baz      # Is this comparison or generics?
```

## Alternatives Considered

### 1. Direct `<>` with Lookahead

**Approach**: Parse `IDENT<...>` and use lookahead to detect if followed by `(`.

**Problems**:
- Parser state corruption during backtracking
- Complex interaction with chained comparisons (`x < 10 > y`)
- Desi supports Python-style chained comparisons, making this even harder

**Verdict**: Rejected due to parser complexity and reliability issues.

### 2. Two-Pass Parsing with Type Lookup

**Approach**: 
1. First pass: collect all type names (class, struct, enum, trait)
2. Second pass: use type table to disambiguate `<>`

**Problems**:
- Forward references break (using `Box<T>` before `class Box` is defined)
- Slower compilation (two full passes)
- Complex implementation across parser/checker boundary

**Verdict**: Rejected due to forward reference issues and performance cost.

### 3. Square Brackets `[]` (Go/Scala style)

**Approach**: Use `identity[int](42)` like Go and Scala.

**Problems**:
- Conflicts with Desi's list/slice syntax: `list[0]`, `list[::2]`
- Ambiguity: `foo[x]` — is `x` a type or index expression?

**Verdict**: Rejected due to conflict with existing index/slice syntax.

### 4. Prefix Syntax `<T>foo()`

**Approach**: Put type args before the function name, like Java's `Collections.<String>emptyList()`.

**Problems**:
- Unusual syntax for most developers
- Method chaining looks awkward: `obj.<T>method()`

**Verdict**: Viable but rejected for unfamiliarity.

### 5. Turbofish `::<>` (Rust style)

**Approach**: Use `identity::<int>(42)` with `::` as the turbofish operator.

**Advantages**:
- Unambiguous: `::` followed by `<` is clearly a type argument list
- Simple parser implementation (lookahead for `: : <` pattern)
- Established precedent in Rust
- Preserves comparison operators and chained comparisons

**Verdict**: Adopted.

## Implementation Details

### Parser Changes

The turbofish is detected in `parsePostfix()` when we see a `COLON` token:

```go
case token.COLON:
    // Check for ::<T> pattern
    if p.peek.Tok != token.COLON {
        return e  // Single colon, not turbofish
    }
    // Fill ahead buffer if needed
    if len(p.ahead) == 0 {
        p.ahead = append(p.ahead, p.sc.Next())
    }
    if p.ahead[0].Tok != token.LT {
        return e  // Not followed by <, not turbofish
    }
    // Consume :: and < then parse type args
```

This approach:
1. Avoids adding `::` as a separate token (would break slice syntax `[::]`)
2. Uses 2-token lookahead to detect the `:: <` pattern
3. Only parses type args when we're certain it's turbofish

### AST Changes

`CallExpr` has a new `TypeArgs` field:

```go
type CallExpr struct {
    Callee   Expr
    TypeArgs []*TypeName  // Explicit type args from turbofish
    Args     []Expr
    ArgNodes []CallArg
    Span     diag.Span
}
```

### Type Checker Integration

When `TypeArgs` is non-empty, the type checker should:
1. Skip type inference for generic parameters
2. Use the explicitly provided types
3. Validate they satisfy any trait bounds

## Trade-offs

| Aspect | Turbofish `::<>` | Alternatives |
|--------|------------------|--------------|
| Parser complexity | Low | High (lookahead/two-pass) |
| Ambiguity | None | Various edge cases |
| Familiarity | Rust developers | Varies by approach |
| Aesthetics | "Ugly" to some | Cleaner looking |
| Type inference compatibility | Excellent | Same |

## Conclusion

The turbofish syntax was chosen for **compiler simplicity, unambiguous parsing, and correctness**. While some find the syntax aesthetically unpleasant, it:

1. Works correctly in all cases
2. Keeps the parser simple and fast
3. Follows established precedent
4. Rarely appears in code (type inference handles most cases)

The "ugliness" is acceptable because explicit type arguments are the exception, not the norm.

## References

- [Rust RFC on turbofish](https://github.com/rust-lang/rfcs/blob/master/text/0558-require-parentheses-for-chained-comparisons.md)
- [TypeScript generics parsing](https://github.com/microsoft/TypeScript/issues/15713)
- [Go generics design](https://go.dev/blog/generics-proposal)
