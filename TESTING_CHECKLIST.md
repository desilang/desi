# Basic Language Feature Testing Checklist

## ✅ Already Tested (in examples/39_basics_smoke.desi)
- [x] Integer literals and operations
- [x] Boolean literals (true/false)
- [x] String literals
- [x] Unary operators (-, not)
- [x] Arithmetic operators (+, -, *, /, %)
- [x] Comparison operators (<, >, ==, !=, <=, >=)
- [x] Tuples (creation and indexing)
- [x] Sets (creation and .contains method)
- [x] Dicts (creation and .has_key method)
- [x] If/else statements
- [x] While loops
- [x] Mutable variables (let mut, :=)
- [x] Variable declaration (let)
- [x] Print statements

## ❓ To Be Tested

### Numeric Types
- [ ] **Float literals and operations** - We added support but didn't test
  - [ ] Float arithmetic (3.14 + 2.0, etc.)
  - [ ] Float comparisons
  - [ ] Negative floats (-3.14)
- [ ] **Different integer sizes** (if implemented)
  - [ ] i8, u8, i16, u16, i32, u32, i64, u64
  - [ ] Type conversions/casts

### Structs and Enums
- [ ] **Struct definition** - Syntax exists but not tested for compilation
  - [ ] Struct instantiation
  - [ ] Field access
- [ ] **Enum definition** - Syntax exists but not tested
  - [ ] Enum variants
  - [ ] Pattern matching with enums

### Functions
- [ ] **User-defined functions** - Only tested main()
  - [ ] Function definition
  - [ ] Function calls with arguments
  - [ ] Function return values
  - [ ] Multiple return values (if implemented)
- [ ] **Lambdas/Closures** - Syntax exists in examples

### Advanced Control Flow
- [ ] **Match expressions** - Syntax exists
- [ ] **For loops** - If they exist
- [ ] **Break/Continue** - If they exist
- [ ] **Return statements**

### Collections
- [ ] **Arrays** - Fixed-size arrays
- [ ] **Lists** - Dynamic arrays
- [ ] **List comprehensions** - Syntax exists in examples
- [ ] **String operations**
  - [ ] String concatenation
  - [ ] String indexing/slicing
  - [ ] String methods

### Advanced Features
- [ ] **Traits/Interfaces** - Examples exist
- [ ] **Method calls** - .method() syntax
- [ ] **Operator overloading**
- [ ] **Error handling** - If it exists

## Notes
- Some features may be type-checked but not fully lowered to LLVM
- Need to verify which features are actually implemented in the backend
