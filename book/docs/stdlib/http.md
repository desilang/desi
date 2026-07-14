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

- **HTTP and HTTPS** — TLS via OpenSSL (macOS/Linux) or Schannel (Windows)
- **IPv4 and IPv6** — dual-stack support
- **Redirects** — automatic following (301, 302, 303, 307, 308)
- **Chunked encoding** — handles `Transfer-Encoding: chunked`
- **Connection pooling** — reuses TCP connections for the same host
- **No dependencies** — ships with the compiler, no pip/npm needed

## HTTPS Requirements

HTTPS requires OpenSSL to be installed:

- **macOS**: `brew install openssl` (auto-detected from Homebrew)
- **Linux**: Usually pre-installed (`libssl-dev` package)
- **Windows**: Schannel (built-in, no external dependencies)

If OpenSSL is not found on macOS/Linux, HTTP requests still work; only HTTPS will fail with a connection error.

---

## HTTP Server

Build web servers and APIs with Desi's built-in HTTP server.

!!! note "Platform support"
    The HTTP **server** (and WebSocket server) currently requires macOS or
    Linux. On Windows, server calls print a "not yet supported" notice; the
    HTTP **client** — including HTTPS via the system's Schannel TLS — works
    fully on all platforms.

### Creating a Server

```desi
import http

def handler(req: Any) -> Any:
    let path = http.req_path(req)
    let method = http.req_method(req)
    return http.text(200, "Hello from Desi!")

def main() -> int:
    let srv = http.server(8080)
    print("Listening on port 8080...")
    http.serve(srv, handler)
    return 0
```

### Request Accessors

Inside your handler function, use these to inspect the incoming request:

| Function | Returns | Description |
|----------|---------|-------------|
| `http.req_method(req)` | `str` | HTTP method (`"GET"`, `"POST"`, etc.) |
| `http.req_path(req)` | `str` | Request path (`"/users"`) |
| `http.req_body(req)` | `str` | Request body text |
| `http.req_header(req, name)` | `str` | Value of a specific header |
| `http.req_query(req)` | `str` | Query string (`"page=1&limit=10"`) |

### Response Builders

Return responses from your handler:

```desi
# Plain text
return http.text(200, "Hello!")

# JSON from a string
return http.json_text(200, '{"status": "ok"}')

# JSON from a dict (auto-serialized)
return http.json(200, {"name": "alice", "age": 30})

# HTML
return http.html(200, "<h1>Welcome!</h1>")

# Custom content type
return http.response(200, data, "application/xml")
```

| Function | Content-Type |
|----------|-------------|
| `http.text(status, body)` | `text/plain` |
| `http.json_text(status, body)` | `application/json` |
| `http.json(status, data)` | `application/json` (auto-serialized) |
| `http.html(status, body)` | `text/html` |
| `http.response(status, body, ct)` | Custom |

---

## Response Helpers

Check response status categories:

```desi
let resp = http.get("https://api.example.com/data")

if http.is_ok(resp):
    print("Success!")
elif http.is_client_error(resp):
    print("Client error:", resp.status)
elif http.is_server_error(resp):
    print("Server error:", resp.status)
```

| Function | True when |
|----------|-----------|
| `http.is_ok(resp)` | Status 200-299 |
| `http.is_redirect(resp)` | Status 301/302/303/307/308 |
| `http.is_client_error(resp)` | Status 400-499 |
| `http.is_server_error(resp)` | Status 500-599 |

---

## Authentication

### Basic Auth

```desi
let headers = http.basic_auth("user", "password")
let resp = http.request("GET", "https://api.example.com/secret", "", headers)
```

### Bearer Token

```desi
let headers = http.bearer_auth("my-api-token")
let resp = http.request("GET", "https://api.example.com/data", "", headers)
```

---

## Form Data & Query Strings

### POST Form

Send form-encoded data (like HTML form submissions):

```desi
let resp = http.post_form("https://example.com/login", {
    "username": "alice",
    "password": "secret"
})
```

### Build Query String

Convert a dict to a URL query string:

```desi
let qs = http.build_query({"q": "hello world", "page": "1"})
# "q=hello%20world&page=1"
```

---

## Request Timeouts

Control how long requests wait before failing:

```desi
# Custom request with 10-second timeout
let resp = http.request_timeout("GET", url, "", "", 10)

# JSON request with 30-second timeout
let resp = http.request_json("POST", url, {"key": "val"}, 30)
```

---

## See Also

- [JSON Module](json.md) — For parsing JSON responses
- [Strings Module](strings.md) — For string manipulation

---

## HTTPS Server (TLS)

Desi supports HTTPS servers with TLS encryption out of the box. Provide a certificate and key to enable TLS:

### Creating an HTTPS Server

```desi
import http

def handler(req: Any) -> Any:
    return http.json(200, {"secure": true, "message": "Hello over TLS!"})

def main() -> int:
    # Pass cert and key to enable TLS
    let srv = http.server(9443, "cert.pem", "key.pem")
    http.serve(srv, handler)
    return 0
```

Without cert/key, `http.server(port)` creates a plain HTTP server (existing behavior).

### Generating a Self-Signed Certificate

For development and testing:

```bash
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem \
    -days 365 -nodes -subj "/CN=localhost"
```

### Testing with curl

```bash
# -k flag accepts self-signed certificates
curl -k https://localhost:9443/
```

### HTTPS + WebSockets (WSS)

WebSocket upgrades over TLS connections automatically use `wss://`:

```desi
import http

def on_message(conn: int, msg: str):
    http.ws_send(conn, "secure echo: " + msg)

def main() -> int:
    let srv = http.server(9443, "cert.pem", "key.pem")
    http.ws(srv, "/ws", on_message)
    http.serve(srv)
    return 0
```

Connect from browser:

```javascript
const ws = new WebSocket("wss://localhost:9443/ws");
ws.onmessage = (e) => console.log(e.data);
```

### TLS Requirements

- **macOS**: `brew install openssl`
- **Linux**: `libssl-dev` (usually pre-installed)
- TLS 1.2+ enforced (no SSLv3 or TLS 1.0/1.1)
- **Windows**: Uses Schannel (built-in, no external dependencies)

---

## WebSocket Server

Build real-time applications with Desi's built-in WebSocket support. WebSockets run on the same HTTP server — no extra dependencies.

### Quick Start

```desi
import http

def on_message(conn: int, msg: str):
    print("Received: " + msg)
    http.ws_send(conn, "echo: " + msg)

def handler(req: Any) -> Any:
    return http.text(200, "Hello!")

def main() -> int:
    let srv = http.server(8080)
    http.ws(srv, "/ws", on_message)
    http.serve(srv, handler)
    return 0
```

Connect from browser JavaScript:

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");
ws.onopen = () => ws.send("hello");
ws.onmessage = (e) => console.log(e.data);  // "echo: hello"
```

### Setting Up WebSocket Routes

Register a WebSocket handler on a path:

```desi
http.ws(srv, "/ws", on_message)
```

The `on_message` callback receives every WebSocket message:

```desi
def on_message(conn: int, msg: str):
    # conn = connection file descriptor (identifies the client)
    # msg  = the message text
    http.ws_send(conn, "got: " + msg)
```

### Lifecycle Events

Track connections and disconnections:

```desi
def on_open(conn: int):
    print("Client connected: " + str(conn))

def on_close(conn: int):
    print("Client disconnected: " + str(conn))

http.ws_on_open(srv, on_open)
http.ws_on_close(srv, on_close)
```

### Sending Messages

```desi
# Send to a specific client
http.ws_send(conn, "hello")

# Broadcast to ALL connected clients
http.ws_broadcast(srv, "announcement!")
```

### Rooms

Group clients into rooms for targeted messaging:

```desi
# In on_open or on_message:
http.ws_join(conn, "lobby")      # join a room
http.ws_leave(conn, "lobby")     # leave a room

# Send to all clients in a room
http.ws_to_room(srv, "lobby", "room message!")
```

### Configuration

```desi
# Set maximum message size (default: 16MB)
http.ws_max_message_size(srv, 1024 * 1024)  # 1MB limit

# Enable automatic ping keepalive (default: disabled)
http.ws_ping_interval(srv, 30)  # ping every 30 seconds

# Enable permessage-deflate compression (RFC 7692)
http.ws_compression(srv, true)
```

Compression reduces message sizes by 60-80% for text-heavy WebSocket traffic. It's negotiated automatically during the handshake — clients that don't support it will continue to work uncompressed.

### WebSocket API Reference

| Function | Description |
|----------|-------------|
| `http.ws(srv, path, on_message)` | Register WS handler on path |
| `http.ws_on_open(srv, callback)` | Set connection open handler |
| `http.ws_on_close(srv, callback)` | Set connection close handler |
| `http.ws_send(conn, msg)` | Send text message to one client |
| `http.ws_broadcast(srv, msg)` | Send to all connected clients |
| `http.ws_join(conn, room)` | Add client to a room |
| `http.ws_leave(conn, room)` | Remove client from a room |
| `http.ws_to_room(srv, room, msg)` | Send to all clients in a room |
| `http.ws_close(conn)` | Close a specific connection |
| `http.ws_max_message_size(srv, bytes)` | Set max incoming message size |
| `http.ws_ping_interval(srv, secs)` | Set ping keepalive interval (0 = off) |
| `http.ws_compression(srv, enabled)` | Enable/disable permessage-deflate compression |

### Callback Signatures

| Callback | Signature |
|----------|-----------|
| `on_message` | `(conn: int, msg: str)` |
| `on_open` | `(conn: int)` |
| `on_close` | `(conn: int)` |

### Features

- **Text & binary frames** — both frame types handled
- **Automatic ping/pong** — configurable keepalive interval
- **Configurable message size** — prevent memory exhaustion
- **Rooms** — group clients for targeted messaging
- **Multiple WS paths** — register handlers on different paths (up to 8)
- **Compression** — permessage-deflate (RFC 7692) reduces bandwidth
- **RFC 6455 compliant** — proper handshake, masking, close frames

### Example: Chat Server

```desi
import http

def on_message(conn: int, msg: str):
    # Broadcast every message to all clients
    http.ws_broadcast(srv, msg)

def on_open(conn: int):
    http.ws_join(conn, "chat")
    http.ws_to_room(srv, "chat", "Someone joined!")

def handler(req: Any) -> Any:
    return http.html(200, "<h1>Chat</h1>")

def main() -> int:
    let srv = http.server(8080)
    http.ws(srv, "/ws", on_message)
    http.ws_on_open(srv, on_open)
    http.ws_ping_interval(srv, 30)  # keep connections alive
    http.ws_compression(srv, true)   # compress messages
    http.serve(srv, handler)
    return 0
```

---

## Server Security & Performance

Desi's HTTP server includes built-in production hardening:

### Request Body Limits

```desi
http.max_body(srv, 1024 * 512)   # 512KB max body size
```

Oversized requests receive `413 Payload Too Large` before the body is read, preventing memory exhaustion.

### Connection Timeout

```desi
http.timeout(srv, 30)   # 30-second timeout
```

Closes idle connections after the specified seconds. Protects against slow client attacks (Slowloris). Default: 15 seconds.

### Rate Limiting

```desi
http.rate_limit(srv, 100, 60)   # 100 requests per 60 seconds per IP
```

Returns `429 Too Many Requests` with a `Retry-After` header when exceeded.

### Server Security API Reference

| Function | Description |
|----------|-------------|
| `http.max_body(srv, bytes)` | Set max request body size (default: 1MB) |
| `http.timeout(srv, secs)` | Set connection timeout (default: 15s) |
| `http.rate_limit(srv, max, window)` | Configure per-IP rate limiting |
| `http.cors(srv, origin)` | Configure CORS (default: `*`) |
