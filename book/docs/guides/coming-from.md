# Coming From Other Languages

This guide maps familiar patterns from Python, Go, and Rust to their Desi equivalents.

---

## Coming From Python

If you know Python, you already know most of Desi's syntax. The key differences are:

### What Stays the Same

| Feature | Python | Desi |
|---------|--------|------|
| Indentation | ✅ Significant | ✅ Significant (tabs only) |
| f-strings | `f"Hello {name}"` | `f"Hello {name}"` |
| List comprehensions | `[x*2 for x in items]` | `[x*2 for x in items]` |
| Slices | `items[1:3]` | `items[1:3]` |
| Print | `print("hello")` | `print("hello")` |
| Decorators | `@decorator` | `@decorator` |
| Classes | `class Foo:` | `class Foo:` |
| Match | `match x:` | `match x:` |

### What's Different

```desi
# Variables are immutable by default
let x = 10          # immutable (Python: x = 10)
var y = 20          # mutable   (Python: x = 10, then x = 30)

# Type annotations are available
let name: str = "Desi"
let age: int = 1

# Functions use 'def' but have explicit return types
def greet(name: str) -> str:
    return f"Hello, {name}!"

# No None — use Option<T> instead
def find(items: list<str>, target: str) -> Option<str>:
    for item in items:
        if item == target:
            return Some(item)
    return None_
```

### pip → Nothing

Desi has no package manager. Everything ships with the compiler:

| Python (pip install) | Desi (import) |
|---------------------|---------------|
| `requests` | `import http` |
| `psycopg2` | `import db` |
| `redis` | `import redis` |
| `flask` / `fastapi` | `import http` (server built-in) |
| `pydantic` | `import validate` |
| `bcrypt` | `import hash` |
| `python-dotenv` | `import dotenv` |
| `pyyaml` | `import yaml` |

---

## Coming From Go

Desi borrows Go's concurrency model but adds Python's readability.

### Key Mappings

| Go | Desi |
|----|------|
| `func main()` | `def main():` |
| `var x int = 10` | `let x: int = 10` |
| `x := 10` | `let x = 10` |
| `fmt.Println("hi")` | `print("hi")` |
| `if err != nil` | `match result:` with `Ok(v)` / `Err(e)` |
| `go func(){}()` | `spawn lambda: ...` |
| `ch := make(chan int)` | `let ch = Channel<int>()` |
| `select { case ... }` | `select:` |
| `struct { Name string }` | `struct Point: x: float, y: float` |
| `interface {}` | `trait Printable:` |

### Error Handling

```go
// Go: error returns
file, err := os.Open("data.txt")
if err != nil {
    log.Fatal(err)
}
```

```desi
# Desi: Result<T, E>
let content = fs.read("data.txt")
match content:
    Ok(data):
        print(data)
    Err(e):
        print(f"Error: {e}")

# Or use the ? operator (propagates errors)
let data = fs.read("data.txt")?
```

### Concurrency

```go
// Go
ch := make(chan string)
go func() {
    ch <- "hello"
}()
msg := <-ch
```

```desi
# Desi
let ch = Channel<str>()
spawn lambda:
    ch.send("hello")
let msg = ch.recv()
```

---

## Coming From Rust

Desi shares Rust's safety model but with Python's syntax.

### Key Mappings

| Rust | Desi |
|------|------|
| `let x = 10;` | `let x = 10` |
| `let mut x = 10;` | `var x = 10` |
| `fn greet(name: &str) -> String` | `def greet(name: str) -> str:` |
| `Option<T>` | `Option<T>` |
| `Result<T, E>` | `Result<T, E>` |
| `Some(v)` / `None` | `Some(v)` / `None_` |
| `Ok(v)` / `Err(e)` | `Ok(v)` / `Err(e)` |
| `match x { ... }` | `match x:` |
| `enum Color { Red, Green, Blue }` | `enum Color: Red, Green, Blue` |
| `struct Point { x: f64, y: f64 }` | `struct Point: x: float, y: float` |
| `impl Foo { ... }` | `impl Foo:` |
| `trait Display { ... }` | `trait Display:` |
| `Vec<T>` | `list<T>` |
| `HashMap<K, V>` | `dict<K, V>` |
| `println!("{}", x)` | `print(f"{x}")` |

### What You Won't Miss

- **No semicolons** — Desi uses newlines
- **No braces** — Desi uses indentation
- **No `&`, `*`, `'a` lifetimes** — Desi handles borrowing automatically
- **No `cargo add`** — Everything is in the standard library
- **No macro syntax** — Decorators serve a similar role

### Pattern Matching

```rust
// Rust
match shape {
    Shape::Circle(r) => std::f64::consts::PI * r * r,
    Shape::Rect(w, h) => w * h,
}
```

```desi
# Desi
match shape:
    Shape.Circle(r):
        3.14159 * r * r
    Shape.Rect(w, h):
        w * h
```

### Generics

```rust
// Rust
fn first<T>(items: &[T]) -> Option<&T> {
    items.first()
}
```

```desi
# Desi
def first<T>(items: list<T>) -> Option<T>:
    if items.len() > 0:
        return Some(items[0])
    return None_
```

---

## Quick Comparison Table

| Feature | Python | Go | Rust | Desi |
|---------|--------|-----|------|------|
| Syntax | Indentation | Braces | Braces | Indentation |
| Types | Dynamic | Static | Static | Static |
| Null safety | `None` | `nil` | `Option<T>` | `Option<T>` |
| Error handling | Exceptions | `error` | `Result<T,E>` | `Result<T,E>` |
| Memory | GC | GC | Ownership | Move + Borrow |
| Concurrency | `asyncio` | Goroutines | `tokio` | Channels + Supervisors |
| Package manager | pip | go get | cargo | Built-in (none needed) |
| Compile target | Bytecode | Machine code | Machine code | Machine code (LLVM) |
| Binary size | N/A | ~10MB | ~1MB | ~100KB |
