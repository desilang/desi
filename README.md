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
import json

def main() -> int:
    let resp = http.get("https://api.example.com/users")
    let data = http.parse_json(resp.body)

    let name = json.get_string(json.object_get(data, "name"))
    print(name)
    return 0
```

Desi compiles to native machine code via LLVM. No garbage collector, no runtime VM, no interpreter. The Desi runtime is linked statically, so what you ship is one executable — on Linux and macOS it also links the system OpenSSL, which HTTPS and the database drivers use.

## Install

```sh
curl -sSL https://desilang.org/install.sh | sh
```

Windows:

```powershell
irm https://desilang.org/install.ps1 | iex
```

Desi compiles through LLVM, so the installer also expects `clang` on your
PATH — it tells you how to get it if it is missing. On Windows you need the
MSVC linker too, from the Visual Studio Build Tools "Desktop development with
C++" workload.

The release binaries are not code-signed yet, so Windows SmartScreen may warn
on first run ("More info" → "Run anyway"), and macOS Gatekeeper may need
`xattr -d com.apple.quarantine <file>`. Building from source avoids both.

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
# HTTP server
import http

def main() -> int:
    let srv = http.server(8080)
    http.listen(srv)
    return 0
```

```desi
# Django-style ORM — no setup, no dependencies
import db

def main() -> int:
    db.connect("postgres", "127.0.0.1", 5432, "myapp", "postgres", "secret")

    # QuerySets are independent and chain like Django's
    let adults = db.objects("users").filter("age__gte", "18").order_by("name")
    print(str(adults.count()))
    return 0
```

```desi
# Pattern matching with exhaustiveness checking
enum Shape:
    Circle: float
    Square: float

def area(s: Shape) -> float:
    let result = match s:
        Shape.Circle(r): 3.14159 * r * r
        Shape.Square(w): w * w
    return result
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

**42 stdlib modules, 89 C runtime files. The only external dependency is the system OpenSSL, used by HTTPS and the database drivers.**

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
| macOS (arm64, x86_64) | ✅ Release binaries, full test suite in CI |
| Linux (x86_64) | ✅ Release binaries, full test suite in CI |
| Windows (x86_64) | ✅ Release binaries, full test suite in CI |
| Linux (arm64) | ⚠️ Builds from source; no prebuilt binary or CI coverage yet |

Every release binary is built natively on its own platform and smoke-tested —
compiled and run — before it is published.

## Documentation

- **[Getting Started](https://desilang.org/getting-started/)** — First steps with Desi
- **[Standard Library Reference](https://desilang.org/stdlib/)** — every module documented
- **[Language Guide](https://desilang.org/guide/)** — Syntax, types, concurrency, memory
- **[Why Desi?](KILLER_FEATURES.md)** — Design philosophy and killer features
- **[Changelog](CHANGELOG.md)** — Release history

## Contributing

```sh
# Run the full test suite
go test ./...

# Run example tests (502 programs with expected output)
./test_examples.sh

# Build everything from scratch
make clean && make
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — see [LICENSE](LICENSE).
