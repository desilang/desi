/*
 * websocket.h — WebSocket types and API for Desi
 */

#ifndef DESI_WEBSOCKET_H
#define DESI_WEBSOCKET_H

#include <stdint.h>
#include <stddef.h>

#ifdef _WIN32
  #include <winsock2.h>
  #define WS_SEND(fd, buf, len) send(fd, buf, len, 0)
  #define WS_RECV(fd, buf, len) recv(fd, buf, len, 0)
#else
  #include <sys/socket.h>
  #define WS_SEND(fd, buf, len) send(fd, buf, len, 0)
  #define WS_RECV(fd, buf, len) recv(fd, buf, len, 0)
#endif

#define WS_MAX_CONNECTIONS 256
#define WS_MAX_ROOMS 32
#define WS_ROOM_NAME_LEN 64

typedef struct {
    int  fd;
    char rooms[WS_MAX_ROOMS][WS_ROOM_NAME_LEN];
    int  room_count;
} WsConnection;

typedef struct {
    WsConnection conns[WS_MAX_CONNECTIONS];
    int conn_count;
    void* on_message;
    void* on_open;
    void* on_close;
    char  ws_path[1024];
} WsState;

extern WsState __ws_state;

/* Handshake + session */
int  ws_do_handshake(int fd, const char* client_key);
void ws_session_loop(int client_fd, const uint8_t* prebuf, size_t prebuf_len);

/* Frame operations */
int ws_send_text(int fd, const char* msg, size_t len);
int ws_send_close(int fd, uint16_t code);
int ws_send_pong(int fd, const char* data, size_t len);
int ws_send_ping(int fd);

/* Desi API */
void __ws_send(int conn_fd, const char* msg);
void __ws_broadcast(const char* msg);
void __ws_close(int conn_fd);
void __ws_join(int conn_fd, const char* room);
void __ws_leave(int conn_fd, const char* room);
void __ws_to_room(const char* room, const char* msg);
void __ws_set_path(const char* path);
void __ws_set_on_message(void* fn);
void __ws_set_on_open(void* fn);
void __ws_set_on_close(void* fn);

#endif /* DESI_WEBSOCKET_H */
