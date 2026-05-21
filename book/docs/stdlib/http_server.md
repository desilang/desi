# HTTP Server

Desi includes a built-in HTTP server with routing, middleware, CORS, rate limiting, static file serving, SSE, file uploads, and graceful shutdown. No external dependencies — it's part of the `http` module.

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
let srv = http.server(9443, "cert.pem", "key.pem")
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
| `http.req_param(req, key)` | `str` | Get query parameter by key |
| `http.req_path_param(req, name)` | `str` | Get URL path parameter |
| `http.req_json(req)` | `Any` | Parse request body as JSON |

## Response Helpers

Return one of these from your handler:

| Function | Content-Type | Description |
|---|---|---|
| `http.text(status, body)` | `text/plain` | Plain text response |
| `http.html(status, body)` | `text/html` | HTML response |
| `http.json_text(status, body)` | `application/json` | JSON response (string body) |
| `http.json(status, data)` | `application/json` | JSON response (auto-serialized dict) |
| `http.response(status, body, ct)` | Custom | Custom content type |
| `http.render(...)` | `text/html` | Unified response builder (see below) |
| `http.html_file(path)` | `text/html` | Serve HTML from file |
| `http.file_response(status, path, ct)` | Custom | Serve any file |

## Template Rendering

The `render()` function is a unified response builder with named parameters:

```desi
# Inline HTML body
return http.render(body="<h1>Hello!</h1>")

# Serve an HTML file from disk
return http.render(file="templates/index.html")

# Custom status code
return http.render(body="<h1>Not Found</h1>", status=404)

# Custom content type
return http.render(body=xml_data, status=200, content_type="application/xml")
```

When `file` is provided, the file is read from disk and served. Otherwise the inline `body` is used.

**Full signature:**

```desi
http.render(body="", file="", status=200, content_type="text/html; charset=utf-8")
```

## Static File Serving

Serve files from a local directory under a URL prefix:

```desi
let srv = http.server(9090)
http.serve_static(srv, "/static", "./public")
# /static/style.css → ./public/style.css
# /static/app.js   → ./public/app.js
```

Files are served with correct MIME types based on file extension.

## File Responses

Serve individual files from your handler:

```desi
# Serve an HTML file
return http.html_file("templates/page.html")

# Serve any file with explicit content type
return http.file_response(200, "data.json", "application/json")
```

## Server Configuration

```desi
let srv = http.server(9090)

# Max request body size (default: 1MB)
http.max_body(srv, 1048576)        # 1MB limit

# Request timeout in seconds (default: 15s)
http.timeout(srv, 30)

# Enable CORS for a specific origin
http.cors(srv, "*")                # allow all origins
http.cors(srv, "https://myapp.com")

# Rate limiting (per IP)
http.rate_limit(srv, 100, 60)      # 100 requests per 60 seconds
```

## Custom Response Headers

Add custom headers to any response:

```desi
def handler(req: Request) -> Any:
    let resp = http.text(200, "ok")
    http.header(resp, "X-Custom", "my-value")
    http.header(resp, "Cache-Control", "no-cache")
    return resp
```

## Cookies

```desi
def handler(req: Request) -> Any:
    # Read a cookie
    let session = http.get_cookie(req, "session_id")

    # Set a cookie (expires in 1 hour)
    let resp = http.text(200, "logged in")
    http.set_cookie(resp, "session_id", "abc123", 3600)
    return resp
```

## Multipart File Upload

Handle file uploads via multipart form data:

```desi
def handle_upload(req: Request) -> Any:
    let filename = http.req_form_filename(req, "file")
    let content = http.req_form_file(req, "file")
    let size = http.req_form_file_size(req, "file")
    let description = http.req_form_field(req, "description")

    print("Uploaded: " + filename + " (" + str(size) + " bytes)")
    return http.json_text(200, "{\"uploaded\": true}")
```

Upload form field helpers:

| Function | Returns | Description |
|---|---|---|
| `http.req_form_field(req, name)` | `str` | Get text field value |
| `http.req_form_file(req, name)` | `str` | Get uploaded file content |
| `http.req_form_filename(req, name)` | `str` | Get original filename |
| `http.req_form_file_size(req, name)` | `int` | Get file size in bytes |

## Server-Sent Events (SSE)

Stream real-time updates to clients:

```desi
def handle_events(req: Request) -> Any:
    let stream = http.sse_start(req)
    http.sse_send(req, "hello")
    http.sse_send_event(req, "update data", "custom-event")
    http.sse_close(req)
    return http.sse_response()
```

Client-side JavaScript:

```javascript
const events = new EventSource("/events");
events.onmessage = (e) => console.log(e.data);
events.addEventListener("custom-event", (e) => console.log(e.data));
```

| Function | Description |
|---|---|
| `http.sse_start(req)` | Begin SSE stream |
| `http.sse_send(req, data)` | Send data event |
| `http.sse_send_event(req, data, event)` | Send named event |
| `http.sse_close(req)` | Close SSE stream |
| `http.sse_response()` | Return SSE response from handler |

## Middleware

Register middleware functions that run before every request:

```desi
def log_request(req: Request) -> Any:
    print(http.req_method(req) + " " + http.req_path(req))
    return None  # continue to handler

http.use(srv, log_request)
```

## Graceful Shutdown

```desi
http.shutdown()    # Stops the server cleanly
```

## See Also

- [HTTP Client](http.md) — GET/POST/PUT/PATCH/DELETE, headers, TLS
- [WebSocket](websocket.md) — Real-time bidirectional communication
