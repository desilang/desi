# Language Features Testing - Status Report

> **⚠ Historical document** — this snapshot dates from the early backend
> milestones and no longer reflects reality (structs, enums, match, for
> loops, classes, and collections have long been implemented). For current
> status see `CHANGELOG.md` (0.1.0) and the example suite:
> `test_examples.sh` / `test_examples.ps1` — 473+/481 passing on all
> platforms.

## ✅ FULLY WORKING

### Core Features
- [x] **Variables**: `let` (immutable), `let mut` (mutable), `:=` (assignment)
- [x] **Primitives**: `int`, `bool`, `string`
- [x] **Operators**:
  - Unary: `-`, `not` 
  - Arithmetic: `+`, `-`, `*`, `/`, `%`
  - Comparison: `<`, `>`, `==`, `!=`, `<=`, `>=`
- [x] **Functions**: 
  - Definition with typed parameters
  - Return values (int, bool - ✅ FIXED)
  - Function calls
  - Complex nested expressions
- [x] **Control Flow**:
  - If/else statements
  - While loops
  - Mutable state in loops
- [x] **Collections**:
  - Tuples (creation, indexing)
  - Sets (creation, `.contains()`)
  - Dicts (creation, `.has_key()`)
- [x] **Backend**:
  - Float arithmetic (fadd, fsub, fmul, fdiv, frem)
  - Float comparisons (fcmp)
  - Proper type tracking for parameters and variables
  - ✅ Bool return types (-> bool) now work correctly

## ⚠️ PARTIALLY WORKING (Syntax OK, Implementation Incomplete)

- [ ] **Structs**:
  - ✅ Syntax parses
  - ✅ Type checking works
  - ❌ Instantiation fails - needs constructor codegen
  - ❌ Field access not fully implemented
  
- [ ] **Enums**:
  - ✅ Syntax parses
  - ❌ Not tested for instantiation/matching

- [ ] **Match expressions**:
  - ✅ Syntax exists in parser
  - ❌ Not tested for lowering/codegen

## ❌ NOT TESTED / LIKELY NOT IMPLEMENTED

- [ ] **For loops** - Need to check if syntax exists
- [ ] **Break/Continue** - Need to check if syntax exists  
- [ ] **Arrays** - Fixed-size arrays
- [ ] **Lists** - Dynamic arrays
- [ ] **String operations** - Concatenation, slicing
- [ ] **Classes** - Full class support with methods
- [ ] **List comprehensions** - Syntax exists but not tested

## Recent Fixes (This Session)

1. ✅ **Function parameters** - Now correctly typed (was treating all as `ptr`)
2. ✅ **Bool return types** - Functions returning `bool` now emit `i1` correctly
3. ✅ **Call type inference** - Function calls now track correct return types
4. ✅ **Unary operators** - `-x` and `not x` Working
5. ✅ **Float support** - Full backend support for float operations

## Next Steps

1. **Struct instantiation** - Generate constructor functions
2. **Field access** - Implement `.field` lowering for structs
3. **Enums** - Test instantiation and match expressions
4. **For loops** - Check syntax and implement if exists
5. **Arrays/Lists** - Core collection types
