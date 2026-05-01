# HTTP Server

Desi includes a built-in HTTP server with routing, middleware, CORS, rate limiting, static file serving, and graceful shutdown. No external dependencies — it's part of the `http` module.

## Import

```desi
import http
from http import Request
```

## Quick Start

```desi
import http
from http import Request

def handle_request(req: Request) -> Any:
    let path = http.req_path(req)

    if path == "/":
        return http.html(200, "<h1>Hello from Desi!</h1>")

    if path == "/api/health":
        return http.json_text(200, "{\"status\":\"ok\"}")

    return http.text(404, "not found")

def main() -> int:
    let srv = http.server(9090)
    http.serve(srv, handle_request)
    return 0
```

Run with `desic run myserver.desi`, then `curl http://localhost:9090/`.

## Creating a Server

```desi
# HTTP
let srv = http.server(9090)

# HTTPS (TLS)
let srv = http.server_tls(9090, "cert.pem", "key.pem")
```

## Request Helpers

Inside your handler, access request details:

| Function | Returns | Description |
|---|---|---|
| `http.req_path(req)` | `str` | URL path (e.g. `/api/users`) |
| `http.req_method(req)` | `str` | HTTP method (`GET`, `POST`, etc.) |
| `http.req_query(req)` | `str` | Query string (e.g. `name=alice&age=30`) |
| `http.req_header(req, name)` | `str` | Get a request header value |
| `http.req_body(req)` | `str` | Request body (POST/PUT) |

## Response Helpers

Return one of these from your handler:

| Function | Content-Type | Description |
|---|---|---|
| `http.text(status, body)` | `text/plain` | Plain text response |
| `http.html(status, body)` | `text/html` | HTML response |
| `http.json_text(status, body)` | `application/json` | JSON response |

## Server Configuration

```desi
let srv = http.server(9090)

# Max request body size (default: unlimited)
http.max_body(srv, 1048576)        # 1MB limit

# Request timeout in seconds
http.timeout(srv, 30)

# Enable CORS for a specific origin
http.cors(srv, "*")                # allow all origins
http.cors(srv, "https://myapp.com")

# Rate limiting
http.rate_limit(srv, 100, 60)      # 100 requests per 60 seconds

# Static file serving
http.static(srv, "/assets", "./public")  # /assets/* → ./public/*
```

## Graceful Shutdown

```desi
http.shutdown()    # Stops the server cleanly
```

## See Also

- [HTTP Client](http.md) — GET/POST/PUT/PATCH/DELETE, headers, TLS
- [WebSocket](websocket.md) — Real-time bidirectional communication
