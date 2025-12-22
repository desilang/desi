# First Program

Let's write your first Desi program - the classic "Hello, World!"

---

## Hello, World!

Create a new file called `hello.desi`:

```desi
# hello.desi
def main():
    print("Namaste, World!")
```

### Understanding the Code

Let's break down each part:

```desi
def main():    # (1)!
    print("Namaste, World!")  # (2)!
```

1. Every Desi program needs a `main` function. The return type is optional!
2. `print()` is a built-in function that outputs to the console

---

## Compile and Run

Compile your program:

```bash
./bin/desic hello.desi
```

This creates an executable at `./build/output/test_exec`. Run it:

```bash
./build/output/test_exec
```

Output:

```
Namaste, World!
```

🎉 Congratulations! You've just run your first Desi program!

---

## A More Interesting Example

Let's create something more substantial - a greeting program:

```desi
# greet.desi

def greet(name: str) -> str:
    return "Namaste, " + name + "!"

def main():
    let message = greet("Desi Developer")
    print(message)
    
    # Let's also print some numbers
    let x = 10
    let y = 20
    print("Sum:")
    print(x + y)
```

Compile and run:

```bash
./bin/desic greet.desi && ./build/output/test_exec
```

Output:

```
Namaste, Desi Developer!
Sum:
30
```

---

## Key Concepts

### Variables

Use `let` for immutable variables:

```desi
let name = "Desi"       # Type inferred as str
let count: int = 42     # Explicit type annotation
```

Use `var` for mutable variables:

```desi
var counter = 0
counter = counter + 1   # OK - counter is mutable
```

### Functions

Define functions with `def`:

```desi
def add(a: int, b: int) -> int:
    return a + b
```

Or use the implicit return (last expression):

```desi
def add(a: int, b: int) -> int:
    a + b  # No return keyword needed!
```

### Comments

```desi
# This is a single-line comment

# Multi-line comments use
# multiple single-line comments
```

---

### Return Type (Optional)

```desi
# ✅ Simple - no return needed for most programs
def main():
    print("Hello")

# ✅ Also valid - return int if you need an exit code
def main() -> int:
    print("Hello")
    0  # Exit code 0 = success
```

### Type Mismatch

```desi
# ❌ Wrong - can't add str and int
let result = "Count: " + 42

# ✅ Correct - use string interpolation (coming soon)
# Or print separately:
print("Count:")
print(42)
```

---

## What's Next?

Now that you can write basic programs, continue with:

- [Editor Setup](editor-setup.md) - Configure your editor for Desi
- [Introduction Tutorial](../tutorials/intro.md) - Learn Desi in depth
- [Variables & Types](../tutorials/variables.md) - All about types
