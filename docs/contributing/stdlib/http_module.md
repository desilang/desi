# HTTP Module — Contributor Guide

The `http` module provides HTTP client, server, WebSocket, SSE, and TLS support. It is a **pure Desi module** wrapping C runtime functions — no compiler interceptor logic required.

## Architecture

```
compiler/lib/http/__mod.desi    ← Desi module (~85 pub functions)
         ↓ @extern("C")
compiler/runtime/http_client.c  ← HTTP client (TLS, redirects, pooling)
compiler/runtime/http_server.c  ← HTTP server (~4000 lines: routing, WS, SSE, TLS, CORS)
```

The lowerer (`lower_call.go`) has **zero** HTTP-specific interceptor code. All dispatch goes through the standard module call path.

## Design Decisions

### Decoupled Architecture (May 2026)

Previously, the lowerer had a ~300-line interceptor `switch` block that hard-coded dispatch for `serve`, `route`, `ws`, `render`, and ~25 other HTTP functions. This was removed because:

1. **Redundancy** — The Desi `@extern("C")` wrappers already mapped 1:1 to C functions
2. **Maintainability** — Every new HTTP function required lowerer changes
3. **Consistency** — All other stdlib modules use the standard call path

The key enabler was `lowerCallArgExpr()` in the lowerer, which automatically emits `hir.FuncRef` for identifiers used in function-type parameter positions. This makes callback-accepting functions like `serve(srv, handler)` work without special-casing.

### Callback Handling

Functions like `serve()`, `ws()`, `ws_on_open()` accept function references:

```desi
http.serve(srv, handle_request)   # handle_request → FuncRef
http.ws(srv, "/ws", on_message)   # on_message → FuncRef
```

The lowerer resolves these via `paramNames` map populated from the function's `@extern("C")` declaration. When a positional arg name matches a function-type parameter, it emits `hir.FuncRef` instead of a regular value.

### Default Parameters

`render()` uses named args with defaults:

```desi
pub def render(body: str = "", file: str = "", status: int = 200,
               content_type: str = "text/html; charset=utf-8") -> Any:
    if file != "":
        return __http_resp_from_file(file, status, content_type)
    return __http_resp_new(body, status, content_type)
```

This replaced a complex lowerer interceptor that had to dispatch to two different C functions based on the `file=` kwarg.

### Overloaded Functions

Several functions have multiple signatures:

| Function | Overloads |
|----------|-----------|
| `server()` | `(port)` and `(port, cert, key)` |
| `serve()` | `(srv)` and `(srv, handler)` |
| `ws_broadcast()` | `(srv, msg)` and `(msg)` |
| `ws_to_room()` | `(srv, room, msg)` and `(room, msg)` |

The type checker resolves the correct overload based on argument count.

## Module Sections

| Section | Functions | Purpose |
|---------|-----------|---------|
| C externs | ~40 `@extern("C")` | Bridge to runtime |
| Client API | `get`, `post`, `put`, `delete`, etc. | HTTP client |
| Server API | `server`, `serve`, `route`, response builders | HTTP server |
| Request helpers | `req_method`, `req_path`, `req_body`, etc. | Request inspection |
| SSE | `sse_start`, `sse_send`, `sse_close` | Server-Sent Events |
| WebSocket | `ws`, `ws_send`, `ws_broadcast`, rooms | Real-time communication |
| Config | `max_body`, `timeout`, `cors`, `rate_limit` | Server hardening |
| Cookie/Proxy/TLS | `set_cookie`, `set_proxy`, `set_ca_bundle` | Advanced features |

## Adding New HTTP Functions

1. Implement in C runtime (`http_server.c` or `http_client.c`)
2. Add `@extern("C")` declaration in `__mod.desi`
3. Add `pub def` wrapper in `__mod.desi`
4. Add test example (mark with `# EXPECTED: SKIP` if it starts a server)
5. Update docs: `book/docs/stdlib/http.md` + `book/docs/stdlib/http_server.md`

**No changes to the lowerer or type checker are needed.**

## Test Examples

All HTTP server examples are marked `# EXPECTED: SKIP` because they start blocking servers. Run manually with `desic run`.

| Example | Tests |
|---------|-------|
| `409_http_server.desi` | Basic echo server |
| `410_http_routing.desi` | Route dispatch with request helpers |
| `412_websocket.desi` | WebSocket echo |
| `413_websocket_broadcast.desi` | WS broadcast + rooms |
| `414_websocket_config.desi` | WS configuration (max size, ping, compression) |
| `415_websocket_edge_cases.desi` | WS edge cases |
| `416_https_server.desi` | HTTPS with TLS |
| `417_wss_chat.desi` | WSS chat app |
| `418_http_features.desi` | Cookie, redirect, timeout (non-blocking) |
| `419_http_server_upload.desi` | Multipart file upload |
| `420_sse_stream.desi` | Server-Sent Events |
| `421_http_render.desi` | render() function |
| `422_http_styled_demo.desi` | Static files + CSS/JS |

## See Also

- [WebSocket Implementation](../runtime/websocket-implementation.md) — Protocol internals
- [Stdlib Architecture](stdlib_architecture.md) — General module pattern
- [HTTP Client Book](../../../book/docs/stdlib/http.md) — User-facing client docs
- [HTTP Server Book](../../../book/docs/stdlib/http_server.md) — User-facing server docs
