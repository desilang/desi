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

- `examples/147_testing_assert.desi` - Assert usage
- `examples/148_testing_test_decorator.desi` - @test decorator
