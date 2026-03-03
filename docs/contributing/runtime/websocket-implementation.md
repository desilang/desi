# WebSocket Implementation

This document explains the WebSocket (RFC 6455) implementation in Desi's C runtime.

## Architecture Overview

WebSocket runs on top of the existing HTTP server. When a browser sends an `Upgrade: websocket` request, the HTTP worker thread performs the RFC 6455 handshake and transitions to a WebSocket session loop on the same socket.

```
Browser                          Desi HTTP Server
───────                          ─────────────────
GET /ws HTTP/1.1                 handle_client() [worker thread]
Upgrade: websocket        ────►  detect Upgrade header
Sec-WebSocket-Key: ...           extract Sec-WebSocket-Key

                          ◄────  ws_do_handshake()
HTTP/1.1 101 Switching           send 101 + Sec-WebSocket-Accept
Protocols

ws.send("hello")          ────►  ws_session_loop()
                                 ws_read_frame() → on_message()
                          ◄────  ws_send_text()
echo: hello
```

## Files

| File | Role |
|------|------|
| `runtime/websocket.c` | SHA-1, Base64, handshake, frame read/write, session loop |
| `runtime/websocket.h` | Public API + platform macros (`WS_SEND`, `WS_RECV`) |
| `runtime/http_server.c` | Upgrade detection (lines ~1062–1117) |
| `lower/lower_call.go` | Lowering `http.ws()`, `http.ws_on_open()`, etc. |
| `backend/llvm/sig_overrides.go` | Runtime function signatures |

## Handshake (RFC 6455 §4.2.2)

```c
static const char* WS_MAGIC = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";

int ws_do_handshake(int fd, const char* client_key) {
    // 1. concat = client_key + WS_MAGIC
    // 2. hash   = SHA-1(concat)
    // 3. accept = Base64(hash)
    // 4. send "HTTP/1.1 101 Switching Protocols\r\n"
    //         "Upgrade: websocket\r\n"
    //         "Connection: Upgrade\r\n"
    //         "Sec-WebSocket-Accept: <accept>\r\n\r\n"
}
```

> [!CAUTION]
> The magic GUID must be exactly `258EAFA5-E914-47DA-95CA-C5AB0DC85B11`. An earlier bug used `...5AB5DC76B00E` which caused all browsers to reject the handshake (error 1006). Programmatic clients like `curl` don't verify the accept key, which masked the bug.

## Frame Protocol

### Reading (client → server)

Client frames are **masked** (RFC 6455 §5.3). `ws_read_frame()`:

1. Read 2-byte header → extract opcode, FIN bit, payload length
2. If length == 126: read 2 more bytes (16-bit extended length)
3. If length == 127: read 8 more bytes (64-bit extended length)
4. Read 4-byte mask key
5. Read payload, XOR each byte with `mask[i % 4]`

### Writing (server → client)

Server frames are **NOT masked**. `ws_send_frame()`:

1. Build header: `0x80 | opcode` (FIN + opcode), then length encoding
2. Send header, then raw payload

### Opcodes

| Opcode | Meaning |
|--------|---------|
| `0x1` | Text frame |
| `0x2` | Binary frame |
| `0x8` | Close |
| `0x9` | Ping |
| `0xA` | Pong |

## Session Loop

`ws_session_loop()` runs on the HTTP worker thread. It blocks on `recv()` reading frames:

```c
void ws_session_loop(int client_fd, const uint8_t* prebuf, size_t prebuf_len) {
    ws_set_prebuf(prebuf, prebuf_len);  // leftover bytes from HTTP read
    WsConnection* conn = ws_register_conn(client_fd);
    if (on_open) on_open(client_fd);

    while (1) {
        if (ws_read_frame(fd, &frame) < 0) break;
        switch (frame.opcode) {
            case TEXT/BINARY: on_message(fd, payload); break;
            case PING:        ws_send_pong(fd, ...);   break;
            case CLOSE:       ws_send_close(fd, 1000); goto done;
        }
    }
    if (on_close) on_close(client_fd);
    ws_unregister_conn(client_fd);
    close(client_fd);
}
```

### Prebuffer

The HTTP server reads up to 64KB from the socket. If the browser pipelines a WebSocket frame in the same TCP segment as the upgrade request, those bytes are saved and drained before calling `recv()` again.

## Connection Management

```c
#define WS_MAX_CONNS 256

typedef struct {
    int    fd;
    char   rooms[WS_MAX_ROOMS][64];
    int    room_count;
} WsConnection;

WsState __ws_state;  // Global state: connections[], handlers, path
```

## Platform Notes

### macOS `SO_RCVTIMEO`

> [!WARNING]
> On macOS, `setsockopt(SO_RCVTIMEO, {0, 0})` means **zero timeout** (non-blocking), NOT "no timeout" like on Linux. The upgrade handler sets `{86400, 0}` (24h) to block effectively forever.

### `MSG_NOSIGNAL`

Not available on macOS. `websocket.h` uses platform-conditional macros:

```c
#ifdef __APPLE__
  #define WS_SEND(fd, buf, len) send(fd, buf, len, 0)
  // SIGPIPE handled by signal(SIGPIPE, SIG_IGN) in http_server.c
#else
  #define WS_SEND(fd, buf, len) send(fd, buf, len, MSG_NOSIGNAL)
#endif
```

## Lowering (Compiler → Runtime)

The compiler lowers Desi API calls to C runtime functions:

| Desi Code | Lowered To |
|-----------|------------|
| `http.ws(srv, path, handler)` | `__ws_set_path(path)` + `__ws_set_on_message(@handler)` |
| `http.ws_on_open(srv, handler)` | `__ws_set_on_open(@handler)` |
| `http.ws_on_close(srv, handler)` | `__ws_set_on_close(@handler)` |
| `http.ws_send(conn, msg)` | `__ws_send(conn, msg)` |
| `http.ws_broadcast(srv, msg)` | `__ws_broadcast(srv, msg)` |
| `http.ws_close(conn)` | `__ws_close(conn)` |
| `http.ws_join(conn, room)` | `__ws_join(conn, room)` |
| `http.ws_leave(conn, room)` | `__ws_leave(conn, room)` |
| `http.ws_to_room(srv, room, msg)` | `__ws_to_room(srv, room, msg)` |

## Related Files

- `compiler/runtime/websocket.c` — Core implementation
- `compiler/runtime/websocket.h` — Headers and platform macros
- `compiler/runtime/http_server.c` — Upgrade detection
- `compiler/internal/lower/lower_call.go` — Lowering (lines ~339–369)
- `compiler/internal/backend/llvm/sig_overrides.go` — Runtime signatures
- `examples/412_websocket.desi` — Example echo server
