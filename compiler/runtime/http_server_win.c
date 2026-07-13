/*
 * http_server_win.c — Windows link stubs for the HTTP server API
 *
 * http_server.c is POSIX-only (fork-free threaded accept loop, POSIX
 * sockets/signals) and is excluded from the Windows build. The Desi `http`
 * module bundles client AND server wrappers, and import-closure IR emission
 * references every pub def — so client-only programs still need the server
 * symbols to link. These stubs satisfy the linker and fail gracefully at
 * runtime; the HTTP client (http.c + Schannel TLS) is fully functional.
 *
 * Entirely compiled out on non-Windows platforms: the real http_server.c
 * provides these symbols there.
 */
#ifdef _WIN32

#include <stdio.h>
#include <stdint.h>
#include <stddef.h>

static void http_server_unsupported(const char* fn) {
    static int warned = 0;
    if (!warned) {
        fprintf(stderr,
                "desi: %s: the HTTP server is not yet supported on Windows "
                "(the HTTP client works)\n", fn);
        warned = 1;
    }
}

void* __http_server_new(int port) { (void)port; http_server_unsupported("http.server"); return NULL; }
void* __http_server_new_tls(int port, const char* cert, const char* key) {
    (void)port; (void)cert; (void)key;
    http_server_unsupported("http.server_tls");
    return NULL;
}
void __http_server_run(void* s) { (void)s; http_server_unsupported("http.listen"); }
void __http_server_shutdown(void* s) { (void)s; }
void __http_server_route(void* s, const char* method, const char* path, void* handler) {
    (void)s; (void)method; (void)path; (void)handler;
    http_server_unsupported("http.route");
}
void __http_server_static(void* s, const char* prefix, const char* dir) {
    (void)s; (void)prefix; (void)dir;
    http_server_unsupported("http.serve_static");
}
void __http_server_set_handler(void* s, void* handler) { (void)s; (void)handler; }
void __http_server_max_body(void* s, int64_t bytes) { (void)s; (void)bytes; }
void __http_server_timeout(void* s, int seconds) { (void)s; (void)seconds; }
void __http_server_use(void* s, void* middleware) { (void)s; (void)middleware; }
void __http_server_rate_limit(void* s, int per_minute) { (void)s; (void)per_minute; }
void __http_server_cors(void* s, const char* origins) { (void)s; (void)origins; }

/* Request accessors — never reached without a running server */
const char* __http_req_method(void* req) { (void)req; return ""; }
const char* __http_req_path(void* req) { (void)req; return ""; }
const char* __http_req_body(void* req) { (void)req; return ""; }
const char* __http_req_header(void* req, const char* name) { (void)req; (void)name; return ""; }
const char* __http_req_query(void* req, const char* name) { (void)req; (void)name; return ""; }
const char* __http_req_param(void* req, const char* name) { (void)req; (void)name; return ""; }
const char* __http_req_path_param(void* req, const char* name) { (void)req; (void)name; return ""; }
const char* __http_req_cookie(void* req, const char* name) { (void)req; (void)name; return ""; }
void* __http_req_json(void* req) { (void)req; return NULL; }
const char* __http_req_form_field(void* req, const char* name) { (void)req; (void)name; return ""; }
const char* __http_req_form_file(void* req, const char* name) { (void)req; (void)name; return ""; }
const char* __http_req_form_filename(void* req, const char* name) { (void)req; (void)name; return ""; }
int32_t __http_req_form_file_size(void* req, const char* name) { (void)req; (void)name; return 0; }

/* Response builders */
void* __http_resp_new(int status, const char* body, const char* content_type) {
    (void)status; (void)body; (void)content_type;
    http_server_unsupported("http.response");
    return NULL;
}
void* __http_resp_from_file(const char* path, const char* content_type) {
    (void)path; (void)content_type;
    http_server_unsupported("http.file_response");
    return NULL;
}
int __http_resp_status(void* resp) { (void)resp; return 0; }
const char* __http_resp_body(void* resp) { (void)resp; return ""; }
const char* __http_resp_content_type(void* resp) { (void)resp; return ""; }
void __http_resp_header(void* resp, const char* name, const char* value) {
    (void)resp; (void)name; (void)value;
}
void __http_resp_cookie(void* resp, const char* name, const char* value, const char* opts) {
    (void)resp; (void)name; (void)value; (void)opts;
}

/* Server-sent events */
int __http_sse_start(void* req) { (void)req; http_server_unsupported("http.sse_start"); return 0; }
void __http_sse_send_data(void* req, const char* data) { (void)req; (void)data; }
void __http_sse_send(void* req, const char* event, const char* data) {
    (void)req; (void)event; (void)data;
}
void __http_sse_close(void* req) { (void)req; }
void* __http_sse_response(void* req) { (void)req; return NULL; }

/* WebSocket server (websocket.c is POSIX-only too; wrapped by the same
 * http module, so client-only programs still link these) */
void __ws_set_path(void* s, const char* path) { (void)s; (void)path; http_server_unsupported("http.ws"); }
void __ws_set_on_message(void* s, void* handler) { (void)s; (void)handler; }
void __ws_set_on_open(void* s, void* handler) { (void)s; (void)handler; }
void __ws_set_on_close(void* s, void* handler) { (void)s; (void)handler; }
void __ws_route(void* s, const char* path, void* handler) { (void)s; (void)path; (void)handler; }
void __ws_route_on_open(void* s, const char* path, void* handler) { (void)s; (void)path; (void)handler; }
void __ws_route_on_close(void* s, const char* path, void* handler) { (void)s; (void)path; (void)handler; }
void __ws_route_on_binary(void* s, const char* path, void* handler) { (void)s; (void)path; (void)handler; }
void __ws_send(void* conn, const char* msg) { (void)conn; (void)msg; }
void __ws_send_binary(void* conn, const void* data, int64_t len) { (void)conn; (void)data; (void)len; }
void __ws_broadcast(void* s, const char* msg) { (void)s; (void)msg; }
void __ws_close(void* conn) { (void)conn; }
void __ws_join(void* conn, const char* room) { (void)conn; (void)room; }
void __ws_leave(void* conn, const char* room) { (void)conn; (void)room; }
void __ws_to_room(void* s, const char* room, const char* msg) { (void)s; (void)room; (void)msg; }
void __ws_set_max_message_size(void* s, int64_t bytes) { (void)s; (void)bytes; }
void __ws_set_ping_interval(void* s, int seconds) { (void)s; (void)seconds; }
void __ws_set_compression(void* s, int enabled) { (void)s; (void)enabled; }
int32_t __ws_conn_count(void* s) { (void)s; return 0; }

#endif /* _WIN32 */

/* Keep the translation unit non-empty on platforms where the real
 * http_server.c provides these symbols. */
typedef int desi_http_server_win_stub_unused_t;
