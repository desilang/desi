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
    print("Run with: desic test test_math.desi")
    return 0
```

Run with:
```bash
desic test test_math.desi
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

### Examples

```desi
def main() -> int:
    let x: int = 5
    
    # Basic assertions
    assert(x == 5)
    assert(x > 0, "x should be positive")
    
    # Expression conditions
    assert(1 + 1 == 2)
    assert(len("hello") == 5)
    
    print("All assertions passed!")
    return 0
```

---

## @test Decorator

Mark test functions with the `@test` decorator:

```desi
@test
def test_something() -> none:
    # Test code with assertions
    assert(condition)
```

**Rules:**
1. Test functions should return `none`
2. Test functions should take no parameters
3. Use `assert` for test conditions
4. Function names conventionally start with `test_`

### Multiple Tests

```desi
@test
def test_addition() -> none:
    assert(1 + 1 == 2)

@test
def test_subtraction() -> none:
    assert(5 - 3 == 2)

@test
def test_edge_cases() -> none:
    assert(0 + 0 == 0)
    assert(-1 + 1 == 0)
```

---

## Running Tests

### Command Line

```bash
# Run tests in a file
desic test myfile.desi

# Verbose mode
desic test myfile.desi -v
desic test myfile.desi --verbose
```

### Output

```
✓ Test file type-checked successfully: myfile.desi
```

If a test fails (assertion fails):
```
Assertion failed: expected x to be positive
```

---

## Best Practices

### ✅ DO

```desi
# DO: Use descriptive test names
@test
def test_user_login_with_valid_credentials() -> none:
    # ...

# DO: Test one thing per test
@test
def test_addition() -> none:
    assert(1 + 1 == 2)

# DO: Use helpful assertion messages
@test
def test_bounds() -> none:
    let x: int = get_value()
    assert(x >= 0, "value should not be negative")
    assert(x <= 100, "value should not exceed 100")
```

### ❌ DON'T

```desi
# DON'T: Test many unrelated things in one test
@test
def test_everything() -> none:
    assert(login_works())
    assert(database_works())
    assert(api_works())

# DON'T: Skip assertion messages for complex conditions
@test
def test_complex() -> none:
    assert(complicated_calculation() == expected)  # ❌ No message
```

---

## Implementation Status

| Feature | Status |
|---------|--------|
| `assert(bool)` | ✅ Implemented |
| `assert(bool, str)` | ✅ Implemented |
| `@test` decorator | ✅ Implemented |
| `desic test` command | ✅ Basic (type-check only) |
| Test execution | 🔄 Coming soon |
| Test discovery | 🔄 Coming soon |

---

## Examples

See working examples:
- `examples/147_testing_assert.desi` - Assert usage
- `examples/148_testing_test_decorator.desi` - @test decorator
