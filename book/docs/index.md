# The Desi Book

<div style="text-align: center; margin: 2rem 0;">
  <h2 style="color: #FF6B35;">A Modern, Python-Inspired Systems Language</h2>
</div>

---

## What is Desi?

**Desi** is a programming language that combines the elegance of Python with the performance of systems languages. It's designed for developers who want:

- 🐍 **Python-like syntax** - Familiar, readable, productive
- ⚡ **Native performance** - Compiles to LLVM, runs fast
- 🔒 **Memory safety** - Arena allocators, RAII, no garbage collector
- 🎯 **Type safety** - Strong typing with inference

---

## Quick Example

```python
# Hello, World in Desi
def main() -> int:
    print("Namaste, World!")
    0
```

A more interesting example:

```python
class Counter:
    pub mut count: int
    
    pub def __new__(self, start: int):
        self.count = start
    
    pub def increment(self):
        self.count = self.count + 1
    
    pub def get(self) -> int:
        return self.count

def main() -> int:
    let c = Counter(0)
    c.increment()
    c.increment()
    print(c.get())  # Prints: 2
    0
```

---

## Why Desi?

### For Python Developers

If you love Python but need more performance, Desi gives you:

- Same indentation-based syntax
- Similar class and function definitions
- Native compilation instead of interpretation

### For Systems Programmers

If you need low-level control but want better ergonomics:

- No manual memory management (arenas handle it)
- Rich standard library of builtins
- Modern language features (generics, pattern matching)

---

## Getting Started

Ready to try Desi? Start here:

<div class="grid cards" markdown>

- :material-download: **[Installation](getting-started/install.md)**
  
  Install Desi on your system

- :material-rocket-launch: **[First Program](getting-started/first-program.md)**
  
  Write your first Desi program

- :material-book-open: **[Tutorials](tutorials/intro.md)**
  
  Learn Desi step by step

</div>

---

## Language Features

| Feature | Description |
|---------|-------------|
| **Variables** | `let` and `var` with type inference |
| **Functions** | `def` with optional type annotations |
| **Classes** | Full OOP with inheritance, methods, properties |
| **Generics** | Type parameters for classes and functions |
| **Collections** | Lists, dicts, sets with Python-like syntax |
| **Pattern Matching** | `match` expressions with exhaustive checking |
| **Error Handling** | `Option` and `Result` types with `?` operator |

---

## Status

Desi is currently in active development. Version **0.1.0** is being prepared for initial release.

!!! warning "Pre-release"
    This documentation is for the upcoming v0.1.0 release. Some features may change.

---

<div style="text-align: center; margin-top: 3rem;">
  <em>"Desi" - meaning "local" or "native" in Hindi, representing code that runs natively on your machine.</em>
</div>
