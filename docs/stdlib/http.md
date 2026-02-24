# HTTP Module Implementation

This document covers the internal implementation details of the `http` module for contributors.

## Architecture

```
compiler/lib/http/__mod.desi           ← Desi public API (Response struct + wrappers)
         ↓ (extern "C" calls)
compiler/runtime/http.c                ← Core HTTP logic (URL parsing, request/response)
         ↓ (includes)
compiler/runtime/http/http_internal.h  ← Shared types (Buffer, Connection, ParsedURL)
compiler/runtime/http/tls_apple.h      ← macOS TLS (OpenSSL via Homebrew)
compiler/runtime/http/tls_openssl.h    ← Linux TLS (system OpenSSL)
compiler/runtime/http/tls_win.h        ← Windows TLS (HTTP-only stub)
```

## Why This Design?

1. **Self-contained**: No external dependencies (libcurl, etc.) — ships with the compiler
2. **Cross-platform**: POSIX sockets + platform-specific TLS backends
3. **Modular**: TLS code split into per-platform headers for easy maintenance
4. **Simple API**: Opaque C response → structured Desi `Response` type

## C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_get(url)` | HTTP GET request |
| `__http_post(url, body)` | HTTP POST with body |
| `__http_put(url, body)` | HTTP PUT with body |
| `__http_patch(url, body)` | HTTP PATCH with body |
| `__http_delete(url)` | HTTP DELETE request |
| `__http_head(url)` | HTTP HEAD request |
| `__http_options(url)` | HTTP OPTIONS request |
| `__http_request(method, url, body, headers)` | Custom method with headers |
| `__http_response_status(resp)` | Extract status code (int) |
| `__http_response_body(resp)` | Extract body (string) |
| `__http_response_headers(resp)` | Extract raw headers (string) |
| `__http_url_encode(s)` | RFC 3986 URL encoding |
| `__http_url_decode(s)` | URL decoding |

## Response Struct

The Desi `Response` struct wraps C response fields:

```desi
struct Response:
    pub status: int     # HTTP status code (200, 404, etc.)
    pub body: str       # Response body text
    pub headers: str    # Raw response headers
```

**Design Note:** The C side returns an opaque `HttpResponse*` pointer. Each Desi wrapper function extracts all three fields via `__http_response_status/body/headers` before returning a structured `Response`.

## TLS Backends

### macOS (`tls_apple.h`)

Uses OpenSSL via Homebrew (auto-detected). SecureTransport was deprecated in macOS 10.15 and returns `errSecParam (-50)` on modern macOS.

Detection chain:
1. `__has_include(<openssl/ssl.h>)` → includes `tls_openssl.h` (same implementation)
2. If not found → HTTP-only mode (HTTPS returns error)

### Linux (`tls_openssl.h`)

Uses system OpenSSL/LibreSSL directly. Available on virtually all distributions.

### Windows (`tls_win.h`)

Stub — HTTP works, HTTPS returns error. Full Schannel implementation deferred.

## Build System Integration

### Makefile

```makefile
# Auto-detects OpenSSL via Homebrew or system paths
OPENSSL_PREFIX := $(shell brew --prefix openssl 2>/dev/null || echo "")
OPENSSL_CFLAGS = -I$(OPENSSL_PREFIX)/include
OPENSSL_LDFLAGS = -L$(OPENSSL_PREFIX)/lib -lssl -lcrypto
```

### Linker (`run_build_cmd.go`)

```go
// detectOpenSSLPrefix() checks:
// 1. brew --prefix openssl (macOS Homebrew)
// 2. /usr, /usr/local, /opt/homebrew (system paths)
// Returns prefix for -L and -I flags
```

## Compiler Changes

The HTTP module required fixing struct export resolution in the resolver:

1. **`exports.go:CollectExports`** — Added struct declaration collection so functions returning module-defined structs (like `Response`) are properly exported
2. **`exports.go:resolveTypeName`** — Added struct type lookup alongside classes and built-ins
3. **Field types** — Struct fields now resolve via `types.FromName` so the LLVM IR generates correct load instructions

## Platform Differences

| Feature | POSIX (macOS/Linux) | Windows |
|---------|---------------------|---------|
| Sockets | `socket()`, `connect()` | `winsock2.h` (`WSAStartup`) |
| DNS | `getaddrinfo()` | `getaddrinfo()` |
| TLS | OpenSSL `SSL_*` | Stub (HTTP only) |
| Close | `close(fd)` | `closesocket(fd)` |

## Adding New Features

1. Add C implementation in `compiler/runtime/http.c`
2. Add `@extern("C")` binding in `compiler/lib/http/__mod.desi`
3. Add `pub def` wrapper function with documentation comment
4. Update OpenSSL/platform flags if needed in `Makefile` and `run_build_cmd.go`
5. Rebuild with `make clean && make`
6. Test with `desic run test_file.desi`

## Known Limitations (Phase 1)

- No timeouts (connections may hang)
- No proxy support
- No cookie persistence
- No custom TLS certificate configuration
- No HTTP/2 support
- `post_json()`/`put_json()`/`patch_json()` auto-serialize via `__json_stringify`, but mixed-type dicts passed as `Any` may not serialize correctly at the C level
