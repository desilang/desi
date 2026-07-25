/*
 * net.c — Low-level networking for Desi stdlib
 *
 * Provides raw TCP and UDP socket operations.
 * Cross-platform: POSIX (macOS/Linux) + WinSock2 (Windows).
 *
 * Architecture:
 *   net.c sits BELOW http.c/websocket.c — those are built on top of
 *   higher-level abstractions. net.c gives direct socket access for
 *   custom protocols, game servers, chat systems, etc.
 *
 * Public API:
 *   TCP Client:
 *     __net_dial(host, port)        → connect, return fd
 *     __net_send(fd, data)          → send string, return bytes sent
 *     __net_recv(fd, max_bytes)     → receive string
 *     __net_close(fd)               → close socket
 *
 *   TCP Server:
 *     __net_listen(host, port)      → bind+listen, return server fd
 *     __net_accept(server_fd)       → accept connection, return client fd
 *
 *   UDP:
 *     __net_udp_open(host, port)    → bind UDP socket, return fd
 *     __net_udp_send(fd, host, port, data) → send datagram
 *     __net_udp_recv(fd, max_bytes) → receive datagram
 *
 *   DNS:
 *     __net_resolve(hostname)       → resolve to IP string
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <stdint.h>

#ifdef _WIN32
  #include <winsock2.h>
  #include <ws2tcpip.h>
  #pragma comment(lib, "ws2_32.lib")
  typedef int socklen_t;
  /* ssize_t is POSIX; Winsock2 send/recv return int, so int is correct here */
  typedef int ssize_t;
  #define CLOSE_SOCKET closesocket
  static int wsa_initialized = 0;
  static void ensure_wsa(void) {
      if (!wsa_initialized) {
          WSADATA wsa;
          WSAStartup(MAKEWORD(2, 2), &wsa);
          wsa_initialized = 1;
      }
  }
#else
  #include <unistd.h>
  #include <sys/socket.h>
  #include <sys/types.h>
  #include <netinet/in.h>
  #include <netdb.h>
  #include <arpa/inet.h>
  #include <fcntl.h>
  #define CLOSE_SOCKET close
  static void ensure_wsa(void) {} /* no-op on POSIX */
#endif

// ============================================================
// TCP Client
// ============================================================

// Create a socket for `p` and connect it; return a connected fd, or -1.
//
// On Windows, a busy loopback can transiently fail connect() with
// WSAEADDRINUSE: the auto-selected local ephemeral port collides with a
// lingering TIME_WAIT 4-tuple from a previous connection. A fresh socket
// picks a different local port, so we retry a bounded number of times
// before giving up. POSIX keeps its single-attempt behavior unchanged.
static int net_connect_one(struct addrinfo* p) {
    int fd = socket(p->ai_family, p->ai_socktype, p->ai_protocol);
    if (fd < 0) return -1;
    if (connect(fd, p->ai_addr, p->ai_addrlen) == 0) return fd;
#ifdef _WIN32
    for (int attempt = 0; attempt < 64 && WSAGetLastError() == WSAEADDRINUSE; attempt++) {
        CLOSE_SOCKET(fd);
        Sleep(1);
        fd = socket(p->ai_family, p->ai_socktype, p->ai_protocol);
        if (fd < 0) return -1;
        if (connect(fd, p->ai_addr, p->ai_addrlen) == 0) return fd;
    }
#endif
    CLOSE_SOCKET(fd);
    return -1;
}

// Connect to host:port, return socket fd (-1 on error)
int32_t __net_dial(const char* host, int32_t port) {
    ensure_wsa();
    if (!host) return -1;

    struct addrinfo hints, *res, *p;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;     // IPv4 or IPv6
    hints.ai_socktype = SOCK_STREAM; // TCP

    char port_str[16];
    snprintf(port_str, sizeof(port_str), "%d", port);

    int status = getaddrinfo(host, port_str, &hints, &res);
    if (status != 0) return -1;

    int fd = -1;
    for (p = res; p != NULL; p = p->ai_next) {
        fd = net_connect_one(p);
        if (fd >= 0) break; // Connected
    }

    freeaddrinfo(res);
    return (int32_t)fd;
}

// Send data on a socket, return bytes sent (-1 on error)
int32_t __net_send(int32_t fd, const char* data) {
    if (!data || fd < 0) return -1;
    size_t len = strlen(data);
    ssize_t sent = send(fd, data, len, 0);
    return (int32_t)sent;
}

// Receive up to max_bytes from socket, return string (empty on error)
char* __net_recv(int32_t fd, int32_t max_bytes) {
    if (fd < 0 || max_bytes <= 0) return strdup("");
    char* buf = (char*)malloc(max_bytes + 1);
    if (!buf) return strdup("");

    ssize_t n = recv(fd, buf, max_bytes, 0);
    if (n <= 0) {
        free(buf);
        return strdup("");
    }
    buf[n] = '\0';
    return buf;
}

// Close a socket
void __net_close(int32_t fd) {
    if (fd >= 0) {
        CLOSE_SOCKET(fd);
    }
}

// ============================================================
// TCP Server
// ============================================================

// Listen on host:port, return server fd (-1 on error)
int32_t __net_listen(const char* host, int32_t port) {
    ensure_wsa();

    struct addrinfo hints, *res;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_flags = AI_PASSIVE;

    char port_str[16];
    snprintf(port_str, sizeof(port_str), "%d", port);

    const char* node = (host && strlen(host) > 0) ? host : NULL;
    int status = getaddrinfo(node, port_str, &hints, &res);
    if (status != 0) return -1;

    int fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
    if (fd < 0) {
        freeaddrinfo(res);
        return -1;
    }

    // Allow address reuse
    int opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    if (bind(fd, res->ai_addr, res->ai_addrlen) < 0) {
        CLOSE_SOCKET(fd);
        freeaddrinfo(res);
        return -1;
    }

    freeaddrinfo(res);

    if (listen(fd, 128) < 0) {
        CLOSE_SOCKET(fd);
        return -1;
    }

    return (int32_t)fd;
}

// Accept a connection, return client fd (-1 on error)
int32_t __net_accept(int32_t server_fd) {
    if (server_fd < 0) return -1;

    struct sockaddr_storage client_addr;
    socklen_t addr_len = sizeof(client_addr);

    int client_fd = accept(server_fd, (struct sockaddr*)&client_addr, &addr_len);
    return (int32_t)client_fd;
}

// Get peer address as "ip:port" string
char* __net_peer_addr(int32_t fd) {
    if (fd < 0) return strdup("");

    struct sockaddr_storage addr;
    socklen_t len = sizeof(addr);
    if (getpeername(fd, (struct sockaddr*)&addr, &len) < 0) {
        return strdup("");
    }

    char ip[INET6_ADDRSTRLEN];
    int port;

    if (addr.ss_family == AF_INET) {
        struct sockaddr_in* s = (struct sockaddr_in*)&addr;
        inet_ntop(AF_INET, &s->sin_addr, ip, sizeof(ip));
        port = ntohs(s->sin_port);
    } else {
        struct sockaddr_in6* s = (struct sockaddr_in6*)&addr;
        inet_ntop(AF_INET6, &s->sin6_addr, ip, sizeof(ip));
        port = ntohs(s->sin6_port);
    }

    char* result = (char*)malloc(INET6_ADDRSTRLEN + 8);
    snprintf(result, INET6_ADDRSTRLEN + 8, "%s:%d", ip, port);
    return result;
}

// ============================================================
// UDP
// ============================================================

// Open a UDP socket, optionally bind to host:port
int32_t __net_udp_open(const char* host, int32_t port) {
    ensure_wsa();

    int fd = socket(AF_INET, SOCK_DGRAM, 0);
    if (fd < 0) return -1;

    // Bind if port > 0
    if (port > 0) {
        struct sockaddr_in addr;
        memset(&addr, 0, sizeof(addr));
        addr.sin_family = AF_INET;
        addr.sin_port = htons(port);

        if (host && strlen(host) > 0) {
            inet_pton(AF_INET, host, &addr.sin_addr);
        } else {
            addr.sin_addr.s_addr = INADDR_ANY;
        }

        if (bind(fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
            CLOSE_SOCKET(fd);
            return -1;
        }
    }

    return (int32_t)fd;
}

// Send UDP datagram to host:port
int32_t __net_udp_send(int32_t fd, const char* host, int32_t port, const char* data) {
    if (fd < 0 || !host || !data) return -1;

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    inet_pton(AF_INET, host, &addr.sin_addr);

    ssize_t sent = sendto(fd, data, strlen(data), 0,
                          (struct sockaddr*)&addr, sizeof(addr));
    return (int32_t)sent;
}

// Receive UDP datagram, return string (empty on error/timeout)
char* __net_udp_recv(int32_t fd, int32_t max_bytes) {
    if (fd < 0 || max_bytes <= 0) return strdup("");
    char* buf = (char*)malloc(max_bytes + 1);
    if (!buf) return strdup("");

    struct sockaddr_storage src_addr;
    socklen_t addr_len = sizeof(src_addr);

    ssize_t n = recvfrom(fd, buf, max_bytes, 0,
                         (struct sockaddr*)&src_addr, &addr_len);
    if (n <= 0) {
        free(buf);
        return strdup("");
    }
    buf[n] = '\0';
    return buf;
}

// ============================================================
// DNS
// ============================================================

// Resolve hostname to IP address string
char* __net_resolve(const char* hostname) {
    ensure_wsa();
    if (!hostname) return strdup("");

    struct addrinfo hints, *res;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;

    int status = getaddrinfo(hostname, NULL, &hints, &res);
    if (status != 0) return strdup("");

    char ip[INET6_ADDRSTRLEN];
    if (res->ai_family == AF_INET) {
        struct sockaddr_in* s = (struct sockaddr_in*)res->ai_addr;
        inet_ntop(AF_INET, &s->sin_addr, ip, sizeof(ip));
    } else {
        struct sockaddr_in6* s = (struct sockaddr_in6*)res->ai_addr;
        inet_ntop(AF_INET6, &s->sin6_addr, ip, sizeof(ip));
    }

    freeaddrinfo(res);
    return strdup(ip);
}

// Set socket to non-blocking mode
int32_t __net_set_nonblocking(int32_t fd) {
    if (fd < 0) return -1;
#ifdef _WIN32
    u_long mode = 1;
    return ioctlsocket(fd, FIONBIO, &mode) == 0 ? 0 : -1;
#else
    int flags = fcntl(fd, F_GETFL, 0);
    if (flags < 0) return -1;
    return fcntl(fd, F_SETFL, flags | O_NONBLOCK) < 0 ? -1 : 0;
#endif
}

// Set socket timeout in milliseconds
int32_t __net_set_timeout(int32_t fd, int32_t timeout_ms) {
    if (fd < 0) return -1;
#ifdef _WIN32
    DWORD tv = timeout_ms;
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, (const char*)&tv, sizeof(tv));
    setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, (const char*)&tv, sizeof(tv));
#else
    struct timeval tv;
    tv.tv_sec = timeout_ms / 1000;
    tv.tv_usec = (timeout_ms % 1000) * 1000;
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));
#endif
    return 0;
}
