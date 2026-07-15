/*
 * websocket.h — WebSocket types and API for Desi
 */

#ifndef DESI_WEBSOCKET_H
#define DESI_WEBSOCKET_H

#include <stdint.h>
#include <stddef.h>
#include "platform.h"
#include "tls.h"

/* Thread-local storage qualifier: MSVC C mode lacks _Thread_local */
#ifndef DESI_TLS_QUAL
  #if defined(_MSC_VER)
    #define DESI_TLS_QUAL __declspec(thread)
  #else
    #define DESI_TLS_QUAL _Thread_local
  #endif
#endif

/* Thread-local SSL pointer for current WS session */
extern DESI_TLS_QUAL DESI_SSL* _ws_current_ssl;

#ifdef _WIN32
  #include <winsock2.h>
#else
  #include <sys/socket.h>
#endif

/* TLS-aware WS macros: use thread-local SSL for current session */
#define WS_SEND(fd, buf, len) DESI_SEND(fd, _ws_current_ssl, buf, len)
#define WS_RECV(fd, buf, len) DESI_RECV(fd, _ws_current_ssl, buf, len)

#define WS_MAX_CONNECTIONS 256
#define WS_MAX_ROOMS 32
#define WS_ROOM_NAME_LEN 64
#define WS_MAX_PATHS 8
#define WS_DEFAULT_MAX_MSG_SIZE (16 * 1024 * 1024) /* 16 MB */

typedef struct {
    int  fd;
    DESI_SSL* ssl;  /* non-NULL for WSS connections */
    char rooms[WS_MAX_ROOMS][WS_ROOM_NAME_LEN];
    int  room_count;
    int  compression; /* 1 = permessage-deflate negotiated */
} WsConnection;

typedef struct {
    char path[1024];
    void* on_message;   /* void (*)(int, const char*) */
    void* on_binary;    /* void (*)(int, const char*, size_t) */
    void* on_open;      /* void (*)(int) */
    void* on_close;     /* void (*)(int) */
} WsRoute;

typedef struct {
    WsConnection conns[WS_MAX_CONNECTIONS];
    int conn_count;
    void* on_message;   /* default message handler (legacy) */
    void* on_open;      /* default open handler */
    void* on_close;     /* default close handler */
    char  ws_path[1024]; /* primary path (legacy) */
    WsRoute routes[WS_MAX_PATHS];
    int route_count;
    size_t max_message_size; /* 0 = use default (16MB) */
    int ping_interval_secs;  /* 0 = disabled */
    int compression_enabled; /* 1 = offer permessage-deflate */
    DesiPlatformRwLock lock; /* thread-safe access to connections/rooms */
} WsState;

extern WsState __ws_state;

/* Initialize WS state (must be called before accepting connections) */
void ws_state_init(void);

/* Handshake + session */
int  ws_do_handshake(int fd, const char* client_key, DESI_SSL* ssl);
int  ws_do_handshake_ext(int fd, const char* client_key, DESI_SSL* ssl,
                         const char* extensions, int* compression_out);
void ws_session_loop(int client_fd, const uint8_t* prebuf, size_t prebuf_len, DESI_SSL* ssl);

/* Frame operations */
int ws_send_text(int fd, const char* msg, size_t len);
int ws_send_binary(int fd, const char* data, size_t len);
int ws_send_close(int fd, uint16_t code);
int ws_send_pong(int fd, const char* data, size_t len);
int ws_send_ping(int fd);

/* Desi API */
void __ws_send(int conn_fd, const char* msg);
void __ws_send_binary(int conn_fd, const char* data, int len);
void __ws_broadcast(const char* msg);
void __ws_close(int conn_fd);
void __ws_join(int conn_fd, const char* room);
void __ws_leave(int conn_fd, const char* room);
void __ws_to_room(const char* room, const char* msg);
void __ws_set_path(const char* path);
void __ws_set_on_message(void* fn);
void __ws_set_on_open(void* fn);
void __ws_set_on_close(void* fn);
void __ws_set_max_message_size(int size);
void __ws_set_ping_interval(int secs);
void __ws_set_compression(int enabled);

/* Route-based API (multiple WS paths) */
void __ws_route(const char* path, void* on_message);
void __ws_route_on_open(const char* path, void* fn);
void __ws_route_on_close(const char* path, void* fn);
void __ws_route_on_binary(const char* path, void* fn);
const WsRoute* ws_find_route(const char* path);

#endif /* DESI_WEBSOCKET_H */
