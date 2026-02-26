/*
 * http_server.c — HTTP server runtime for Desi
 *
 * Phase 3A: TCP listener + accept loop
 * Phase 3B: Request parsing + response builder (JSON, HTML, text, custom)
 * Phase 3C: Route table + handler dispatch + graceful shutdown
 *
 * Public API (called from Desi via extern):
 *   Server lifecycle:
 *     __http_server_new(port)              → create server, bind, listen
 *     __http_server_route(srv,method,path,handler) → register route
 *     __http_server_run(server)            → blocking accept loop
 *
 *   Response builders:
 *     __http_resp_new(status, body, content_type) → HttpServerResponse*
 *     __http_resp_status(resp)             → int
 *     __http_resp_body(resp)               → const char*
 *     __http_resp_content_type(resp)       → const char*
 *
 *   Request accessors:
 *     __http_req_method(req)               → const char*
 *     __http_req_path(req)                 → const char*
 *     __http_req_body(req)                 → const char*
 *     __http_req_header(req, name)         → const char*
 *     __http_req_query(req)                → const char*
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <signal.h>

#ifdef _WIN32
  #include <winsock2.h>
  #include <ws2tcpip.h>
  typedef SOCKET server_socket_t;
  #define CLOSE_SOCKET(s) closesocket(s)
  #define INVALID_SOCK INVALID_SOCKET
  static void ensure_wsa_server(void) {
      static int init = 0;
      if (!init) { WSADATA wsa; WSAStartup(MAKEWORD(2,2), &wsa); init = 1; }
  }
#else
  #include <sys/socket.h>
  #include <netinet/in.h>
  #include <arpa/inet.h>
  typedef int server_socket_t;
  #define CLOSE_SOCKET(s) close(s)
  #define INVALID_SOCK (-1)
  #define ensure_wsa_server() ((void)0)
#endif

/* ============================================================
 * Graceful Shutdown
 * ============================================================ */

static volatile sig_atomic_t server_running = 1;

static void shutdown_handler(int sig) {
    (void)sig;
    server_running = 0;
    printf("\n\033[33mShutting down server...\033[0m\n");
    fflush(stdout);
}

static void install_signal_handlers(void) {
    struct sigaction sa;
    sa.sa_handler = shutdown_handler;
    sa.sa_flags = 0;
    sigemptyset(&sa.sa_mask);
    sigaction(SIGINT, &sa, NULL);
    sigaction(SIGTERM, &sa, NULL);
}

/* ============================================================
 * Data Structures
 * ============================================================ */

/* ---- Server Request (parsed from raw HTTP) ---- */

typedef struct {
    char method[16];        /* GET, POST, PUT, etc. */
    char path[1024];        /* /api/users */
    char query[1024];       /* key=val&key2=val2 (after ?) */
    char* headers;          /* raw headers string */
    char* body;             /* request body (POST/PUT) */
    int  body_len;
} HttpServerRequest;

/* ---- Server Response ---- */

typedef struct {
    int   status;
    char* body;
    char* content_type;
    char* extra_headers;    /* additional headers (optional) */
} HttpServerResponse;

/* ---- Handler callback typedef ---- */
/* Desi handler: takes request ptr, returns response ptr */
typedef HttpServerResponse* (*route_handler_fn)(HttpServerRequest* req);

/* Global Desi dispatch function — set via __http_server_set_handler before listen() */
static route_handler_fn __desi_http_handler = NULL;

void __http_server_set_handler(route_handler_fn fn) {
    __desi_http_handler = fn;
}

/* ---- Route Entry ---- */

typedef struct {
    char method[16];
    char path[1024];
    route_handler_fn handler;
} Route;

/* ---- Server (dynamic route table, no fixed limit) ---- */

typedef struct {
    server_socket_t fd;
    int port;
    Route* routes;      /* dynamic array */
    int route_count;
    int route_cap;      /* current capacity */
} HttpServer;

/* ============================================================
 * Request Parsing
 * ============================================================ */

static HttpServerRequest* parse_request(const char* raw, int raw_len) {
    HttpServerRequest* req = (HttpServerRequest*)calloc(1, sizeof(HttpServerRequest));
    if (!req) return NULL;

    /* Parse request line: "METHOD /path?query HTTP/1.1\r\n" */
    const char* line_end = strstr(raw, "\r\n");
    if (!line_end) {
        free(req);
        return NULL;
    }

    /* Extract method */
    const char* p = raw;
    int i = 0;
    while (p < line_end && *p != ' ' && i < 15) {
        req->method[i++] = *p++;
    }
    req->method[i] = '\0';

    /* Skip space */
    if (*p == ' ') p++;

    /* Extract path (and query string if present) */
    i = 0;
    while (p < line_end && *p != ' ' && *p != '?' && i < 1023) {
        req->path[i++] = *p++;
    }
    req->path[i] = '\0';

    /* Extract query string after ? */
    if (*p == '?') {
        p++; /* skip ? */
        i = 0;
        while (p < line_end && *p != ' ' && i < 1023) {
            req->query[i++] = *p++;
        }
        req->query[i] = '\0';
    }

    /* Find headers (after first \r\n) */
    const char* headers_start = line_end + 2;
    const char* headers_end = strstr(headers_start, "\r\n\r\n");
    if (headers_end) {
        int hdr_len = (int)(headers_end - headers_start);
        req->headers = (char*)malloc(hdr_len + 1);
        if (req->headers) {
            memcpy(req->headers, headers_start, hdr_len);
            req->headers[hdr_len] = '\0';
        }

        /* Body starts after \r\n\r\n */
        const char* body_start = headers_end + 4;
        int body_len = raw_len - (int)(body_start - raw);
        if (body_len > 0) {
            req->body = (char*)malloc(body_len + 1);
            if (req->body) {
                memcpy(req->body, body_start, body_len);
                req->body[body_len] = '\0';
                req->body_len = body_len;
            }
        }
    } else {
        req->headers = strdup("");
        req->body = strdup("");
    }

    return req;
}

static void free_request(HttpServerRequest* req) {
    if (!req) return;
    free(req->headers);
    free(req->body);
    free(req);
}

/* Find a header value by name (case-insensitive) */
static const char* find_header(const char* headers, const char* name) {
    if (!headers || !name) return "";
    static char value_buf[1024];

    int name_len = strlen(name);
    const char* p = headers;
    while (*p) {
        /* Case-insensitive match */
        if (strncasecmp(p, name, name_len) == 0 && p[name_len] == ':') {
            p += name_len + 1;
            /* Skip whitespace */
            while (*p == ' ') p++;
            /* Copy until \r\n or end */
            int i = 0;
            while (*p && *p != '\r' && *p != '\n' && i < 1023) {
                value_buf[i++] = *p++;
            }
            value_buf[i] = '\0';
            return value_buf;
        }
        /* Skip to next line */
        while (*p && *p != '\n') p++;
        if (*p == '\n') p++;
    }
    return "";
}

/* ============================================================
 * Request Accessors (Desi extern)
 * ============================================================ */

const char* __http_req_method(HttpServerRequest* req) {
    return req ? req->method : "";
}

const char* __http_req_path(HttpServerRequest* req) {
    return req ? req->path : "";
}

const char* __http_req_body(HttpServerRequest* req) {
    return req ? (req->body ? req->body : "") : "";
}

const char* __http_req_header(HttpServerRequest* req, const char* name) {
    if (!req || !req->headers) return "";
    return find_header(req->headers, name);
}

const char* __http_req_query(HttpServerRequest* req) {
    return req ? req->query : "";
}

/* ============================================================
 * Response Builders (Desi extern)
 * ============================================================ */

HttpServerResponse* __http_resp_new(int status, const char* body, const char* content_type) {
    HttpServerResponse* resp = (HttpServerResponse*)calloc(1, sizeof(HttpServerResponse));
    if (!resp) return NULL;
    resp->status = status;
    resp->body = body ? strdup(body) : strdup("");
    resp->content_type = content_type ? strdup(content_type) : strdup("text/plain");
    resp->extra_headers = NULL;
    return resp;
}

int __http_resp_status(HttpServerResponse* resp) {
    return resp ? resp->status : 0;
}

const char* __http_resp_body(HttpServerResponse* resp) {
    return resp ? resp->body : "";
}

const char* __http_resp_content_type(HttpServerResponse* resp) {
    return resp ? resp->content_type : "";
}

static void free_response(HttpServerResponse* resp) {
    if (!resp) return;
    free(resp->body);
    free(resp->content_type);
    free(resp->extra_headers);
    free(resp);
}

/* ============================================================
 * HTTP Status Text
 * ============================================================ */

static const char* status_text(int status) {
    switch (status) {
        /* 1xx Informational */
        case 100: return "Continue";
        case 101: return "Switching Protocols";
        case 102: return "Processing";
        case 103: return "Early Hints";
        /* 2xx Success */
        case 200: return "OK";
        case 201: return "Created";
        case 202: return "Accepted";
        case 203: return "Non-Authoritative Information";
        case 204: return "No Content";
        case 205: return "Reset Content";
        case 206: return "Partial Content";
        case 207: return "Multi-Status";
        case 208: return "Already Reported";
        case 226: return "IM Used";
        /* 3xx Redirection */
        case 300: return "Multiple Choices";
        case 301: return "Moved Permanently";
        case 302: return "Found";
        case 303: return "See Other";
        case 304: return "Not Modified";
        case 305: return "Use Proxy";
        case 307: return "Temporary Redirect";
        case 308: return "Permanent Redirect";
        /* 4xx Client Error */
        case 400: return "Bad Request";
        case 401: return "Unauthorized";
        case 402: return "Payment Required";
        case 403: return "Forbidden";
        case 404: return "Not Found";
        case 405: return "Method Not Allowed";
        case 406: return "Not Acceptable";
        case 407: return "Proxy Authentication Required";
        case 408: return "Request Timeout";
        case 409: return "Conflict";
        case 410: return "Gone";
        case 411: return "Length Required";
        case 412: return "Precondition Failed";
        case 413: return "Content Too Large";
        case 414: return "URI Too Long";
        case 415: return "Unsupported Media Type";
        case 416: return "Range Not Satisfiable";
        case 417: return "Expectation Failed";
        case 418: return "I'm a Teapot";
        case 421: return "Misdirected Request";
        case 422: return "Unprocessable Content";
        case 423: return "Locked";
        case 424: return "Failed Dependency";
        case 425: return "Too Early";
        case 426: return "Upgrade Required";
        case 428: return "Precondition Required";
        case 429: return "Too Many Requests";
        case 431: return "Request Header Fields Too Large";
        case 451: return "Unavailable For Legal Reasons";
        /* 5xx Server Error */
        case 500: return "Internal Server Error";
        case 501: return "Not Implemented";
        case 502: return "Bad Gateway";
        case 503: return "Service Unavailable";
        case 504: return "Gateway Timeout";
        case 505: return "HTTP Version Not Supported";
        case 506: return "Variant Also Negotiates";
        case 507: return "Insufficient Storage";
        case 508: return "Loop Detected";
        case 510: return "Not Extended";
        case 511: return "Network Authentication Required";
        default:  return "Unknown";
    }
}

/* ============================================================
 * Send Response on Wire
 * ============================================================ */

static void send_response(server_socket_t client_fd, HttpServerResponse* resp) {
    if (!resp) {
        /* Default 500 response */
        const char* err = "HTTP/1.1 500 Internal Server Error\r\n"
                          "Content-Length: 21\r\nConnection: close\r\n"
                          "Server: Desi/0.1\r\n\r\n"
                          "Internal Server Error";
        send(client_fd, err, strlen(err), 0);
        return;
    }

    int body_len = resp->body ? (int)strlen(resp->body) : 0;

    /* Build response header */
    char header[4096];
    int hdr_len = snprintf(header, sizeof(header),
        "HTTP/1.1 %d %s\r\n"
        "Content-Type: %s\r\n"
        "Content-Length: %d\r\n"
        "Connection: close\r\n"
        "Server: Desi/0.1\r\n"
        "Access-Control-Allow-Origin: *\r\n"
        "%s"
        "\r\n",
        resp->status, status_text(resp->status),
        resp->content_type ? resp->content_type : "text/plain",
        body_len,
        resp->extra_headers ? resp->extra_headers : "");

    /* Send header + body */
    send(client_fd, header, hdr_len, 0);
    if (body_len > 0) {
        send(client_fd, resp->body, body_len, 0);
    }
}

/* ============================================================
 * Route Registration (Desi extern)
 * ============================================================ */

void __http_server_route(HttpServer* srv, const char* method,
                         const char* path, route_handler_fn handler) {
    if (!srv) {
        fprintf(stderr, "http_server: server is NULL\n");
        return;
    }
    /* Grow route table if needed */
    if (srv->route_count >= srv->route_cap) {
        int new_cap = srv->route_cap == 0 ? 16 : srv->route_cap * 2;
        Route* new_routes = (Route*)realloc(srv->routes, new_cap * sizeof(Route));
        if (!new_routes) {
            fprintf(stderr, "http_server: failed to grow route table\n");
            return;
        }
        srv->routes = new_routes;
        srv->route_cap = new_cap;
    }
    Route* r = &srv->routes[srv->route_count++];
    strncpy(r->method, method, sizeof(r->method) - 1);
    strncpy(r->path, path, sizeof(r->path) - 1);
    r->handler = handler;
}

/* ============================================================
 * Route Matching
 * ============================================================ */

static Route* match_route(HttpServer* srv, const char* method, const char* path) {
    if (!srv) return NULL;

    /* Exact match first */
    for (int i = 0; i < srv->route_count; i++) {
        Route* r = &srv->routes[i];
        /* Match method (case-insensitive) and exact path */
        if (strcasecmp(r->method, method) == 0 && strcmp(r->path, path) == 0) {
            return r;
        }
    }

    /* Wildcard: "*" method matches any method */
    for (int i = 0; i < srv->route_count; i++) {
        Route* r = &srv->routes[i];
        if (strcmp(r->method, "*") == 0 && strcmp(r->path, path) == 0) {
            return r;
        }
    }

    return NULL;
}

/* ============================================================
 * Default Responses
 * ============================================================ */

static HttpServerResponse default_404 = {
    .status = 404,
    .body = "{\"error\":\"not found\"}",
    .content_type = "application/json",
    .extra_headers = NULL
};

static HttpServerResponse default_405 = {
    .status = 405,
    .body = "{\"error\":\"method not allowed\"}",
    .content_type = "application/json",
    .extra_headers = NULL
};

/* Check if path exists but method doesn't match */
static int path_exists(HttpServer* srv, const char* path) {
    for (int i = 0; i < srv->route_count; i++) {
        if (strcmp(srv->routes[i].path, path) == 0) return 1;
    }
    return 0;
}

/* ============================================================
 * Server: Create + Accept Loop
 * ============================================================ */

HttpServer* __http_server_new(int port) {
    ensure_wsa_server();

    HttpServer* srv = (HttpServer*)calloc(1, sizeof(HttpServer));
    if (!srv) {
        fprintf(stderr, "http_server: malloc failed\n");
        return NULL;
    }
    srv->port = port;
    srv->routes = NULL;
    srv->route_count = 0;
    srv->route_cap = 0;

    /* Create socket */
    srv->fd = socket(AF_INET, SOCK_STREAM, 0);
    if (srv->fd == INVALID_SOCK) {
        fprintf(stderr, "http_server: socket() failed: %s\n", strerror(errno));
        free(srv);
        return NULL;
    }

    /* Allow address reuse (avoids "address already in use" on restart) */
    int opt = 1;
    setsockopt(srv->fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    /* Bind */
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;  /* 0.0.0.0 */
    addr.sin_port = htons((uint16_t)port);

    if (bind(srv->fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        fprintf(stderr, "http_server: bind() failed on port %d: %s\n", port, strerror(errno));
        CLOSE_SOCKET(srv->fd);
        free(srv);
        return NULL;
    }

    /* Listen with backlog of 128 */
    if (listen(srv->fd, 128) < 0) {
        fprintf(stderr, "http_server: listen() failed: %s\n", strerror(errno));
        CLOSE_SOCKET(srv->fd);
        free(srv);
        return NULL;
    }

    return srv;
}

/* ---- Handle a single client connection ---- */

static void handle_client(HttpServer* srv, server_socket_t client_fd) {
    /* Read request (up to 8KB) */
    char buf[8192];
    ssize_t nread = recv(client_fd, buf, sizeof(buf) - 1, 0);
    if (nread <= 0) {
        CLOSE_SOCKET(client_fd);
        return;
    }
    buf[nread] = '\0';

    /* Parse request */
    HttpServerRequest* req = parse_request(buf, (int)nread);
    if (!req) {
        const char* bad = "HTTP/1.1 400 Bad Request\r\n"
                          "Content-Length: 11\r\nConnection: close\r\n\r\n"
                          "Bad Request";
        send(client_fd, bad, strlen(bad), 0);
        CLOSE_SOCKET(client_fd);
        return;
    }

    HttpServerResponse* resp = NULL;
    int resp_owned = 0;   /* whether we need to free resp */

    /* Dispatch to Desi handler if registered */
    if (__desi_http_handler) {
        resp = __desi_http_handler(req);
        resp_owned = 1;
    } else if (srv->route_count > 0) {
        /* C-level route dispatch */
        Route* route = match_route(srv, req->method, req->path);
        if (route && route->handler) {
            resp = route->handler(req);
            resp_owned = 1;
        } else if (path_exists(srv, req->path)) {
            resp = &default_405;
        } else {
            resp = &default_404;
        }
    } else {
        /* No routes registered — echo mode (Phase 3A fallback) */
        char body[2048];
        snprintf(body, sizeof(body),
            "{\"status\":\"ok\",\"server\":\"desi\","
            "\"request\":{\"method\":\"%s\",\"path\":\"%s\",\"query\":\"%s\"}}",
            req->method, req->path, req->query);

        resp = __http_resp_new(200, body, "application/json");
        resp_owned = 1;
    }

    send_response(client_fd, resp);
    CLOSE_SOCKET(client_fd);

    /* Log */
    int status = resp ? resp->status : 500;
    printf("%s %s → %d\n", req->method, req->path, status);
    fflush(stdout);

    if (resp_owned) free_response(resp);
    free_request(req);
}

/* ---- Blocking accept loop with graceful shutdown ---- */

void __http_server_run(HttpServer* srv) {
    if (!srv) {
        fprintf(stderr, "http_server: server is NULL\n");
        return;
    }

    install_signal_handlers();

    printf("\033[32m✓ Desi HTTP Server listening on http://0.0.0.0:%d\033[0m\n", srv->port);
    if (srv->route_count > 0) {
        printf("  Routes:\n");
        for (int i = 0; i < srv->route_count; i++) {
            printf("    %s %s\n", srv->routes[i].method, srv->routes[i].path);
        }
    } else {
        printf("  (no routes — echo mode)\n");
    }
    printf("  Press Ctrl+C to stop\n\n");
    fflush(stdout);

    while (server_running) {
        struct sockaddr_in client_addr;
        socklen_t client_len = sizeof(client_addr);

        server_socket_t client_fd = accept(srv->fd,
            (struct sockaddr*)&client_addr, &client_len);

        if (client_fd == INVALID_SOCK) {
            if (!server_running) break;  /* shutdown signal */
            fprintf(stderr, "http_server: accept() failed: %s\n", strerror(errno));
            continue;
        }

        handle_client(srv, client_fd);
    }

    /* Cleanup */
    CLOSE_SOCKET(srv->fd);
    printf("\033[32m✓ Server stopped\033[0m\n");
    fflush(stdout);
}
