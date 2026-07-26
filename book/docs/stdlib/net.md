# net — Low-Level Networking

The `net` module provides raw TCP and UDP socket operations for custom protocols. For HTTP/WebSocket, use the `http` module instead.

## Import

```desi
import net
```

## API Reference

### TCP Client

| Function | Description |
|---|---|
| `net.dial(host, port) -> int` | Connect to host:port, returns socket fd |
| `net.send(fd, data) -> int` | Send data, returns bytes sent |
| `net.recv(fd, max_bytes) -> str` | Receive data |
| `net.close(fd)` | Close socket |

### TCP Server

| Function | Description |
|---|---|
| `net.listen(host, port) -> int` | Bind and listen, returns server fd |
| `net.accept(server_fd) -> int` | Accept connection, returns client fd |
| `net.peer_addr(fd) -> str` | Get peer address as `"ip:port"` |

### UDP

| Function | Description |
|---|---|
| `net.udp_open(host, port) -> int` | Open UDP socket |
| `net.udp_send(fd, host, port, data) -> int` | Send datagram |
| `net.udp_recv(fd, max_bytes) -> str` | Receive datagram |

### DNS

| Function | Description |
|---|---|
| `net.resolve(hostname) -> str` | Resolve hostname to IP address |

### Socket Options

| Function | Description |
|---|---|
| `net.set_nonblocking(fd) -> int` | Set non-blocking mode |
| `net.set_timeout(fd, ms) -> int` | Set timeout in milliseconds |

## Examples

### TCP Echo Client

```desi
import net

let fd = net.dial("127.0.0.1", 8080)
net.send(fd, "hello")
let reply = net.recv(fd, 1024)
print(reply)
net.close(fd)
```

### TCP Server

```desi
import net

let srv = net.listen("127.0.0.1", 8080)
let client = net.accept(srv)
let data = net.recv(client, 1024)
net.send(client, f"echo: {data}")
net.close(client)
net.close(srv)
```

### DNS Lookup

```desi
import net

let ip = net.resolve("example.com")
print(f"IP: {ip}")
```

## net vs http

| Feature | `net` | `http` |
|---|---|---|
| TCP sockets | ✅ | ❌ (internal) |
| UDP sockets | ✅ | ❌ |
| DNS resolution | ✅ | ❌ (internal) |
| HTTP client | ❌ | ✅ |
| HTTP server | ❌ | ✅ |
| WebSocket | ❌ | ✅ |
| TLS/HTTPS | ❌ (use http) | ✅ |

Use `net` for custom protocols (game servers, chat, IRC). Use `http` for web APIs and WebSocket.

## Error handling

`dial`, `listen`, and `accept` return `-1` on failure. Always check the
result before using the descriptor — a blocking `accept()` will otherwise
wait forever for a peer that never connects:

```desi
let fd = net.dial("127.0.0.1", 8080)
if fd < 0:
	print("connect failed")
	return 1
```

On Windows, an intermittent `dial` failure on `127.0.0.1` is usually local
port exhaustion rather than a problem with your code — see
[Windows: dial failed on localhost](../guides/windows-networking.md).
