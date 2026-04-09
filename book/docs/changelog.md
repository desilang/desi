# Changelog

All notable changes to Desi will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

**Standard Library — New Modules**

- `io` module: `input()`, `read_all()`, `eprint()`, `ewrite()`, `flush()`, `has_input()`
- `sys` module: `version()`, `maxsize()`, `byteorder()`, `sizeof_ptr()`, `recursion_limit()`, `call_depth()`
- `crypto` module: `sha256()`, `sha512()`, `md5()`, `sha1()`, `hmac_sha256()`, `constant_time_equal()`, `token_hex()`, `hash_password()`, `verify_password()`
- `sqlite3` module: Embedded SQLite3 database — zero external dependencies
  - `connect()`, `disconnect()`, `execute()`, `query()`, `fetch_next()`, `get()`, `get_field()`
  - Transactions: `begin()`, `commit()`, `rollback()`
  - Metadata: `last_insert_id()`, `changes()`, `col_count()`, `col_name()`, `row_count()`

**Standard Library — New Functions**

- `strings`: Case convention converters — `camel_case()`, `pascal_case()`, `snake_case()`, `kebab_case()`, `screaming_snake()`
- `strings`: Text utilities — `word_wrap()`, `is_numeric()`, `dedent()`, `abbreviate()`
- `os`: `home_dir()`, `temp_dir()`, `which()`, `sleep_ms()`

### Changed

- `sqlite3`: Renamed `open()`/`close()` to `connect()`/`disconnect()` (avoids compiler builtin collision)
- `sys`: Replaced `pub extern stdout/stderr` with function-based API (compiler doesn't support pub extern variables)

### Fixed

- Makefile: Fixed archive merge that lost libmpdec symbols (`mpd_qset_string`) — extracted to subdirectory to avoid `io.o` filename collision
- Test 298: Fixed `pub from import` test to use `strings.upper` instead of invalid `io.print`
- All 451 tests now pass (0 failures)


---

## [0.1.0] - TBD

### Added

**Core Language**

- Variables with `let` (immutable) and `var` (mutable)
- Type inference with optional annotations
- Functions with `def`
- Classes with inheritance
- Structs and enums
- Pattern matching with `match`

**Type System**

- Basic types: `int`, `float`, `str`, `bool`
- Sized integers: `i32`, `i64`, `u32`, `u64`
- Collections: `list<T>`, `dict<K,V>`, `set<T>`
- Generic classes and functions
- `Option<T>` and `Result<T,E>` types

**Builtins**

- I/O: `print()`
- Collections: `len()`, `range()`, `enumerate()`
- Aggregation: `sum()`, `min()`, `max()`
- Boolean: `any()`, `all()`
- Transformation: `map()`, `filter()`, `reduce()`
- Ordering: `sorted()`, `reversed()`
- Combining: `zip()`

**Classes**

- Constructors (`__new__`)
- Methods and properties
- Visibility (`pub`)
- Static and class methods
- Operator overloading
- Inheritance

**Memory Management**

- Arena allocators
- RAII with `using`

**Standard Library**

- Core: `strings`, `os`, `io`, `fs`, `path`, `sys`, `args`, `env`
- Data: `json`, `toml`, `csv`, `fmt`, `template`, `encoding`, `base64`, `compress`
- Math: `math`, `random`
- Time: `time`, `datetime`
- Network: `http`, `net`
- Crypto: `crypto`, `hash`, `uuid`
- Database: `db` (ORM), `sqlite3`, `redis`
- Concurrency: `sync` (Mutex, RwLock, Channel, TaskGroup, Supervisor)
- Utilities: `log`, `re`, `color`, `validate`


---

## Version History

| Version | Release Date | Highlights |
|---------|--------------|------------|
| 0.1.0 | TBD | Initial release |

---

## Upgrade Guides

### Upgrading to 0.2.0 (Future)

!!! note "Coming Soon"
    Upgrade guide will be added when 0.2.0 is released.

When upgrading between major versions, check this section for breaking changes and migration steps.
