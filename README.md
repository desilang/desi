<p align="center">
  <img src="book/docs/assets/logo.svg" alt="Desi" width="120">
  <br>
  <strong>Desi</strong>
  <br>
  A compiled language with Python's clarity, Rust's safety, and batteries included.
</p>

<p align="center">
  <a href="https://desilang.org"><img src="https://img.shields.io/badge/docs-desilang.org-blue"></a>
  <a href="CHANGELOG.md"><img src="https://img.shields.io/badge/version-0.1.0-green"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-lightgrey"></a>
</p>

---

```desi
import http

let resp = http.get("https://api.example.com/users")
let users = resp.json()

for user in users:
    print(f"{user['name']} — {user['email']}")
```

Desi compiles to native machine code via LLVM. No garbage collector, no runtime VM, no system dependencies. Ship a single binary.

## Install

```sh
curl -sSL https://desilang.org/install.sh | sh
```

Or build from source — see [BUILDING.md](BUILDING.md) for full instructions:

```sh
# macOS / Linux (installs prerequisites automatically)
git clone https://github.com/desilang/desi
cd desi && ./bootstrap.sh

# Windows (installs prerequisites automatically)
git clone https://github.com/desilang/desi
cd desi; .\bootstrap.ps1
```

`bootstrap.sh` checks for Go, LLVM/Clang, OpenSSL, and Make, installs
whatever is missing via Homebrew/apt/dnf/pacman, then builds. If you already
have the toolchain, `make` on its own is enough.

## Quick Start

```sh
# Create a project
mkdir hello && cd hello
echo 'print("Hello, Desi!")' > main.desi

# Compile and run
desic run main.desi
```

## Why Desi?

Desi takes the best ideas from Python, Rust, Go, Elixir, and Django — and combines them into one language where everything works out of the box:

- **Python's readability** — Indentation-based, no braces, no semicolons
- **Rust's safety** — Move semantics, borrow checking, `Option<T>` and `Result<T, E>`, no null
- **Go's concurrency** — Channels, `select`, structured concurrency with supervisors
- **Django's ORM** — QuerySets, field lookups, migrations, signals — all built-in
- **LLVM performance** — Compiles to native code, ships as a single binary

## A Taste of Desi

```desi
# Web server with routing
import http

http.get("/", lambda req:
    http.text("Hello, World!"))

http.get("/users/:id", lambda req:
    let id = req.param("id")
    let user = User.objects.get("id", id)
    http.json({"name": user.name, "email": user.email}))

http.listen(8080)
```

```desi
# Django-style ORM — no setup, no dependencies
import db

@model
class User:
    name: str
    email: str
    age: int

db.connect("postgres://localhost/myapp")
db.create_table(User)

let adults = User.objects.filter("age__gte", "18").order_by("name").all()
for user in adults:
    print(f"{user.name}: {user.email}")
```

```desi
# Pattern matching with exhaustiveness checking
enum Shape:
    Circle(float)
    Rect(float, float)
    Triangle(float, float, float)

def area(s: Shape) -> float:
    match s:
        Shape.Circle(r):
            return 3.14159 * r * r
        Shape.Rect(w, h):
            return w * h
        Shape.Triangle(a, b, c):
            let s = (a + b + c) / 2.0
            return (s * (s-a) * (s-b) * (s-c)) ** 0.5
```

## Batteries-Included Standard Library

Everything below ships with the compiler. **No package manager needed.**

| Category | Modules |
|----------|---------|
| **Web & Network** | `http` (client + server), `websocket`, `net` (TCP/UDP), `url`, `tls` |
| **Database** | PostgreSQL, MySQL, SQLite3, Redis — Django-style ORM with migrations |
| **Data Formats** | `json`, `toml`, `yaml`, `csv`, `ini`, `template` |
| **Security** | `hash` (SHA-256, bcrypt, HMAC), `jwt`, `uuid`, `base64`, `encoding` |
| **CLI & System** | `args`, `cli`, `process`, `signal`, `shell`, `os`, `fs`, `path`, `env`, `dotenv` |
| **Concurrency** | `mutex`, `rwlock`, `channel`, `taskgroup`, `supervisor`, `semaphore`, `atomic`, `future` |
| **Utilities** | `strings`, `fmt`, `re`, `math`, `random`, `datetime`, `time`, `color`, `log`, `validate` |
| **Data Structures** | `collections` (deque, counter, stack, queue), `bytes`, `cache` (LRU, TTL) |
| **Dev Tools** | `testing`, `diff`, `table`, `mime`, `compress` |

**50 modules, 74 C runtime files, zero external dependencies.**

## Tooling

| Tool | Command | Description |
|------|---------|-------------|
| **Compiler** | `desic build` | Compile a project to a native binary |
| **Runner** | `desic run` | Compile and run in one step |
| **Test Runner** | `desic test` | Run `*_test.desi` files |
| **Formatter** | `desifmt` | Auto-format Desi source code |
| **REPL** | `desirepl` | Interactive Desi shell |
| **Language Server** | `desilsp` | Full LSP for any editor |
| **Hot Reload** | `desic watch` | Recompile on file change |

## IDE Support

| Editor | Setup |
|--------|-------|
| **VS Code** | `cd editors/vscode && npm install` then F5 |
| **IntelliJ/IDEA** | `cd editors/intellij && ./gradlew buildPlugin` |
| **Any LSP Editor** | Configure to run `desilsp` via stdio |

**LSP Features:** Hover, completion, go-to-definition, find references, rename, code actions, formatting, semantic highlighting, inlay hints, diagnostics.

## Platform Support

| Platform | Status |
|----------|--------|
| macOS (x86_64, arm64) | ✅ Fully supported |
| Linux (x86_64, arm64) | ✅ Fully supported |
| Windows (x86_64) | ✅ Native build + cross-compile |

## Documentation

- **[Getting Started](https://desilang.org/getting-started/)** — First steps with Desi
- **[Standard Library Reference](https://desilang.org/stdlib/)** — All 50 modules documented
- **[Language Guide](https://desilang.org/guide/)** — Syntax, types, concurrency, memory
- **[Why Desi?](KILLER_FEATURES.md)** — Design philosophy and killer features
- **[Changelog](CHANGELOG.md)** — Release history

## Contributing

```sh
# Run the full test suite
go test ./...

# Run example tests (484 programs with expected output)
./test_examples.sh

# Build everything from scratch
make clean && make
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — see [LICENSE](LICENSE).
