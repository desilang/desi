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

## Known Limitations (Phase 1 Client)

- No proxy support
- No cookie persistence
- No custom TLS certificate configuration
- No HTTP/2 support
- `post_json()`/`put_json()`/`patch_json()` auto-serialize via `__json_stringify`, but mixed-type dicts passed as `Any` may not serialize correctly at the C level

---

## HTTP Server

The http module includes a built-in HTTP server for building web applications and APIs.

### Architecture

```
http.__mod.desi         ← Desi API: server(), serve(), text(), json(), etc.
     ↓ (extern "C" calls)
http.c                  ← C runtime: accept loop, request parsing, response writing
     ↓ (lowerer intercepts)
lower_call.go           ← Intercepts http.serve(srv, handler) to emit handler registration
```

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_server_new(port)` | Create server bound to port |
| `__http_server_run(server)` | Start accept loop (blocks) |
| `__http_server_set_handler(fn)` | Register Desi function as request handler |
| `__http_resp_new(status, body, ct)` | Build HTTP response |
| `__http_req_method(req)` | Get request method |
| `__http_req_path(req)` | Get request path |
| `__http_req_body(req)` | Get request body |
| `__http_req_header(req, name)` | Get specific header |
| `__http_req_query(req)` | Get query string |
| `__http_req_param(req, key)` | Get query parameter |
| `__http_req_path_param(req, name)` | Get path parameter (`:id`) |
| `__http_server_route(srv, method, path, fn)` | Register route handler |
| `__http_server_static(srv, prefix, dir)` | Register static file serving |
| `__http_server_max_body(srv, bytes)` | Set max request body size |
| `__http_server_use(srv, fn)` | Register middleware |
| `__http_server_rate_limit(srv, max, window)` | Configure rate limiting |
| `__http_resp_header(resp, key, value)` | Add custom response header |
| `__http_server_cors(srv, origin)` | Configure CORS (default: `*`) |
| `__http_req_cookie(req, name)` | Read cookie from request |
| `__http_resp_cookie(resp, name, value, max_age)` | Set cookie on response |
| `__ws_send(fd, msg)` | Send text to WS connection |
| `__ws_broadcast(msg)` | Broadcast to all WS clients |
| `__ws_close(fd)` | Close WS connection |
| `__ws_join(fd, room)` | Join a WS room |
| `__ws_leave(fd, room)` | Leave a WS room |
| `__ws_to_room(room, msg)` | Broadcast to WS room |
| `__ws_set_path(path)` | Set WS endpoint path |
| `__ws_set_on_message(fn)` | Register WS message handler |
| `__ws_set_on_open(fn)` | Register WS connect handler |
| `__ws_set_on_close(fn)` | Register WS disconnect handler |

### Response Builders

| Desi Function | C Content-Type |
|---------------|----------------|
| `http.text(status, body)` | `text/plain; charset=utf-8` |
| `http.json_text(status, body)` | `application/json` |
| `http.json(status, data)` | `application/json` (auto-serializes dict) |
| `http.html(status, body)` | `text/html; charset=utf-8` |
| `http.response(status, body, ct)` | Custom content type |

### Lowerer Integration

`http.serve(srv, handler)` is intercepted by the lowerer in `lower_call.go`:
1. Emits `__http_server_set_handler(@handler)` — registers the Desi function as a C callback
2. Emits `__http_server_run(srv)` — starts the accept loop

The Desi function body for `serve()` is never executed — it's a stub for the type checker.

---

## Concurrency Architecture

The HTTP server uses a **Supervisor thread pool** for concurrent request handling:

```
Main Thread                 Supervisor Pool (8 workers)
    │                           │
    ├── accept() ──→ submit ──→ Worker 1: handle_client()
    ├── accept() ──→ submit ──→ Worker 2: handle_client()
    ├── accept() ──→ submit ──→ Worker 3: handle_client()
    │   ...                     ...
    └── SIGINT ──→ supervisor_stop() ──→ drain + join
```

### Implementation (`http_server.c`)

- `ClientTask` struct bundles `HttpServer*` + `client_fd`
- `client_task_fn()` wrapper calls `handle_client()` then frees the task
- `__http_server_run()` creates a Supervisor with 8 workers (ONE_FOR_ONE)
- Each accepted connection is dispatched via `supervisor_submit()`
- Graceful shutdown: `supervisor_stop()` drains queue, joins workers

### Thread-Safety

- **`find_header_safe()`** — uses caller-provided stack buffer (not static)
- **Route table** — read-only during accept loop (set before `run()`)
- **`__desi_http_handler`** — global function pointer, set once before `run()`

---

## Query String & Form Helpers

| Desi Function | Purpose |
|---------------|---------|
| `http.build_query(params)` | Dict → URL-encoded query string |
| `http.post_form(url, data)` | POST with `application/x-www-form-urlencoded` |

### C Runtime

| C Function | Purpose |
|------------|---------|
| `__dict_to_query_str(data)` | Dict to `key=val&key=val` |

---

## Authentication Helpers

| Desi Function | Result |
|---------------|--------|
| `http.basic_auth(user, pass)` | `Authorization: Basic base64(user:pass)\r\n` |
| `http.bearer_auth(token)` | `Authorization: Bearer <token>\r\n` |

### C Runtime

| C Function | Purpose |
|------------|---------|
| `__base64_encode(s)` | Base64 encoding for Basic auth |

---

## Response Helpers

Pure Desi functions (no C runtime needed):

| Function | Returns |
|----------|---------|
| `http.is_ok(resp)` | `true` if status 200-299 |
| `http.is_redirect(resp)` | `true` if status 301/302/303/307/308 |
| `http.is_client_error(resp)` | `true` if status 400-499 |
| `http.is_server_error(resp)` | `true` if status 500-599 |

---

## Request Timeout Support

The `request_timeout()` function adds configurable timeouts:

| C Function | Purpose |
|------------|---------|
| `__http_request_timeout(method, url, body, headers, timeout_secs)` | Request with timeout |

Desi API:
```desi
let resp = http.request_timeout("GET", url, "", "", 10)  # 10s timeout
let resp = http.request_json("POST", url, data, 30)      # JSON + 30s timeout
```

---

## Adding New Features

1. Add C implementation in `compiler/runtime/http.c`
2. Add `@extern("C")` binding in `compiler/lib/http/__mod.desi`
3. Add `pub def` wrapper function with documentation comment
4. Update OpenSSL/platform flags if needed in `Makefile` and `run_build_cmd.go`
5. Rebuild with `make clean && make`
6. Test with `desic run test_file.desi`

---

## Routing API

Register handlers for specific HTTP methods and URL patterns:

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_server_route(srv, method, path, fn)` | Register route with method + path |

### Desi API

| Function | Description |
|----------|-------------|
| `http.get(srv, path, handler)` | Register GET handler |
| `http.post(srv, path, handler)` | Register POST handler |
| `http.put(srv, path, handler)` | Register PUT handler |
| `http.delete(srv, path, handler)` | Register DELETE handler |
| `http.patch(srv, path, handler)` | Register PATCH handler |
| `http.route(srv, method, path, handler)` | Register any method |

### Example

```desi
import http

def hello(req: Any) -> Any:
    return http.text(200, "Hello!")

def main() -> int:
    let srv = http.server(8080)
    http.get(srv, "/hello", hello)
    http.serve(srv)
    return 0
```

### Route Matching

Routes are matched in priority order:
1. **Exact match** — method + path
2. **Wildcard method** — `*` matches any method
3. **Path patterns** — `:param` segments (see below)

---

## Path Parameters

Use `:param` syntax in route patterns to capture URL segments:

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_req_path_param(req, name)` | Get captured path parameter |

### Desi API

```desi
def user_handler(req: Any) -> Any:
    let id = http.req_path_param(req, "id")
    return http.json(200, {"user_id": id})

http.get(srv, "/users/:id", user_handler)
# GET /users/42 → {"user_id":"42"}
```

Multiple parameters:

```desi
def post_handler(req: Any) -> Any:
    let category = http.req_path_param(req, "category")
    let slug = http.req_path_param(req, "slug")
    return http.json(200, {"category": category, "slug": slug})

http.get(srv, "/posts/:category/:slug", post_handler)
# GET /posts/tech/hello → {"category":"tech","slug":"hello"}
```

**Implementation:**
- `match_path_pattern()` splits pattern and path by `/`, matches segments
- Parameters stored in `HttpServerRequest.param_names[8]` / `param_values[8]`
- Max 8 parameters per route

---

## Query Parameters

Parse query string parameters from URLs:

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_req_param(req, key)` | Extract query parameter by key |

### Desi API

```desi
def search(req: Any) -> Any:
    let q = http.req_param(req, "q")       # ?q=desi
    let page = http.req_param(req, "page") # ?page=2
    return http.json(200, {"query": q, "page": page})

http.get(srv, "/search", search)
# GET /search?q=desi&page=2 → {"query":"desi","page":"2"}
```

Returns `""` if the parameter is not present.

---

## Request Body as JSON

Parse POST/PUT/PATCH request bodies as JSON:

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_req_json(req)` | Parse body via `__json_parse()`, returns `JsonNode*` |

### Desi API

```desi
import http
import json

def api_handler(req: Any) -> Any:
    let body = http.req_json(req)
    let name = json.get_string(json.object_get(body, "name"))
    return http.json(200, {"hello": name})

http.post(srv, "/api/users", api_handler)
```

Returns `none` if the body is empty or invalid JSON.

---

## Body Size Limits

Reject oversized request bodies before they consume server memory:

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_server_max_body(srv, max_bytes)` | Set max body size |

### Desi API

```desi
http.max_body(srv, 1024 * 512)   # 512KB max
```

- Default: 1MB (1,048,576 bytes)
- Oversized requests receive `413 Payload Too Large` with JSON error body
- Checked via `Content-Length` header before reading body

---

## Rate Limiting

Token-bucket rate limiting per client IP:

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_server_rate_limit(srv, max, window)` | Configure rate limiter |

### Desi API

```desi
http.rate_limit(srv, 60, 60)   # 60 requests per 60 seconds per IP
```

- Returns `429 Too Many Requests` with `Retry-After` header
- Token bucket refills based on elapsed time
- Tracked per IP address (up to 256 buckets)

---

## Middleware

Run functions before route handlers for logging, auth, etc.:

### Server C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__http_server_use(srv, fn)` | Register middleware function |

### Desi API

```desi
# Middleware: return none to pass through, return response to short-circuit
def auth_check(req: Any) -> Any:
    let token = http.req_header(req, "Authorization")
    if token == "":
        return http.json(401, {"error": "unauthorized"})
    return none   # pass through to route handler

def logger(req: Any) -> Any:
    print(http.req_method(req) + " " + http.req_path(req))
    return none

http.use(srv, logger)
http.use(srv, auth_check)
```

- Up to 16 middleware functions
- Executed in registration order
- Return `none` → continue chain
- Return response → short-circuit (skip remaining middleware + handler)

---

## Static File Serving

### Desi API

```desi
http.serve_static(srv, "/static", "./public")
# GET /static/style.css → serves ./public/style.css
```

- 25+ MIME types supported
- 1-hour `Cache-Control` headers
- **Security:** blocks `..`, `%2e`, backticks, backslashes → `403 Forbidden`

---

## Keep-Alive Connections

HTTP/1.1 keep-alive is enabled by default:

- **Idle timeout:** 15 seconds
- **Max requests per connection:** 100
- Respects `Connection: close` header
- Server logs: `[ka]` for keep-alive, `[close]` for closed

---

## Complete Server Example

```desi
import http
import json

def logger(req: Any) -> Any:
    print(http.req_method(req) + " " + http.req_path(req))
    return none

def greet(req: Any) -> Any:
    let name = http.req_param(req, "name")
    if name == "":
        name = "stranger"
    return http.text(200, "Hello, " + name + "!")

def create_user(req: Any) -> Any:
    let body = http.req_json(req)
    let name = json.get_string(json.object_get(body, "name"))
    let resp = http.json(201, {"created": name})
    http.set_cookie(resp, "session", "abc123", 3600)
    return resp

def user_profile(req: Any) -> Any:
    let id = http.req_path_param(req, "id")
    let session = http.get_cookie(req, "session")
    let resp = http.json(200, {"user_id": id, "session": session})
    http.header(resp, "X-Request-Id", "req-42")
    return resp

def main() -> int:
    let srv = http.server(8080)

    # Security
    http.max_body(srv, 1024 * 512)
    http.rate_limit(srv, 100, 60)
    http.cors(srv, "*")

    # Middleware
    http.use(srv, logger)

    # Routes
    http.get(srv, "/greet", greet)
    http.post(srv, "/users", create_user)
    http.get(srv, "/users/:id", user_profile)
    http.serve_static(srv, "/static", "./public")

    http.serve(srv)
    return 0
```

---

## Custom Response Headers

Add arbitrary headers to any response:

### Desi API

```desi
let resp = http.text(200, "Hello!")
http.header(resp, "X-Request-Id", "abc-123")
http.header(resp, "X-Custom", "value")
return resp
```

### C Runtime

| C Function | Purpose |
|------------|---------|
| `__http_resp_header(resp, key, value)` | Append `Key: Value\r\n` to response `extra_headers` |

---

## CORS (Cross-Origin Resource Sharing)

CORS is **enabled by default** with `Access-Control-Allow-Origin: *`. Override for specific origins:

### Desi API

```desi
# Allow all origins
http.cors(srv, "*")

# Allow specific origin
http.cors(srv, "https://example.com")
```

### Behavior

- **Preflight:** `OPTIONS` requests auto-return `204 No Content` with:
  - `Access-Control-Allow-Origin`, `Access-Control-Allow-Methods`, `Access-Control-Allow-Headers`
  - `Access-Control-Max-Age: 86400` (24h cache)
- **Regular requests:** `Access-Control-Allow-Origin` header auto-injected
- **Defaults:** Methods: `GET, POST, PUT, DELETE, PATCH, OPTIONS`. Headers: `Content-Type, Authorization, X-Requested-With`

### C Runtime

| C Function | Purpose |
|------------|---------|
| `__http_server_cors(srv, origin)` | Configure CORS origin + defaults |

---

## Cookies

Read and write HTTP cookies:

### Desi API

```desi
# Read cookie from request
let session = http.get_cookie(req, "session_id")

# Set cookie on response (name, value, max_age_seconds)
let resp = http.json(200, {"status": "ok"})
http.set_cookie(resp, "session_id", "abc123", 3600)  # 1 hour
http.set_cookie(resp, "prefs", "dark", 0)             # session cookie
return resp
```

### Security Defaults

All cookies automatically include:
- `HttpOnly` — not accessible via JavaScript
- `SameSite=Lax` — CSRF protection
- `Path=/` — available site-wide

### C Runtime

| C Function | Purpose |
|------------|---------|
| `__http_req_cookie(req, name)` | Parse `Cookie:` header, return value |
| `__http_resp_cookie(resp, name, value, max_age)` | Append `Set-Cookie` header |

---

## WebSockets

Full-duplex, persistent connections for real-time features (chat, live updates, games).

### Register a WebSocket Endpoint

```desi
import http

def on_message(conn: int, msg: str) -> none:
    print("Received: " + msg)
    http.ws_send(conn, "Echo: " + msg)

def main() -> int:
    let srv = http.server(8080)
    http.ws(srv, "/ws", on_message)
    http.serve(srv)
    return 0
```

Clients connect via `ws://localhost:8080/ws`.

### Lifecycle Hooks

```desi
def on_open(conn: int) -> none:
    print("Client connected: " + str(conn))

def on_close(conn: int) -> none:
    print("Client disconnected: " + str(conn))

http.ws_on_open(srv, on_open)
http.ws_on_close(srv, on_close)
```

### Send and Broadcast

```desi
# Send to one client
http.ws_send(conn, "hello")

# Broadcast to ALL connected clients
http.ws_broadcast(srv, "announcement")

# Close a connection
http.ws_close(conn)
```

### Rooms

Group clients for targeted messaging:

```desi
def on_message(conn: int, msg: str) -> none:
    # Join a room on command
    if msg == "/join lobby":
        http.ws_join(conn, "lobby")
        http.ws_send(conn, "Joined lobby")
    else:
        # Send to everyone in the room
        http.ws_to_room(srv, "lobby", msg)

# Leave a room
http.ws_leave(conn, "room1")
```

### Complete Chat Server Example

```desi
import http

let srv = http.server(0)

def on_open(conn: int) -> none:
    http.ws_join(conn, "chat")
    http.ws_to_room(srv, "chat", "User joined!")

def on_close(conn: int) -> none:
    http.ws_to_room(srv, "chat", "User left!")

def on_message(conn: int, msg: str) -> none:
    http.ws_to_room(srv, "chat", msg)

def main() -> int:
    srv = http.server(8080)
    http.ws(srv, "/ws", on_message)
    http.ws_on_open(srv, on_open)
    http.ws_on_close(srv, on_close)
    http.serve(srv)
    return 0
```

### WebSocket API Reference

| Function | Description |
|----------|-------------|
| `http.ws(srv, path, handler)` | Register WS endpoint (handler: `conn, msg -> none`) |
| `http.ws_on_open(srv, handler)` | Set connect handler (`conn -> none`) |
| `http.ws_on_close(srv, handler)` | Set disconnect handler (`conn -> none`) |
| `http.ws_send(conn, msg)` | Send text to one client |
| `http.ws_broadcast(srv, msg)` | Send to all connected clients |
| `http.ws_close(conn)` | Close a connection |
| `http.ws_join(conn, room)` | Join a room |
| `http.ws_leave(conn, room)` | Leave a room |
| `http.ws_to_room(srv, room, msg)` | Broadcast to a room |

### WebSocket Limits

- Max 256 concurrent connections
- Max 32 rooms per connection
- Max 16MB per message frame
- Auto ping/pong for connection health

---

## TLS / HTTPS Server

The HTTP server supports TLS via OpenSSL for HTTPS and WSS connections.

### Architecture

```
http.server(9443, "cert.pem", "key.pem")
    ↓ Desi overload
__http_server_new_tls(port, cert, key)      ← creates SSL_CTX
    ↓
__http_server_run(srv)                       ← accept loop
    ↓ accept() → desi_tls_accept(ctx, fd) → SSL*
handle_client(srv, fd, ssl)                  ← per-connection
    ↓ DESI_SEND/DESI_RECV macros auto-dispatch TLS or raw
```

### Files

| File | Purpose |
|------|---------|
| `tls.h` | Forward-declared SSL types, `DESI_SEND`/`DESI_RECV` macros |
| `tls.c` | OpenSSL wrapper: SSL_CTX creation, handshake, send/recv, cleanup |
| `http_server.c` | Integrated: `ssl_ctx` on `HttpServer`, `ssl` per-connection |

### C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `desi_tls_ctx_new(cert, key)` | Create `SSL_CTX*`, load cert/key, set TLS 1.2+ minimum |
| `desi_tls_accept(ctx, fd)` | Wrap accepted fd in `SSL*`, do TLS handshake |
| `desi_tls_send(ssl, buf, len)` | `SSL_write` wrapper |
| `desi_tls_recv(ssl, buf, len)` | `SSL_read` wrapper |
| `desi_tls_close(ssl)` | `SSL_shutdown` + `SSL_free` |
| `desi_tls_ctx_free(ctx)` | `SSL_CTX_free` |
| `__http_server_new_tls(port, cert, key)` | Create HTTPS server with TLS context |

### Auto-Dispatch Macros

`DESI_SEND` and `DESI_RECV` (defined in `tls.h`) auto-select TLS or raw sockets:

```c
#define DESI_SEND(fd, ssl, buf, len) \
    ((ssl) ? desi_tls_send((ssl), (buf), (len)) : send((fd), (buf), (len), 0))

#define DESI_RECV(fd, ssl, buf, len) \
    ((ssl) ? desi_tls_recv((ssl), (buf), (len)) : recv((fd), (buf), (len), 0))
```

Used in all send/recv call sites in `http_server.c` (7 locations). When `ssl == NULL` (plain HTTP), falls through to raw sockets with zero overhead.

### Per-Connection SSL Lifecycle

1. `accept()` returns `client_fd`
2. If `srv->ssl_ctx != NULL`: `desi_tls_accept(ctx, fd)` → `SSL*`
3. `SSL*` threaded through `handle_client(srv, fd, ssl)` → `send_response_ka(fd, ssl, ...)` → `try_serve_static(srv, fd, ssl, ...)`
4. On connection close: `desi_tls_close(ssl)` then `CLOSE_SOCKET(fd)`

### Compile-Time Detection

`tls.c` uses `__has_include(<openssl/ssl.h>)` to detect OpenSSL at compile time. If unavailable, stub functions return `NULL`/`-1` — the server starts in HTTP-only mode.

### Error Handling

- **SSL_CTX creation errors** (bad cert/key) — logged and `__http_server_new_tls` returns `NULL`
- **Handshake failures** — `SSL_ERROR_SSL` logged with full OpenSSL error string; `SSL_ERROR_SYSCALL` (plain-HTTP probes) silenced
- **Send/recv errors** — `DESI_SEND`/`DESI_RECV` return `-1`, connection closed normally

### Build System

Already handled — no changes needed:

- **Makefile**: `$(OPENSSL_CFLAGS)` passed to `$(CC)`, `tls.c` auto-discovered by wildcard
- **run_build_cmd.go**: `detectOpenSSLPrefix()` finds Homebrew/system OpenSSL, links `-lssl -lcrypto`
- **watch_cmd.go**: Same OpenSSL detection for hot-reload builds

### Desi API

```desi
# Plain HTTP (existing)
let srv = http.server(8080)

# HTTPS with TLS (new overload)
let srv = http.server(9443, "cert.pem", "key.pem")
```

Internally maps to `__http_server_new_tls(port, cert, key)` via Desi function overloading.

