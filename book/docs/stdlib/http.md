# HTTP Module

The `http` module provides a built-in HTTP/HTTPS client for making web requests. No external dependencies required — it ships with the Desi compiler.

## Quick Start

```desi
import http

let resp = http.get("https://example.com")
print(resp.status)    # 200
print(resp.body)      # HTML content
```

## Making Requests

### GET

```desi
let resp = http.get("https://api.example.com/users")
print(resp.body)
```

### POST

```desi
# With a string body
let resp = http.post("https://api.example.com/users", '{"name": "alice"}')

# With auto-JSON serialization
let resp = http.post_json("https://api.example.com/users", {"name": "alice", "age": 30})
```

### PUT / PATCH

```desi
let resp = http.put("https://api.example.com/users/1", '{"name": "bob"}')
let resp = http.patch("https://api.example.com/users/1", '{"name": "bob"}')

# With auto-JSON serialization
let resp = http.put_json("https://api.example.com/users/1", {"name": "bob"})
let resp = http.patch_json("https://api.example.com/users/1", {"status": "active"})
```

### DELETE / HEAD / OPTIONS

```desi
let resp = http.delete("https://api.example.com/users/1")
let resp = http.head("https://example.com")
let resp = http.options("https://api.example.com")
```

### Custom Requests

Use `request()` for full control over method, body, and headers:

```desi
let resp = http.request(
    "POST",
    "https://api.example.com/data",
    '{"key": "value"}',
    "Authorization: Bearer my-token\r\nContent-Type: application/json\r\n"
)
```

## Response Object

Every request returns a `Response` with three fields:

| Field | Type | Description |
|-------|------|-------------|
| `status` | `int` | HTTP status code (200, 404, 500, etc.) |
| `body` | `str` | Response body text |
| `headers` | `str` | Raw response headers |

```desi
let resp = http.get("https://httpbin.org/get")

if resp.status == 200:
    print("Success!")
    print(resp.body)
```

## URL Encoding

Encode and decode URL components per RFC 3986:

```desi
let encoded = http.url_encode("hello world")     # "hello%20world"
let decoded = http.url_decode("hello%20world")    # "hello world"
```

## API Reference

### Request Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `get` | `(url: str) -> Response` | GET request |
| `post` | `(url: str, body: str) -> Response` | POST with string body |
| `post_json` | `(url: str, data: Any) -> Response` | POST with auto-JSON body |
| `put` | `(url: str, body: str) -> Response` | PUT with string body |
| `put_json` | `(url: str, data: Any) -> Response` | PUT with auto-JSON body |
| `patch` | `(url: str, body: str) -> Response` | PATCH with string body |
| `patch_json` | `(url: str, data: Any) -> Response` | PATCH with auto-JSON body |
| `delete` | `(url: str) -> Response` | DELETE request |
| `head` | `(url: str) -> Response` | HEAD request |
| `options` | `(url: str) -> Response` | OPTIONS request |
| `request` | `(method: str, url: str, body: str, headers: str) -> Response` | Custom request |

### Utility Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `url_encode` | `(s: str) -> str` | URL-encode a string (RFC 3986) |
| `url_decode` | `(s: str) -> str` | URL-decode a string |

## Features

- **HTTP and HTTPS** — TLS via OpenSSL (macOS/Linux)
- **IPv4 and IPv6** — dual-stack support
- **Redirects** — automatic following (301, 302, 303, 307, 308)
- **Chunked encoding** — handles `Transfer-Encoding: chunked`
- **No dependencies** — ships with the compiler, no pip/npm needed

## HTTPS Requirements

HTTPS requires OpenSSL to be installed:

- **macOS**: `brew install openssl` (auto-detected from Homebrew)
- **Linux**: Usually pre-installed (`libssl-dev` package)
- **Windows**: HTTP only in Phase 1

If OpenSSL is not found, HTTP requests still work; only HTTPS will fail with a connection error.

## See Also

- [JSON Module](json.md) — For parsing JSON responses
- [Strings Module](strings.md) — For string manipulation
