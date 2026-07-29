# Introduction to Desi

Welcome to the Desi tutorial series! This guide will teach you Desi from the ground up.

---

## What is Desi?

Desi is a programming language designed for developers who:

- Love Python's syntax and readability
- Need native performance
- Want memory safety without garbage collection

The name "Desi" (देसी) means "local" or "native" in Hindi - representing code that compiles to native machine code.

---

## Design Philosophy

### Python-Inspired Syntax

Desi uses indentation-based syntax like Python:

```desi
def factorial(n: int) -> int:
    if n <= 1:
        return 1
    return n * factorial(n - 1)
```

### Strong Static Typing

Types are checked at compile time, catching errors early:

```desi
let name: str = "Desi"
let count: int = 42
let pi: float = 3.14159
```

Type inference makes this less verbose:

```desi
let name = "Desi"    # Inferred as str
let count = 42       # Inferred as int
```

### Memory Safety

Desi uses arenas and RAII for memory management - no garbage collector pauses, no manual memory management:

```desi
using arena = Arena():
    let data = arena.alloc(1024)  # Allocated in arena
# Memory automatically freed when arena goes out of scope
```

---

## Desi vs Python

| Feature | Desi | Python |
|---------|------|--------|
| Syntax | Indentation-based | Indentation-based |
| Typing | Static, compile-time | Dynamic, runtime |
| Execution | Compiled (LLVM) | Interpreted |
| Performance | Native speed | Slower |
| Memory | Arenas, RAII | Garbage collected |

---

## A Complete Example

Here's a taste of what Desi code looks like:

```desi
# A simple class with generics
class Stack<T>:
    pub mut items: list<T>

    pub def __new__(self):
        self.items = []

    pub def push(self, item: T):
        self.items.append(item)

    pub def pop(self) -> T:
        return self.items.pop()

    pub def is_empty(self) -> bool:
        return len(self.items) == 0

def main() -> int:
    # __new__ takes no argument mentioning T, so state it with turbofish
    let stack = Stack::<int>()

    stack.push(1)
    stack.push(2)
    stack.push(3)

    while not stack.is_empty():
        print(str(stack.pop()))
    return 0
```

---

## Contents of This Tutorial

This tutorial is organized into chapters:

1. **Variables & Types** - Declaring variables, understanding types
2. **Functions** - Defining and calling functions
3. **Collections** - Lists, dicts, and sets
4. **Classes** - Object-oriented programming
5. **Generics** - Type parameters

Each chapter builds on the previous, so we recommend following them in order.

---

## Ready?

Let's start with [Variables & Types](variables.md)!
