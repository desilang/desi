# First Program

Let's write your first Desi program - the classic "Hello, World!"

---

## Hello, World!

Create a new file called `hello.desi`:

```desi
# hello.desi
def main() -> int:
    print("Namaste, World!")
    0
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

## Run It

The simplest way — build and run in one step:

```bash
desic run hello.desi
```

Output:

```
Namaste, World!
```

Or build an executable you can distribute:

```bash
desic build hello.desi -o hello
./hello
```

🎉 Congratulations! You've just run your first Desi program!

---

## A More Interesting Example

Let's create something more substantial - a greeting program:

```desi
# greet.desi

def greet(name: str) -> str:
    return "Namaste, " + name + "!"

def main() -> int:
    let message = greet("Desi Developer")
    print(message)
    
    # String interpolation with f-strings
    let x = 10
    let y = 20
    print(f"Sum: {x + y}")
    0
```

Run it:

```bash
desic run greet.desi
```

Output:

```
Namaste, Desi Developer!
Sum: 30
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

### Return Types

```desi
# Return int for exit codes
def main() -> int:
    print("Hello")
    0  # Exit code 0 = success
```

### String Interpolation

```desi
let name = "world"
let count = 42
print(f"Hello {name}, count is {count}")
```

---

## Creating a Project

For larger programs, use `desic init` to create a project with a `desi.mod` manifest:

```bash
desic init myapp
cd myapp
desic run          # runs src/main.desi
desic build        # builds to build/output/myapp
```

This creates:
```
myapp/
├── desi.mod          # Project manifest
├── src/
│   └── main.desi     # Entry point
└── tests/            # Test files
```

---

## What's Next?

Now that you can write basic programs, continue with:

- [IDE Setup](ide-setup.md) - Configure your editor for Desi
- [Introduction Tutorial](../tutorials/intro.md) - Learn Desi in depth
- [Variables & Types](../tutorials/variables.md) - All about types
