/*
 * http_internal.h — Shared types and declarations for the HTTP module
 */
#ifndef DESI_HTTP_INTERNAL_H
#define DESI_HTTP_INTERNAL_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <ctype.h>
#include <stdint.h>

/* ---- Platform socket abstraction ---- */

#ifdef _WIN32
  #include <winsock2.h>
  #include <ws2tcpip.h>
  #pragma comment(lib, "ws2_32.lib")
  #define DESI_SOCKET SOCKET
  #define DESI_INVALID_SOCKET INVALID_SOCKET
  #define DESI_CLOSE_SOCKET closesocket
  typedef int ssize_t;
#else
  #include <unistd.h>
  #include <sys/socket.h>
  #include <sys/types.h>
  #include <netdb.h>
  #define DESI_SOCKET int
  #define DESI_INVALID_SOCKET (-1)
  #define DESI_CLOSE_SOCKET close
#endif

/* ---- Dynamic buffer ---- */

typedef struct {
    char *data;
    size_t len;
    size_t cap;
} Buffer;

static inline void buf_init(Buffer *b) {
    b->cap = 4096;
    b->data = (char *)malloc(b->cap);
    b->len = 0;
    if (b->data) b->data[0] = '\0';
}

static inline void buf_append(Buffer *b, const char *data, size_t n) {
    if (!b->data) return;
    while (b->len + n + 1 > b->cap) {
        b->cap *= 2;
        b->data = (char *)realloc(b->data, b->cap);
        if (!b->data) return;
    }
    memcpy(b->data + b->len, data, n);
    b->len += n;
    b->data[b->len] = '\0';
}

static inline void buf_free(Buffer *b) {
    free(b->data);
    b->data = NULL;
    b->len = b->cap = 0;
}

/* ---- Parsed URL ---- */

typedef struct {
    char scheme[16];
    char host[256];
    char port[8];
    char path[2048];
} ParsedURL;

/* ---- Connection (platform TLS fields added via includes) ---- */

typedef struct {
    DESI_SOCKET fd;
    int use_tls;
    void *tls_state; /* Opaque platform-specific TLS context */
} Connection;

/* ---- HTTP response ---- */

typedef struct {
    int status;
    char *body;
    char *headers;
} HttpResponse;

/* ---- Platform socket I/O ---- */

static inline ssize_t sock_send(DESI_SOCKET fd, const void *buf, size_t len) {
#ifdef _WIN32
    return send(fd, (const char *)buf, (int)len, 0);
#else
    return write(fd, buf, len);
#endif
}

static inline ssize_t sock_recv(DESI_SOCKET fd, void *buf, size_t len) {
#ifdef _WIN32
    return recv(fd, (char *)buf, (int)len, 0);
#else
    return read(fd, buf, len);
#endif
}

/* ---- TLS interface (implemented per platform) ---- */

static int  tls_handshake(Connection *c, const char *host);
static ssize_t tls_write(Connection *c, const void *buf, size_t len);
static ssize_t tls_read(Connection *c, void *buf, size_t len);
static void tls_close(Connection *c);

#endif /* DESI_HTTP_INTERNAL_H */
