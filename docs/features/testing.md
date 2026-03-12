# Testing in Desi - Complete Guide

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [Assert Builtin](#assert-builtin)
3. [@test Decorator](#test-decorator)
4. [Running Tests](#running-tests)
5. [Best Practices](#best-practices)

---

## Quick Start

```desi
# test_math.desi

@test
def test_addition() -> none:
    assert(1 + 1 == 2, "basic addition")
    assert(2 + 3 == 5)

@test  
def test_comparison() -> none:
    assert(5 > 3)
    assert(2 < 4)

def main() -> int:
    # Call all test functions
    test_addition()
    test_comparison()
    return 0
```

Run with:
```bash
desic test test_math.desi
```

Output:
```
==> Type-checking test_math.desi ...
==> Generating LLVM IR...
==> Compiling to object file...
==> Linking executable...
==> Running tests...
✓ All tests passed!
```

---

## Assert Builtin

The `assert` function checks conditions at runtime:

```desi
# One argument: condition only
assert(x > 0)

# Two arguments: condition + custom message
assert(x > 0, "x must be positive")
```

**Behavior:**
- If condition is `true`: continues execution
- If condition is `false`: prints error message and exits with code 1

**Signatures:**
```desi
def assert(condition: bool) -> none
def assert(condition: bool, message: str) -> none
```

---

## Assert Equality / Inequality

### assert_eq

Checks that two values are **equal**. On failure, shows diff output:

```desi
assert_eq(42, 42)          # passes
assert_eq(1 + 1, 3)        # fails with:
# assertion failed: line 2: assert_eq failed
#   expected: 2
#     actual: 3

assert_eq("hello", "hello", "greeting check")  # with custom message
```

**Signatures:**
```desi
def assert_eq(expected: int, actual: int) -> none
def assert_eq(expected: str, actual: str) -> none
def assert_eq(expected: bool, actual: bool) -> none
def assert_eq(expected: T, actual: T, message: str) -> none
```

### assert_ne

Checks that two values are **not equal**:

```desi
assert_ne(1, 2)             # passes
assert_ne(42, 42)            # fails with:
# assertion failed: line 2: assert_ne failed
#   values should differ but both are: 42

assert_ne("a", "b", "must be different")  # with custom message
```

**Signatures:**
```desi
def assert_ne(a: int, b: int) -> none
def assert_ne(a: str, b: str) -> none
def assert_ne(a: bool, b: bool) -> none
def assert_ne(a: T, b: T, message: str) -> none
```

## @test Decorator

Mark test functions with the `@test` decorator:

```desi
@test
def test_something() -> none:
    assert(condition)
```

**Rules:**
1. Test functions should return `none`
2. Test functions take no parameters
3. Use `assert` for test conditions
4. Function names start with `test_`
5. **Call test functions from `main()`**

### Complete Pattern

```desi
@test
def test_addition() -> none:
    assert(1 + 1 == 2)

@test
def test_subtraction() -> none:
    assert(5 - 3 == 2)

def main() -> int:
    test_addition()
    test_subtraction()
    return 0
```

---

## Running Tests

### Command Line

```bash
# Run tests
desic test myfile.desi

# Verbose mode
desic test myfile.desi -v
```

### What Happens

1. **Type-check** - Validates syntax and types
2. **Generate IR** - Produces LLVM IR
3. **Compile** - Uses `llc` for object file
4. **Link** - Uses `clang` for executable
5. **Execute** - Runs the test

### Success Output

```
==> Type-checking myfile.desi ...
==> Generating LLVM IR...
==> Compiling to object file...
==> Linking executable...
==> Running tests...
✓ All tests passed!
```

### Failure Output

```
Assertion failed: expected x to be positive

✗ TEST FAILED
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All tests passed |
| 1 | Test failure |
| 2 | Compilation error |

---

## Best Practices

### ✅ DO

```desi
# Descriptive names
@test
def test_user_login_valid() -> none:
    # ...

# Helpful messages
@test
def test_bounds() -> none:
    assert(x >= 0, "should not be negative")

# Call all tests from main()
def main() -> int:
    test_user_login_valid()
    test_bounds()
    return 0
```

### ❌ DON'T

```desi
# Don't forget to call tests
def main() -> int:
    return 0  # ❌ Tests won't run!
```

---

## Prerequisites

```bash
# macOS
brew install llvm

# Ubuntu/Debian
sudo apt install llvm clang
```

---

## Examples

All testing examples are provided inline throughout this document:

- **Quick Start**: See opening section for complete test pattern
- **Assert**: See "Assert Builtin" section
- **@test Decorator**: See "@test Decorator" section
