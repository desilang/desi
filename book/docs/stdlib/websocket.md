# WebSocket

Desi's HTTP server includes built-in WebSocket support for real-time bidirectional communication. No external dependencies.

## Import

```desi
import http
```

## Quick Start — Echo Server

```desi
import http
from http import Request

def on_message(conn: int, msg: str):
    print("Received: " + msg)
    http.ws_send(conn, "echo: " + msg)

def on_open(conn: int):
    print("Client connected")

def on_close(conn: int):
    print("Client disconnected")

def handle_request(req: Request) -> Any:
    return http.html(200, "<h1>WebSocket Server</h1>")

def main() -> int:
    let srv = http.server(9090)

    # Register WebSocket handlers
    http.ws(srv, "/ws", on_message)
    http.ws_on_open(srv, on_open)
    http.ws_on_close(srv, on_close)

    http.serve(srv, handle_request)
    return 0
```

Connect from JavaScript: `new WebSocket("ws://localhost:9090/ws")`.

## API Reference

### Setup

| Function | Description |
|---|---|
| `http.ws(srv, path, on_message)` | Register WebSocket endpoint at `path` with message handler |
| `http.ws_on_open(srv, handler)` | Called when a client connects |
| `http.ws_on_close(srv, handler)` | Called when a client disconnects |

### Messaging

| Function | Description |
|---|---|
| `http.ws_send(conn, msg)` | Send a message to a specific connection |
| `http.ws_broadcast(srv, msg)` | Send a message to all connected clients |
| `http.ws_close(conn)` | Close a specific connection |

### Rooms

| Function | Description |
|---|---|
| `http.ws_join(conn, room)` | Add a connection to a named room |
| `http.ws_leave(conn, room)` | Remove a connection from a room |
| `http.ws_to_room(srv, room, msg)` | Broadcast to all connections in a room |

## Rooms Example

```desi
def on_message(conn: int, msg: str):
    if msg == "/join lobby":
        http.ws_join(conn, "lobby")
        http.ws_send(conn, "Joined lobby")
    elif msg == "/leave lobby":
        http.ws_leave(conn, "lobby")
        http.ws_send(conn, "Left lobby")
    else:
        # Broadcast to lobby room
        http.ws_to_room(srv, "lobby", msg)
```

## TLS (WSS)

WebSocket connections are automatically upgraded to WSS when using an HTTPS server:

```desi
let srv = http.server_tls(9090, "cert.pem", "key.pem")
http.ws(srv, "/ws", on_message)
http.serve(srv, handle_request)
# Clients connect via wss://localhost:9090/ws
```

## See Also

- [HTTP Server](http_server.md) — Server setup, routing, middleware
- [HTTP Client](http.md) — Making HTTP requests
