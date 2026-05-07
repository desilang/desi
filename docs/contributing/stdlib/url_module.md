# URL Module Implementation

Internal documentation for the `url` standard library module.

## Architecture

```
compiler/lib/url.desi     → Desi API (safe wrappers)
compiler/runtime/url.c    → C runtime (RFC 3986 parser)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__url_parse` | `url.parse` | `(const char*) → UrlHandle*` |
| `__url_get_scheme` | `url.get_scheme` | `(UrlHandle*) → char*` |
| `__url_get_host` | `url.get_host` | `(UrlHandle*) → char*` |
| `__url_get_port` | `url.get_port` | `(UrlHandle*) → int32` |
| `__url_get_path` | `url.get_path` | `(UrlHandle*) → char*` |
| `__url_get_query` | `url.get_query` | `(UrlHandle*) → char*` |
| `__url_get_fragment` | `url.get_fragment` | `(UrlHandle*) → char*` |
| `__url_encode` | `url.encode` | `(const char*) → char*` |
| `__url_decode` | `url.decode` | `(const char*) → char*` |
| `__url_query_get` | `url.query_get` | `(const char*, const char*) → char*` |
| `__url_build` | `url.build` | `(scheme, host, port, path, query, fragment) → char*` |
| `__url_join` | `url.join` | `(const char*, const char*) → char*` |
| `__url_free` | `url.free` | `(UrlHandle*) → int32` |

## Handle Pattern

The `url.parse()` function returns a `cptr` handle (an opaque pointer to a heap-allocated C struct). All getter functions take this handle as the first argument. The handle must be freed with `url.free()`.

**Important**: All handles use `cptr` (not `int`) to avoid 64-bit pointer truncation.

## Test Coverage

- `examples/500_url_module.desi` — parse, getters, encode/decode, query_get, build, join
