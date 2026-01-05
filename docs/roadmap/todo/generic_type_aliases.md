# TODO: Generic Type Aliases

**Status**: Proposed  
**Priority**: Medium  
**Related**: Generics, Type system

## Overview

Support type aliases for generic types to improve code readability and maintainability.

## Current State

The `type` keyword is **already reserved** in Desi (see `compiler/internal/token/keywords.go`).

**Check needed**: Verify `type` is not currently used for anything that conflicts with type aliases.

## Proposed Syntax

```desi
# Basic type alias
type IntBox = Box<int>

# Generic type alias (parameterized)
type Pair<T> = (T, T)
type MaybeList<T> = Option<List<T>>

# Complex aliases for clarity
type UserId = int
type Result<T> = Result<T, Error>
type Handler<T> = fn(T) -> Result<T>
```

## Use Cases

### 1. Code Clarity

**Before**:
```desi
def process(
    input: Result<Option<List<String>>, Error>
) -> Result<Option<List<String>>, Error>:
    # Hard to read, DRY violation
```

**After**:
```desi
type MaybeStrings = Option<List<String>>
type DataResult = Result<MaybeStrings, Error>

def process(input: DataResult) -> DataResult:
    # Much clearer!
```

### 2. API Simplification

```desi
# Internal complex type
type _InternalMap = Dict<String, List<Tuple<int, float>>>

# Public alias hides complexity
type ScoreBoard = _InternalMap

# Users don't need to know internal representation
def get_scores() -> ScoreBoard:
    pass
```

### 3. Domain Modeling

```desi
# Makes intent clear
type UserId = int
type Email = String
type Timestamp = int

def send_email(user: UserId, to: Email, at: Timestamp):
    # Type system prevents mixing up int parameters
```

### 4. Migration/Refactoring

```desi
# Old API
type OldResult<T> = Result<T, String>

# New API with better errors
type NewResult<T> = Result<T, Error>

# Gradual migration
type ApiResult<T> = NewResult<T>  # Can swap back to OldResult if needed
```

## Implementation

### Phase 1: Non-Generic Aliases

```go
// In compiler/internal/ast/nodes.go
type TypeAliasDecl struct {
    Name  Ident
    Type  *TypeName
    Span  diag.Span
}

// In compiler/internal/check/check.go
func (c *checker) collectTypeAlias(td *ast.TypeAliasDecl) {
    // Resolve the target type
    targetType := c.resolveType(td.Type)
    
    // Register alias in scope
    c.scope.Define(&Symbol{
        Name: td.Name.Name,
        Kind: SymType,
        Type: targetType,
    })
}
```

### Phase 2: Generic Aliases

```desi
type Pair<T> = (T, T)

# Internally becomes:
# A function from Type -> Type
# Pair[int] = (int, int)
```

### Variance Considerations

For now: **All type aliases are invariant** (safest default).

Future: Support variance annotations if needed:
```desi
type ReadOnlyBox<+T> = Box<T>  # Covariant (read-only)
type WriteOnlyBox<-T> = Box<T> # Contravariant (write-only)
type Box<T> = ...               # Invariant (default)
```

See `variance.md` for details on variance.

## Parser Changes

```go
// In compiler/internal/parse/decl.go

func (p *Parser) parseDecl() ast.Decl {
    switch p.cur.Tok {
    case token.KW_type:
        return p.parseTypeAlias()
    // ... existing cases
    }
}

func (p *Parser) parseTypeAlias() *ast.TypeAliasDecl {
    p.expect(token.KW_type, "type")
    name := p.parseIdent()
    
    // Optional type parameters
    var typeParams []ast.Ident
    if p.accept(token.LT) {
        typeParams = p.parseTypeParamList()
        p.expect(token.GT, ">")
    }
    
    p.expect(token.ASSIGN, "=")
    targetType := p.parseTypeName()
    
    return &ast.TypeAliasDecl{
        Name:       name,
        TypeParams: typeParams,
        Type:       targetType,
    }
}
```

## Type Resolution

```go
// In compiler/internal/check/typename_resolver.go

func (c *checker) resolveType(tn *ast.TypeName) types.T {
    // Check if name is a type alias
    if sym := c.scope.Lookup(tn.Name); sym != nil && sym.Kind == SymType {
        baseType := sym.Type
        
        // If base type is generic and we have type arguments, instantiate
        if len(tn.Params) > 0 {
            // Instantiate generic alias
            return c.instantiateGeneric(baseType, tn.Params)\n        }
        
        return baseType
    }
    
    // ... existing resolution logic
}
```

## Benefits

✅ **Code clarity**: Self-documenting types  
✅ **DRY principle**: Define complex types once  
✅ **Refactoring**: Change type definition in one place  
✅ **Abstraction**: Hide implementation details  
✅ **Domain modeling**: Express intent clearly  

## Task Breakdown

- [ ] Verify `type` keyword is not used elsewhere
- [ ] Add `TypeAliasDecl` to AST
- [ ] Implement parser for type aliases
- [ ] Add type alias collection to checker
- [ ] Implement type resolution for aliases
- [ ] Add support for generic type aliases
- [ ] Write tests for various alias patterns
- [ ] Document type alias feature for users

## Examples to Test

```desi
# Simple alias
type Counter = int

# Generic alias
type Pair<T> = (T, T)

# Nested generics
type Matrix<T> = List<List<T>>

# Function type alias
type BinaryOp<T> = fn(T, T) -> T

# Result type alias
type Result<T> = Result<T, Error>

# Complex nested alias
type AsyncResult<T> = Future<Result<T, Error>>
```

## References

- TypeScript: `type` keyword
- Rust: `type` keyword for aliases
- Haskell: `type` keyword
- Swift: `typealias` keyword
