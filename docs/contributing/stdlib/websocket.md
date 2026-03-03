# WebSocket Implementation

Contributor documentation for the WebSocket module internals.

## Architecture

```
websocket.h           ← shared types (WsConnection, WsState)
websocket.c           ← RFC 6455 implementation (handshake, frames, rooms)
http_server.c         ← upgrade detection → ws_session_loop()
__mod.desi            ← Desi API wrappers
lower_call.go         ← intercepts ws, ws_on_open, ws_on_close
sig_overrides.go      ← return type overrides for __ws_* functions
```

## Protocol (RFC 6455)

### Handshake

1. Client sends `GET /ws` with `Upgrade: websocket` + `Sec-WebSocket-Key`
2. Server computes `SHA1(key + "258EAFA5-E914-47DA-95CA-5AB5DC76B00E")`
3. Server responds `101 Switching Protocols` with Base64-encoded accept key
4. Connection upgrades to WebSocket framed protocol

SHA-1 + Base64 are self-contained in `websocket.c` — no OpenSSL dependency. These are protocol requirements, not security (TLS handles security).

### Frame Format

```
 0               1               2               3
 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+-+-+-+-+-------+-+-------------+-------------------------------+
|F|R|R|R| opcode|M| Payload len |    Extended payload length    |
|I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
|N|V|V|V|       |S|             |                               |
+-+-+-+-+-------+-+-------------+-------------------------------+
```

- Opcodes: `0x1` text, `0x2` binary, `0x8` close, `0x9` ping, `0xA` pong
- Client→server: MUST be masked (4-byte XOR key)
- Server→client: unmasked
- Payload len: 7-bit (< 126), 16-bit (== 126), 64-bit (== 127)

## C Runtime Functions

| C Function | Desi API | Purpose |
|------------|----------|---------|
| `ws_do_handshake(fd, key)` | internal | Send 101 Switching Protocols |
| `ws_session_loop(fd)` | internal | Read frame loop with handler dispatch |
| `ws_read_frame(fd, frame)` | internal | Decode one incoming frame |
| `ws_send_text(fd, msg, len)` | internal | Send text frame |
| `ws_send_close(fd, code)` | internal | Send close frame |
| `ws_send_ping(fd)` | internal | Send ping |
| `ws_send_pong(fd, data, len)` | internal | Send pong (auto on ping) |
| `__ws_send(fd, msg)` | `http.ws_send()` | Send text to connection |
| `__ws_broadcast(msg)` | `http.ws_broadcast()` | Send to all connections |
| `__ws_close(fd)` | `http.ws_close()` | Close + unregister |
| `__ws_join(fd, room)` | `http.ws_join()` | Add conn to room |
| `__ws_leave(fd, room)` | `http.ws_leave()` | Remove conn from room |
| `__ws_to_room(room, msg)` | `http.ws_to_room()` | Broadcast to room |
| `__ws_set_path(path)` | via lowerer | Set WS endpoint path |
| `__ws_set_on_message(fn)` | via lowerer | Register message handler |
| `__ws_set_on_open(fn)` | via lowerer | Register connect handler |
| `__ws_set_on_close(fn)` | via lowerer | Register disconnect handler |

## Data Structures

### WsConnection
```c
typedef struct {
    int  fd;                                       // socket fd (0 = empty)
    char rooms[WS_MAX_ROOMS][WS_ROOM_NAME_LEN];   // up to 32 rooms
    int  room_count;
} WsConnection;
```

### WsState (global)
```c
typedef struct {
    WsConnection conns[WS_MAX_CONNECTIONS];  // 256 slots
    int conn_count;
    void* on_message;   // fn(int conn_fd, const char* msg)
    void* on_open;      // fn(int conn_fd)
    void* on_close;     // fn(int conn_fd)
    char  ws_path[1024];
} WsState;
```

## HTTP Server Integration

In `handle_client()` (http_server.c), after request parsing:

```c
// Detect Upgrade: websocket header
const char* upgrade = find_header_safe(req->headers, "Upgrade", ...);
if (strcasecmp(upgrade, "websocket") == 0 && path matches ws_path) {
    ws_do_handshake(client_fd, ws_key);
    ws_session_loop(client_fd);  // blocks until disconnect
    return;  // connection taken over
}
```

The WS session loop runs on a supervisor worker thread — no special threading needed.

## Lowerer

`http.ws(srv, path, handler)` emits two calls:
1. `__ws_set_path(path)` — register endpoint path
2. `__ws_set_on_message(@handler)` — register Desi function as callback

`http.ws_on_open/close(srv, handler)` emits:
- `__ws_set_on_open/close(@handler)` with function reference

## Limits

| Limit | Value | Defined in |
|-------|-------|------------|
| Max connections | 256 | `WS_MAX_CONNECTIONS` in websocket.h |
| Max rooms/conn | 32 | `WS_MAX_ROOMS` in websocket.h |
| Room name length | 64 chars | `WS_ROOM_NAME_LEN` in websocket.h |
| Max frame payload | 16MB | hardcoded in `ws_read_frame()` |

## Files

| File | Purpose |
|------|---------|
| `compiler/runtime/websocket.c` | Core WS implementation |
| `compiler/runtime/websocket.h` | Shared types |
| `compiler/runtime/http_server.c` | Upgrade detection |
| `compiler/lib/http/__mod.desi` | Desi API wrappers |
| `compiler/internal/lower/lower_call.go` | Lowerer interception |
| `compiler/internal/backend/llvm/sig_overrides.go` | LLVM return types |
| `docs/stdlib/http.md` | Learner documentation |
