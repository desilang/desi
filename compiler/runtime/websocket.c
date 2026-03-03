/*
 * websocket.c — WebSocket runtime for Desi (RFC 6455)
 *
 * Self-contained: minimal SHA-1 + Base64 (protocol handshake only).
 * Actual transport security via TLS (wss://) handled by http layer.
 *
 * Public API:
 *   ws_do_handshake(fd, key)          → send 101 Switching Protocols
 *   ws_read_frame(fd, buf, cap)       → read + decode one frame
 *   ws_send_text(fd, msg, len)        → send text frame
 *   ws_send_close(fd, code)           → send close frame
 *   ws_send_pong(fd, data, len)       → send pong frame
 *
 * Server integration API (called from http_server.c):
 *   __ws_send(conn, msg)
 *   __ws_broadcast(srv_ptr, msg)
 *   __ws_close(conn)
 *   __ws_join(conn, room)
 *   __ws_leave(conn, room)
 *   __ws_to_room(srv_ptr, room, msg)
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>
#include <errno.h>
#include "websocket.h"

/* ============================================================
 * Minimal SHA-1 (RFC 3174) — protocol handshake only
 * ============================================================ */

static void sha1_transform(uint32_t state[5], const uint8_t block[64]) {
    uint32_t w[80];
    for (int i = 0; i < 16; i++) {
        w[i] = ((uint32_t)block[i*4] << 24) | ((uint32_t)block[i*4+1] << 16) |
               ((uint32_t)block[i*4+2] << 8) | (uint32_t)block[i*4+3];
    }
    for (int i = 16; i < 80; i++) {
        uint32_t t = w[i-3] ^ w[i-8] ^ w[i-14] ^ w[i-16];
        w[i] = (t << 1) | (t >> 31);
    }
    uint32_t a = state[0], b = state[1], c = state[2], d = state[3], e = state[4];
    for (int i = 0; i < 80; i++) {
        uint32_t f, k;
        if (i < 20)      { f = (b & c) | (~b & d); k = 0x5A827999; }
        else if (i < 40) { f = b ^ c ^ d;          k = 0x6ED9EBA1; }
        else if (i < 60) { f = (b & c) | (b & d) | (c & d); k = 0x8F1BBCDC; }
        else              { f = b ^ c ^ d;          k = 0xCA62C1D6; }
        uint32_t tmp = ((a << 5) | (a >> 27)) + f + e + k + w[i];
        e = d; d = c; c = (b << 30) | (b >> 2); b = a; a = tmp;
    }
    state[0] += a; state[1] += b; state[2] += c; state[3] += d; state[4] += e;
}

static void sha1(const uint8_t* data, size_t len, uint8_t hash[20]) {
    uint32_t state[5] = {0x67452301, 0xEFCDAB89, 0x98BADCFE, 0x10325476, 0xC3D2E1F0};
    size_t i;
    for (i = 0; i + 64 <= len; i += 64)
        sha1_transform(state, data + i);
    /* Padding */
    uint8_t block[64];
    size_t rem = len - i;
    memcpy(block, data + i, rem);
    block[rem++] = 0x80;
    if (rem > 56) {
        memset(block + rem, 0, 64 - rem);
        sha1_transform(state, block);
        memset(block, 0, 56);
    } else {
        memset(block + rem, 0, 56 - rem);
    }
    uint64_t bits = (uint64_t)len * 8;
    for (int j = 0; j < 8; j++)
        block[56 + j] = (uint8_t)(bits >> (56 - 8*j));
    sha1_transform(state, block);
    for (int j = 0; j < 5; j++) {
        hash[j*4]   = (uint8_t)(state[j] >> 24);
        hash[j*4+1] = (uint8_t)(state[j] >> 16);
        hash[j*4+2] = (uint8_t)(state[j] >> 8);
        hash[j*4+3] = (uint8_t)(state[j]);
    }
}

/* ============================================================
 * Base64 Encode
 * ============================================================ */

static const char b64_table[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

static void base64_encode(const uint8_t* in, size_t len, char* out) {
    size_t i, j = 0;
    for (i = 0; i + 2 < len; i += 3) {
        out[j++] = b64_table[(in[i] >> 2) & 0x3F];
        out[j++] = b64_table[((in[i] & 0x3) << 4) | ((in[i+1] >> 4) & 0xF)];
        out[j++] = b64_table[((in[i+1] & 0xF) << 2) | ((in[i+2] >> 6) & 0x3)];
        out[j++] = b64_table[in[i+2] & 0x3F];
    }
    if (i < len) {
        out[j++] = b64_table[(in[i] >> 2) & 0x3F];
        if (i + 1 < len) {
            out[j++] = b64_table[((in[i] & 0x3) << 4) | ((in[i+1] >> 4) & 0xF)];
            out[j++] = b64_table[((in[i+1] & 0xF) << 2)];
        } else {
            out[j++] = b64_table[((in[i] & 0x3) << 4)];
            out[j++] = '=';
        }
        out[j++] = '=';
    }
    out[j] = '\0';
}

/* ============================================================
 * WebSocket Handshake (RFC 6455 §4.2.2)
 * ============================================================ */

static const char* WS_MAGIC = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";

int ws_do_handshake(int fd, const char* client_key) {
    if (!client_key || !client_key[0]) return -1;

    /* Concatenate key + magic GUID */
    char concat[256];
    snprintf(concat, sizeof(concat), "%s%s", client_key, WS_MAGIC);

    /* SHA-1 hash */
    uint8_t hash[20];
    sha1((const uint8_t*)concat, strlen(concat), hash);

    /* Base64 encode */
    char accept_key[64];
    base64_encode(hash, 20, accept_key);

    /* Send 101 response */
    char response[512];
    int len = snprintf(response, sizeof(response),
        "HTTP/1.1 101 Switching Protocols\r\n"
        "Upgrade: websocket\r\n"
        "Connection: Upgrade\r\n"
        "Sec-WebSocket-Accept: %s\r\n"
        "\r\n", accept_key);

    ssize_t sent = WS_SEND(fd, response, len);
    return (int)sent;
}

/* ============================================================
 * Frame Opcodes
 * ============================================================ */

#define WS_OP_CONTINUATION 0x0
#define WS_OP_TEXT         0x1
#define WS_OP_BINARY       0x2
#define WS_OP_CLOSE        0x8
#define WS_OP_PING         0x9
#define WS_OP_PONG         0xA

/* ============================================================
 * Frame Reading (server receives masked frames from client)
 * ============================================================ */

typedef struct {
    int    opcode;       /* frame opcode */
    int    fin;          /* FIN bit (1 = final fragment) */
    char*  payload;      /* decoded payload (caller frees) */
    size_t payload_len;
} WsFrame;

/* ---- Per-connection recv prebuffer ----
 * When handle_client reads the HTTP upgrade request via recv(), it may read
 * bytes belonging to the WebSocket session (e.g. browser pipelined frame).
 * These leftover bytes are stored here and drained BEFORE calling recv().
 */
typedef struct {
    const uint8_t* data;
    size_t         len;
    size_t         pos;
} WsRecvBuf;

/* Thread-local so each worker's WS session has its own buffer */
static _Thread_local WsRecvBuf _ws_prebuf = {NULL, 0, 0};

static void ws_set_prebuf(const uint8_t* data, size_t len) {
    _ws_prebuf.data = data;
    _ws_prebuf.len  = len;
    _ws_prebuf.pos  = 0;
}

/* Read exactly n bytes: drain prebuffer first, then call recv() */
static int ws_recv_exact(int fd, void* buf, size_t n) {
    uint8_t* out   = (uint8_t*)buf;
    size_t   total = 0;

    /* 1. Drain prebuffer first */
    if (_ws_prebuf.data && _ws_prebuf.pos < _ws_prebuf.len) {
        size_t avail = _ws_prebuf.len - _ws_prebuf.pos;
        size_t take  = avail < n ? avail : n;
        memcpy(out, _ws_prebuf.data + _ws_prebuf.pos, take);
        _ws_prebuf.pos += take;
        total += take;
    }

    /* 2. Read remainder from socket */
    while (total < n) {
        ssize_t r = WS_RECV(fd, out + total, n - total);
        if (r <= 0) return -1;
        total += (size_t)r;
    }
    return 0;
}

/* Read one WebSocket frame. Returns 0 on success, -1 on error/close */
int ws_read_frame(int fd, WsFrame* frame) {
    uint8_t hdr[2];
    if (ws_recv_exact(fd, hdr, 2) < 0) return -1;

    frame->fin = (hdr[0] >> 7) & 1;
    frame->opcode = hdr[0] & 0x0F;
    int masked = (hdr[1] >> 7) & 1;
    uint64_t payload_len = hdr[1] & 0x7F;

    /* Extended payload length */
    if (payload_len == 126) {
        uint8_t ext[2];
        if (ws_recv_exact(fd, ext, 2) < 0) return -1;
        payload_len = ((uint64_t)ext[0] << 8) | ext[1];
    } else if (payload_len == 127) {
        uint8_t ext[8];
        if (ws_recv_exact(fd, ext, 8) < 0) return -1;
        payload_len = 0;
        for (int i = 0; i < 8; i++)
            payload_len = (payload_len << 8) | ext[i];
    }

    /* Masking key (client→server frames MUST be masked) */
    uint8_t mask[4] = {0};
    if (masked) {
        if (ws_recv_exact(fd, mask, 4) < 0) return -1;
    }

    /* Payload (limit to 16MB) */
    if (payload_len > 16 * 1024 * 1024) return -1;

    frame->payload = (char*)malloc(payload_len + 1);
    if (!frame->payload) return -1;

    if (payload_len > 0) {
        if (ws_recv_exact(fd, frame->payload, (size_t)payload_len) < 0) {
            free(frame->payload);
            frame->payload = NULL;
            return -1;
        }
        /* Unmask */
        if (masked) {
            for (size_t i = 0; i < payload_len; i++)
                frame->payload[i] ^= mask[i % 4];
        }
    }
    frame->payload[payload_len] = '\0';
    frame->payload_len = (size_t)payload_len;
    return 0;
}

/* ============================================================
 * Frame Writing (server sends unmasked frames to client)
 * ============================================================ */

static int ws_send_frame(int fd, int opcode, const char* data, size_t len) {
    uint8_t header[10];
    int hdr_len = 0;

    header[0] = 0x80 | (opcode & 0x0F); /* FIN=1 + opcode */
    hdr_len++;

    if (len < 126) {
        header[1] = (uint8_t)len; /* no mask bit for server */
        hdr_len = 2;
    } else if (len <= 0xFFFF) {
        header[1] = 126;
        header[2] = (uint8_t)(len >> 8);
        header[3] = (uint8_t)(len & 0xFF);
        hdr_len = 4;
    } else {
        header[1] = 127;
        for (int i = 0; i < 8; i++)
            header[2 + i] = (uint8_t)(len >> (56 - 8*i));
        hdr_len = 10;
    }

    if (WS_SEND(fd, header, hdr_len) < 0) return -1;
    if (len > 0 && WS_SEND(fd, data, len) < 0) return -1;
    return 0;
}

int ws_send_text(int fd, const char* msg, size_t len) {
    return ws_send_frame(fd, WS_OP_TEXT, msg, len);
}

int ws_send_close(int fd, uint16_t code) {
    uint8_t payload[2] = {(uint8_t)(code >> 8), (uint8_t)(code & 0xFF)};
    return ws_send_frame(fd, WS_OP_CLOSE, (const char*)payload, 2);
}

int ws_send_pong(int fd, const char* data, size_t len) {
    return ws_send_frame(fd, WS_OP_PONG, data, len);
}

int ws_send_ping(int fd) {
    return ws_send_frame(fd, WS_OP_PING, NULL, 0);
}

/* ============================================================
 * Connection + Room Tracking
 * ============================================================ */

/* Global WS state (shared with http_server.c via websocket.h) */
WsState __ws_state = {0};

/* Register a connection */
static WsConnection* ws_register_conn(int fd) {
    for (int i = 0; i < WS_MAX_CONNECTIONS; i++) {
        if (__ws_state.conns[i].fd == 0) {
            __ws_state.conns[i].fd = fd;
            __ws_state.conns[i].room_count = 0;
            __ws_state.conn_count++;
            return &__ws_state.conns[i];
        }
    }
    return NULL; /* full */
}

/* Unregister a connection */
static void ws_unregister_conn(int fd) {
    for (int i = 0; i < WS_MAX_CONNECTIONS; i++) {
        if (__ws_state.conns[i].fd == fd) {
            __ws_state.conns[i].fd = 0;
            __ws_state.conns[i].room_count = 0;
            __ws_state.conn_count--;
            return;
        }
    }
}

/* Find connection by fd */
static WsConnection* ws_find_conn(int fd) {
    for (int i = 0; i < WS_MAX_CONNECTIONS; i++) {
        if (__ws_state.conns[i].fd == fd)
            return &__ws_state.conns[i];
    }
    return NULL;
}

/* ============================================================
 * Desi-callable API functions
 * ============================================================ */

/* Send text message to a single connection */
void __ws_send(int conn_fd, const char* msg) {
    if (conn_fd <= 0 || !msg) return;
    ws_send_text(conn_fd, msg, strlen(msg));
}

/* Broadcast text to ALL WebSocket connections */
void __ws_broadcast(const char* msg) {
    if (!msg) return;
    size_t len = strlen(msg);
    for (int i = 0; i < WS_MAX_CONNECTIONS; i++) {
        if (__ws_state.conns[i].fd > 0) {
            ws_send_text(__ws_state.conns[i].fd, msg, len);
        }
    }
}

/* Close a WebSocket connection */
void __ws_close(int conn_fd) {
    if (conn_fd <= 0) return;
    ws_send_close(conn_fd, 1000); /* 1000 = normal closure */
    ws_unregister_conn(conn_fd);
    close(conn_fd);
}

/* Join a room */
void __ws_join(int conn_fd, const char* room) {
    if (conn_fd <= 0 || !room) return;
    WsConnection* conn = ws_find_conn(conn_fd);
    if (!conn || conn->room_count >= WS_MAX_ROOMS) return;
    /* Check if already in room */
    for (int i = 0; i < conn->room_count; i++) {
        if (strcmp(conn->rooms[i], room) == 0) return;
    }
    strncpy(conn->rooms[conn->room_count], room, WS_ROOM_NAME_LEN - 1);
    conn->rooms[conn->room_count][WS_ROOM_NAME_LEN - 1] = '\0';
    conn->room_count++;
}

/* Leave a room */
void __ws_leave(int conn_fd, const char* room) {
    if (conn_fd <= 0 || !room) return;
    WsConnection* conn = ws_find_conn(conn_fd);
    if (!conn) return;
    for (int i = 0; i < conn->room_count; i++) {
        if (strcmp(conn->rooms[i], room) == 0) {
            /* Shift remaining rooms down */
            for (int j = i; j < conn->room_count - 1; j++)
                memcpy(conn->rooms[j], conn->rooms[j+1], WS_ROOM_NAME_LEN);
            conn->room_count--;
            return;
        }
    }
}

/* Broadcast to a specific room */
void __ws_to_room(const char* room, const char* msg) {
    if (!room || !msg) return;
    size_t len = strlen(msg);
    for (int i = 0; i < WS_MAX_CONNECTIONS; i++) {
        WsConnection* conn = &__ws_state.conns[i];
        if (conn->fd <= 0) continue;
        for (int r = 0; r < conn->room_count; r++) {
            if (strcmp(conn->rooms[r], room) == 0) {
                ws_send_text(conn->fd, msg, len);
                break;
            }
        }
    }
}

/* Set WS endpoint path */
void __ws_set_path(const char* path) {
    if (path) strncpy(__ws_state.ws_path, path, sizeof(__ws_state.ws_path) - 1);
}

/* Set handlers (called from lowerer) */
void __ws_set_on_message(void* fn) { __ws_state.on_message = fn; }
void __ws_set_on_open(void* fn)    { __ws_state.on_open = fn; }
void __ws_set_on_close(void* fn)   { __ws_state.on_close = fn; }

/* ============================================================
 * WebSocket Session Loop (called by http_server.c after upgrade)
 *
 * Reads frames in a loop, dispatches to Desi handlers.
 * The on_message handler is called as: handler(conn_fd, msg_str)
 * ============================================================ */

typedef void (*ws_message_fn)(int conn_fd, const char* msg);
typedef void (*ws_lifecycle_fn)(int conn_fd);

void ws_session_loop(int client_fd, const uint8_t* prebuf, size_t prebuf_len) {
    /* Install any bytes that were already read past the HTTP headers */
    ws_set_prebuf(prebuf, prebuf_len);



    /* Register connection */
    WsConnection* conn = ws_register_conn(client_fd);
    if (!conn) {
        fprintf(stderr, "[ws] max connections reached, closing fd=%d\n", client_fd);
        ws_send_close(client_fd, 1013); /* Try Again */
        close(client_fd);
        return;
    }

    /* Call on_open handler */
    if (__ws_state.on_open) {
        ((ws_lifecycle_fn)__ws_state.on_open)(client_fd);
    }

    printf("[ws] client connected fd=%d (%d total)\n", client_fd, __ws_state.conn_count);
    fflush(stdout);

    /* Read frame loop */
    WsFrame frame;
    while (1) {
        if (ws_read_frame(client_fd, &frame) < 0) break;

        switch (frame.opcode) {
        case WS_OP_TEXT:
        case WS_OP_BINARY:
            if (__ws_state.on_message) {
                ((ws_message_fn)__ws_state.on_message)(client_fd, frame.payload);
            }
            break;

        case WS_OP_PING:
            ws_send_pong(client_fd, frame.payload, frame.payload_len);
            break;

        case WS_OP_CLOSE:
            /* Echo close frame back */
            ws_send_close(client_fd, 1000);
            free(frame.payload);
            goto done;

        case WS_OP_PONG:
            /* Ignore unsolicited pong */
            break;
        }

        free(frame.payload);
    }

done:
    /* Call on_close handler */
    if (__ws_state.on_close) {
        ((ws_lifecycle_fn)__ws_state.on_close)(client_fd);
    }

    printf("[ws] client disconnected fd=%d (%d remaining)\n", client_fd, __ws_state.conn_count - 1);
    fflush(stdout);

    /* Clear prebuffer (safety) */
    ws_set_prebuf(NULL, 0);

    ws_unregister_conn(client_fd);
    close(client_fd);
}
