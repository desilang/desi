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
#include "supervisor.h"
#include "websocket.h"
#include "tls.h"
#include <netinet/tcp.h>  /* TCP_NODELAY */

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

/* Gzip compression support */
#include "http/gzip.h"

/* Minimum body size for gzip compression (1KB) */
#define GZIP_MIN_SIZE 1024

/* ============================================================
 * Graceful Shutdown
 * ============================================================ */

static volatile sig_atomic_t server_running = 1;
static volatile int active_connections = 0;  /* active in-flight requests */

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
    /* Path parameters from pattern matching (e.g. /users/:id) */
    char param_names[8][64];   /* up to 8 path params */
    char param_values[8][256];
    int  param_count;
    /* Connection info for SSE (set by handle_client before dispatch) */
    server_socket_t _fd;    /* raw socket fd */
    void*           _ssl;   /* DESI_SSL* or NULL */
} HttpServerRequest;

/* ---- Server Response ---- */

typedef struct {
    int   status;
    char* body;
    char* content_type;
    char* extra_headers;    /* additional headers (optional) */
    char* file_path;        /* if set, stream from disk instead of body */
    long  file_size;        /* file size for Content-Length (when streaming) */
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

/* ---- Middleware callback ---- */
typedef HttpServerResponse* (*middleware_fn)(HttpServerRequest* req);

/* ---- Rate limiter bucket (token bucket per IP) ---- */
typedef struct {
    uint32_t ip;            /* IPv4 address */
    int      tokens;        /* remaining tokens */
    time_t   last_refill;   /* last refill timestamp */
} RateBucket;

#define MAX_RATE_BUCKETS 1024
#define RATE_DEFAULT_MAX   60  /* requests per window */
#define RATE_DEFAULT_WINDOW 60 /* seconds */

/* ---- Server (dynamic route table, no fixed limit) ---- */

typedef struct {
    server_socket_t fd;
    int port;
    Route* routes;      /* dynamic array */
    int route_count;
    int route_cap;      /* current capacity */
    char static_prefix[256]; /* URL prefix for static files */
    char static_dir[1024];   /* filesystem directory */
    int  max_body_size;      /* max request body in bytes (0 = default 1MB) */
    /* Middleware chain */
    middleware_fn middlewares[16];  /* up to 16 middleware functions */
    int middleware_count;
    /* Rate limiter */
    RateBucket rate_buckets[MAX_RATE_BUCKETS];
    int rate_max;          /* max tokens (0 = disabled) */
    int rate_window;       /* refill window in seconds */
    /* CORS */
    char* cors_origin;     /* allowed origin (NULL = no CORS headers) */
    char* cors_methods;    /* allowed methods */
    char* cors_headers;    /* allowed headers */
    /* TLS */
    DESI_SSL_CTX* ssl_ctx; /* non-NULL = HTTPS mode */
    /* Request timeout */
    int request_timeout;    /* per-request timeout in seconds (0 = default 30s) */
} HttpServer;

static void free_server(HttpServer* srv) {
    if (!srv) return;
    if (srv->ssl_ctx) desi_tls_ctx_free(srv->ssl_ctx);
    if (srv->fd != INVALID_SOCK) CLOSE_SOCKET(srv->fd);
    free(srv->routes);
    free(srv->cors_origin);
    free(srv->cors_methods);
    free(srv->cors_headers);
    free(srv);
}

/* ============================================================
 * Request Parsing
 * ============================================================ */

/* Extract Content-Length from raw headers (returns -1 if not found) */
static int parse_content_length(const char* headers) {
    if (!headers) return -1;
    const char* cl = strcasestr(headers, "Content-Length:");
    if (!cl) return -1;
    cl += 15; /* skip "Content-Length:" */
    while (*cl == ' ') cl++;
    return atoi(cl);
}

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

        /* Validate against Content-Length if present */
        int expected_len = parse_content_length(req->headers);
        if (expected_len >= 0 && expected_len < body_len) {
            body_len = expected_len; /* trust Content-Length over raw data */
        }

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

static void free_form_data(void); /* forward decl */

static void free_request(HttpServerRequest* req) {
    if (!req) return;
    free_form_data(); /* clean up any parsed multipart data */
    free(req->headers);
    free(req->body);
    free(req);
}

/* Find a header value by name (case-insensitive) */
/* Thread-safe: writes into caller-provided buffer */
static const char* find_header_safe(const char* headers, const char* name,
                                     char* out_buf, int out_buf_size) {
    if (!headers || !name) return "";

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
            while (*p && *p != '\r' && *p != '\n' && i < out_buf_size - 1) {
                out_buf[i++] = *p++;
            }
            out_buf[i] = '\0';
            return out_buf;
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
    /* Thread-safe: use stack-allocated buffer, return strdup */
    char buf[1024];
    const char* val = find_header_safe(req->headers, name, buf, sizeof(buf));
    if (val == buf) return strdup(buf);  /* caller must free, but Desi GC handles it */
    return "";
}

const char* __http_req_query(HttpServerRequest* req) {
    return req ? req->query : "";
}

/* ---- Query parameter extraction: ?key=val&key2=val2 → value for key ---- */

const char* __http_req_param(HttpServerRequest* req, const char* key) {
    if (!req || !key || req->query[0] == '\0') return "";
    int key_len = (int)strlen(key);
    const char* p = req->query;
    while (*p) {
        /* Match key */
        if (strncmp(p, key, key_len) == 0 && p[key_len] == '=') {
            p += key_len + 1;  /* skip key= */
            /* Return value (static buffer, caller copies in Desi) */
            static __thread char param_buf[1024];
            int i = 0;
            while (*p && *p != '&' && i < 1023) {
                param_buf[i++] = *p++;
            }
            param_buf[i] = '\0';
            return param_buf;
        }
        /* Skip to next & */
        while (*p && *p != '&') p++;
        if (*p == '&') p++;
    }
    return "";
}

/* ---- Path parameter accessor: /users/:id → value for "id" ---- */

const char* __http_req_path_param(HttpServerRequest* req, const char* name) {
    if (!req || !name) return "";
    for (int i = 0; i < req->param_count; i++) {
        if (strcmp(req->param_names[i], name) == 0) {
            return req->param_values[i];
        }
    }
    return "";
}

/* ---- Parse request body as JSON ---- */
/* Forward declare __json_parse from json.c */
extern void* __json_parse(const char* text);

void* __http_req_json(HttpServerRequest* req) {
    if (!req || !req->body || req->body_len == 0) return NULL;
    return __json_parse(req->body);
}

/* ---- Path pattern matching with :param capture ---- */
/* Pattern: /users/:id/posts/:post_id
 * Path:    /users/42/posts/hello
 * Extracts: id=42, post_id=hello */
static int match_path_pattern(const char* pattern, const char* path,
                               HttpServerRequest* req) {
    req->param_count = 0;
    const char* pp = pattern;
    const char* rp = path;

    while (*pp && *rp) {
        if (*pp == ':') {
            /* Extract param name */
            pp++;  /* skip : */
            char name[64];
            int ni = 0;
            while (*pp && *pp != '/' && ni < 63) {
                name[ni++] = *pp++;
            }
            name[ni] = '\0';

            /* Extract param value */
            char val[256];
            int vi = 0;
            while (*rp && *rp != '/' && vi < 255) {
                val[vi++] = *rp++;
            }
            val[vi] = '\0';

            if (req->param_count < 8) {
                strncpy(req->param_names[req->param_count], name, 63);
                strncpy(req->param_values[req->param_count], val, 255);
                req->param_count++;
            }
        } else {
            if (*pp != *rp) return 0;  /* mismatch */
            pp++;
            rp++;
        }
    }
    /* Both must be consumed (or pattern ends with param at end) */
    return (*pp == '\0' && *rp == '\0');
}

/* ============================================================
 * Rate Limiter (token bucket per IP)
 * ============================================================ */

static int rate_limit_check(HttpServer* srv, uint32_t client_ip) {
    if (srv->rate_max <= 0) return 1;  /* disabled */

    time_t now = time(NULL);
    RateBucket* bucket = NULL;
    RateBucket* oldest = &srv->rate_buckets[0];

    /* Find existing bucket or oldest for eviction */
    for (int i = 0; i < MAX_RATE_BUCKETS; i++) {
        RateBucket* b = &srv->rate_buckets[i];
        if (b->ip == client_ip && b->last_refill > 0) {
            bucket = b;
            break;
        }
        if (b->last_refill < oldest->last_refill) {
            oldest = b;
        }
    }

    if (!bucket) {
        /* New IP — use oldest bucket (eviction) */
        bucket = oldest;
        bucket->ip = client_ip;
        bucket->tokens = srv->rate_max;
        bucket->last_refill = now;
    }

    /* Refill tokens based on elapsed time */
    int elapsed = (int)(now - bucket->last_refill);
    if (elapsed > 0) {
        int refill = (elapsed * srv->rate_max) / srv->rate_window;
        bucket->tokens += refill;
        if (bucket->tokens > srv->rate_max) bucket->tokens = srv->rate_max;
        bucket->last_refill = now;
    }

    /* Consume a token */
    if (bucket->tokens > 0) {
        bucket->tokens--;
        return 1;  /* allowed */
    }
    return 0;  /* rate limited */
}

/* ============================================================
 * Server Configuration (Desi extern)
 * ============================================================ */

/* Set maximum request body size (default 1MB if not set) */
void __http_server_max_body(HttpServer* srv, int max_bytes) {
    if (srv) srv->max_body_size = max_bytes;
}

/* Set per-request timeout in seconds (default 15s if not set) */
void __http_server_timeout(HttpServer* srv, int timeout_secs) {
    if (srv && timeout_secs > 0) srv->request_timeout = timeout_secs;
}

/* Register a middleware function */
void __http_server_use(HttpServer* srv, middleware_fn fn) {
    if (srv && srv->middleware_count < 16) {
        srv->middlewares[srv->middleware_count++] = fn;
    }
}

/* Enable rate limiting: max requests per window (seconds) */
void __http_server_rate_limit(HttpServer* srv, int max_requests, int window_secs) {
    if (!srv) return;
    srv->rate_max = max_requests;
    srv->rate_window = window_secs > 0 ? window_secs : RATE_DEFAULT_WINDOW;
    memset(srv->rate_buckets, 0, sizeof(srv->rate_buckets));
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

HttpServerResponse* __http_resp_from_file(int status, const char* path, const char* content_type) {
    if (!path) return __http_resp_new(500, "http.html: no file path provided", "text/plain");
    FILE* f = fopen(path, "rb");
    if (!f) {
        char errbuf[256];
        snprintf(errbuf, sizeof(errbuf), "http.html: cannot open file '%s'", path);
        fprintf(stderr, "%s\n", errbuf);
        return __http_resp_new(500, errbuf, "text/plain");
    }
    fseek(f, 0, SEEK_END);
    long len = ftell(f);
    fseek(f, 0, SEEK_SET);

    HttpServerResponse* resp = (HttpServerResponse*)calloc(1, sizeof(HttpServerResponse));
    if (!resp) { fclose(f); return NULL; }
    resp->status = status;
    resp->content_type = content_type ? strdup(content_type) : strdup("text/html; charset=utf-8");
    resp->extra_headers = NULL;

    /* For large files (>64KB), use streaming mode: store path, don't load into memory */
    if (len > 65536) {
        fclose(f);
        resp->body = NULL;
        resp->file_path = strdup(path);
        resp->file_size = len;
    } else {
        /* Small files: load into memory (fast, allows gzip) */
        char* buf = (char*)malloc(len + 1);
        if (!buf) { fclose(f); free(resp); return __http_resp_new(500, "http.html: out of memory", "text/plain"); }
        size_t nread = fread(buf, 1, len, f);
        fclose(f);
        buf[nread] = '\0';
        resp->body = buf;
        resp->file_path = NULL;
        resp->file_size = 0;
    }
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

/* ---- Custom Response Headers ---- */

void __http_resp_header(HttpServerResponse* resp, const char* key, const char* value) {
    if (!resp || !key || !value) return;
    /* Build "Key: Value\r\n" */
    size_t klen = strlen(key);
    size_t vlen = strlen(value);
    size_t hdr_len = klen + 2 + vlen + 2; /* key: value\r\n */
    if (resp->extra_headers) {
        size_t old_len = strlen(resp->extra_headers);
        resp->extra_headers = (char*)realloc(resp->extra_headers, old_len + hdr_len + 1);
        sprintf(resp->extra_headers + old_len, "%s: %s\r\n", key, value);
    } else {
        resp->extra_headers = (char*)malloc(hdr_len + 1);
        sprintf(resp->extra_headers, "%s: %s\r\n", key, value);
    }
}

/* ---- CORS Configuration ---- */

void __http_server_cors(HttpServer* srv, const char* origin) {
    if (!srv || !origin) return;
    free(srv->cors_origin);
    free(srv->cors_methods);
    free(srv->cors_headers);
    srv->cors_origin = strdup(origin);
    srv->cors_methods = strdup("GET, POST, PUT, DELETE, PATCH, OPTIONS");
    srv->cors_headers = strdup("Content-Type, Authorization, X-Requested-With");
}

/* ---- Cookie Helpers ---- */

/* Parse Cookie header: "name1=val1; name2=val2" → value for name */
const char* __http_req_cookie(HttpServerRequest* req, const char* name) {
    static __thread char cookie_val[512];
    if (!req || !name) return "";
    const char* cookies = find_header_safe(req->headers, "Cookie",
                                           cookie_val, sizeof(cookie_val));
    if (!cookies || !cookies[0]) return "";
    /* Search for name= in cookie string */
    size_t nlen = strlen(name);
    const char* p = cookies;
    while (*p) {
        /* skip whitespace */
        while (*p == ' ') p++;
        if (strncmp(p, name, nlen) == 0 && p[nlen] == '=') {
            /* Found it — extract value until ; or end */
            const char* val_start = p + nlen + 1;
            const char* val_end = val_start;
            while (*val_end && *val_end != ';') val_end++;
            size_t vlen = val_end - val_start;
            if (vlen >= sizeof(cookie_val)) vlen = sizeof(cookie_val) - 1;
            memcpy(cookie_val, val_start, vlen);
            cookie_val[vlen] = '\0';
            return cookie_val;
        }
        /* Skip to next cookie */
        while (*p && *p != ';') p++;
        if (*p == ';') p++;
    }
    return "";
}

/* Set-Cookie: name=value; Max-Age=N; Path=/; HttpOnly; SameSite=Lax */
void __http_resp_cookie(HttpServerResponse* resp, const char* name,
                        const char* value, int max_age) {
    if (!resp || !name || !value) return;
    char cookie_hdr[1024];
    if (max_age > 0) {
        snprintf(cookie_hdr, sizeof(cookie_hdr),
            "Set-Cookie: %s=%s; Max-Age=%d; Path=/; HttpOnly; SameSite=Lax\r\n",
            name, value, max_age);
    } else {
        snprintf(cookie_hdr, sizeof(cookie_hdr),
            "Set-Cookie: %s=%s; Path=/; HttpOnly; SameSite=Lax\r\n",
            name, value);
    }
    /* Append to extra_headers */
    if (resp->extra_headers) {
        size_t old_len = strlen(resp->extra_headers);
        size_t new_len = strlen(cookie_hdr);
        resp->extra_headers = (char*)realloc(resp->extra_headers, old_len + new_len + 1);
        memcpy(resp->extra_headers + old_len, cookie_hdr, new_len + 1);
    } else {
        resp->extra_headers = strdup(cookie_hdr);
    }
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

/* Keep-alive idle timeout in seconds */
#define KEEPALIVE_TIMEOUT_SECS 15
/* Max requests per keep-alive connection */
#define KEEPALIVE_MAX_REQUESTS 100

static void send_response_ka(server_socket_t client_fd, DESI_SSL* ssl, HttpServerResponse* resp, int keep_alive, int accept_gzip) {
    if (!resp) {
        /* Default 500 response */
        const char* err = "HTTP/1.1 500 Internal Server Error\r\n"
                          "Content-Length: 21\r\nConnection: close\r\n"
                          "Server: Desi/0.1\r\n\r\n"
                          "Internal Server Error";
        DESI_SEND(client_fd, ssl, err, strlen(err));
        return;
    }

    int body_len = resp->body ? (int)strlen(resp->body) : (resp->file_path ? (int)resp->file_size : 0);
    const char* conn_header = keep_alive ? "keep-alive" : "close";

    /* Try gzip compression for text bodies > GZIP_MIN_SIZE */
    unsigned char* gzip_body = NULL;
    size_t gzip_len = 0;
    int use_gzip = 0;

    if (accept_gzip && desi_gzip_available() && body_len > GZIP_MIN_SIZE && resp->body) {
        /* Only compress text-like content types */
        const char* ct = resp->content_type ? resp->content_type : "text/plain";
        if (strstr(ct, "text/") || strstr(ct, "json") || strstr(ct, "xml") ||
            strstr(ct, "javascript") || strstr(ct, "css")) {
            gzip_body = desi_gzip_compress((const unsigned char*)resp->body, body_len, &gzip_len);
            if (gzip_body && gzip_len < (size_t)body_len) {
                use_gzip = 1;
            } else {
                free(gzip_body);
                gzip_body = NULL;
            }
        }
    }

    /* Build response header */
    char header[4096];
    int hdr_len = snprintf(header, sizeof(header),
        "HTTP/1.1 %d %s\r\n"
        "Content-Type: %s\r\n"
        "Content-Length: %d\r\n"
        "%s"
        "Connection: %s\r\n"
        "Server: Desi/0.1\r\n"
        "%s"
        "\r\n",
        resp->status, status_text(resp->status),
        resp->content_type ? resp->content_type : "text/plain",
        use_gzip ? (int)gzip_len : body_len,
        use_gzip ? "Content-Encoding: gzip\r\nVary: Accept-Encoding\r\n" : "",
        conn_header,
        resp->extra_headers ? resp->extra_headers : "");

    /* Send header + body */
    DESI_SEND(client_fd, ssl, header, hdr_len);
    if (use_gzip) {
        DESI_SEND(client_fd, ssl, (const char*)gzip_body, (int)gzip_len);
        free(gzip_body);
    } else if (resp->file_path) {
        /* Stream from disk in 64KB chunks — avoids loading large files into memory */
        FILE* sf = fopen(resp->file_path, "rb");
        if (sf) {
            char chunk[65536];
            size_t n;
            while ((n = fread(chunk, 1, sizeof(chunk), sf)) > 0) {
                DESI_SEND(client_fd, ssl, chunk, (int)n);
            }
            fclose(sf);
        }
    } else if (body_len > 0) {
        DESI_SEND(client_fd, ssl, resp->body, body_len);
    }
}

/* Backward compat wrapper */
static void send_response(server_socket_t client_fd, HttpServerResponse* resp) {
    send_response_ka(client_fd, NULL, resp, 0, 0);
}

/* ============================================================
 * Determine keep-alive from request headers
 * HTTP/1.1 default is keep-alive; HTTP/1.0 default is close
 * ============================================================ */

static int should_keep_alive(HttpServerRequest* req) {
    if (!req || !req->headers) return 0;
    char buf[64];
    const char* conn = find_header_safe(req->headers, "Connection", buf, sizeof(buf));
    if (conn == buf) {
        /* Explicit Connection header */
        if (strcasecmp(buf, "close") == 0) return 0;
        if (strcasecmp(buf, "keep-alive") == 0) return 1;
    }
    /* HTTP/1.1 defaults to keep-alive */
    return 1;
}

/* ============================================================
 * Static File Serving
 * ============================================================ */

static const char* mime_for_ext(const char* path) {
    const char* dot = strrchr(path, '.');
    if (!dot) return "application/octet-stream";
    dot++;
    if (strcasecmp(dot, "html") == 0 || strcasecmp(dot, "htm") == 0)
        return "text/html; charset=utf-8";
    if (strcasecmp(dot, "css") == 0)  return "text/css; charset=utf-8";
    if (strcasecmp(dot, "js") == 0)   return "text/javascript; charset=utf-8";
    if (strcasecmp(dot, "json") == 0) return "application/json";
    if (strcasecmp(dot, "png") == 0)  return "image/png";
    if (strcasecmp(dot, "jpg") == 0 || strcasecmp(dot, "jpeg") == 0)
        return "image/jpeg";
    if (strcasecmp(dot, "gif") == 0)  return "image/gif";
    if (strcasecmp(dot, "svg") == 0)  return "image/svg+xml";
    if (strcasecmp(dot, "ico") == 0)  return "image/x-icon";
    if (strcasecmp(dot, "webp") == 0) return "image/webp";
    if (strcasecmp(dot, "woff") == 0) return "font/woff";
    if (strcasecmp(dot, "woff2") == 0) return "font/woff2";
    if (strcasecmp(dot, "ttf") == 0)  return "font/ttf";
    if (strcasecmp(dot, "otf") == 0)  return "font/otf";
    if (strcasecmp(dot, "txt") == 0)  return "text/plain; charset=utf-8";
    if (strcasecmp(dot, "xml") == 0)  return "text/xml; charset=utf-8";
    if (strcasecmp(dot, "pdf") == 0)  return "application/pdf";
    if (strcasecmp(dot, "zip") == 0)  return "application/zip";
    if (strcasecmp(dot, "mp4") == 0)  return "video/mp4";
    if (strcasecmp(dot, "webm") == 0) return "video/webm";
    if (strcasecmp(dot, "mp3") == 0)  return "audio/mpeg";
    if (strcasecmp(dot, "wasm") == 0) return "application/wasm";
    return "application/octet-stream";
}

void __http_server_static(HttpServer* srv, const char* prefix, const char* dir) {
    if (!srv) return;
    strncpy(srv->static_prefix, prefix, sizeof(srv->static_prefix) - 1);
    strncpy(srv->static_dir, dir, sizeof(srv->static_dir) - 1);
}

/* Try to serve a static file. Returns 1 if served, 0 if not a static path. */
static int try_serve_static(HttpServer* srv, server_socket_t client_fd,
                            DESI_SSL* ssl, HttpServerRequest* req, int keep_alive) {
    if (srv->static_prefix[0] == '\0') return 0;  /* no static dir configured */
    if (strcmp(req->method, "GET") != 0) return 0; /* only GET for static */

    int prefix_len = (int)strlen(srv->static_prefix);
    if (strncmp(req->path, srv->static_prefix, prefix_len) != 0) return 0;

    /* Build filesystem path */
    const char* rel = req->path + prefix_len;
    if (rel[0] == '/') rel++;  /* skip leading slash after prefix */

    /* Security: reject directory traversal and dangerous characters.
     * Block: ".." (raw and URL-encoded %2e), backticks, backslash */
    if (strstr(rel, "..") != NULL ||
        strstr(rel, "%2e") != NULL || strstr(rel, "%2E") != NULL ||
        strchr(rel, '`') != NULL ||
        strchr(rel, '\\') != NULL) {
        HttpServerResponse resp403 = {.status = 403, .body = "Forbidden",
            .content_type = "text/plain", .extra_headers = NULL};
        send_response_ka(client_fd, ssl, &resp403, keep_alive, 0);
        return 1;
    }

    char filepath[2048];
    snprintf(filepath, sizeof(filepath), "%s/%s", srv->static_dir, rel);

    /* If path is empty or ends with /, try index.html */
    int flen = (int)strlen(filepath);
    if (flen > 0 && (filepath[flen - 1] == '/' || rel[0] == '\0')) {
        if (filepath[flen - 1] != '/') strcat(filepath, "/");
        strcat(filepath, "index.html");
    }

    FILE* f = fopen(filepath, "rb");
    if (!f) return 0;  /* file not found, fall through to 404 */

    /* Get file size */
    fseek(f, 0, SEEK_END);
    long file_size = ftell(f);
    fseek(f, 0, SEEK_SET);

    /* Read file */
    char* file_data = (char*)malloc(file_size + 1);
    if (!file_data) {
        fclose(f);
        return 0;
    }
    fread(file_data, 1, file_size, f);
    fclose(f);
    file_data[file_size] = '\0';

    /* Send response */
    const char* mime = mime_for_ext(filepath);
    const char* conn = keep_alive ? "keep-alive" : "close";
    char header[4096];
    int hdr_len = snprintf(header, sizeof(header),
        "HTTP/1.1 200 OK\r\n"
        "Content-Type: %s\r\n"
        "Content-Length: %ld\r\n"
        "Connection: %s\r\n"
        "Server: Desi/0.1\r\n"
        "Cache-Control: public, max-age=3600\r\n"
        "\r\n", mime, file_size, conn);

    DESI_SEND(client_fd, ssl, header, hdr_len);
    DESI_SEND(client_fd, ssl, file_data, file_size);
    free(file_data);
    return 1;
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

static Route* match_route(HttpServer* srv, const char* method, const char* path,
                           HttpServerRequest* req) {
    if (!srv) return NULL;

    /* Pass 1: exact match (method + path) */
    for (int i = 0; i < srv->route_count; i++) {
        Route* r = &srv->routes[i];
        if (strcasecmp(r->method, method) == 0 && strcmp(r->path, path) == 0) {
            return r;
        }
    }

    /* Pass 2: wildcard method "*" with exact path */
    for (int i = 0; i < srv->route_count; i++) {
        Route* r = &srv->routes[i];
        if (strcmp(r->method, "*") == 0 && strcmp(r->path, path) == 0) {
            return r;
        }
    }

    /* Pass 3: path pattern matching with :param capture */
    for (int i = 0; i < srv->route_count; i++) {
        Route* r = &srv->routes[i];
        if (strchr(r->path, ':') == NULL) continue;  /* skip non-pattern routes */
        if (strcasecmp(r->method, method) != 0 && strcmp(r->method, "*") != 0) continue;
        if (match_path_pattern(r->path, path, req)) {
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
    /* Default CORS: allow all origins (can be overridden via __http_server_cors) */
    srv->cors_origin = strdup("*");
    srv->cors_methods = strdup("GET, POST, PUT, DELETE, PATCH, OPTIONS");
    srv->cors_headers = strdup("Content-Type, Authorization, X-Requested-With");

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

/* Create an HTTPS server with TLS certificate and key */
HttpServer* __http_server_new_tls(int port, const char* cert_path, const char* key_path) {
    HttpServer* srv = __http_server_new(port);
    if (!srv) return NULL;

    /* Initialize TLS context */
    srv->ssl_ctx = desi_tls_ctx_new(cert_path, key_path);
    if (!srv->ssl_ctx) {
        fprintf(stderr, "http_server: failed to initialize TLS with cert=%s key=%s\n",
                cert_path, key_path);
        free_server(srv);
        return NULL;
    }

    return srv;
}

/* ---- Handle a single client connection (with keep-alive) ---- */

static void handle_client(HttpServer* srv, server_socket_t client_fd, DESI_SSL* ssl) {
    __sync_fetch_and_add(&active_connections, 1);

    /* Set request timeout on the socket */
    int req_timeout = srv->request_timeout > 0 ? srv->request_timeout : KEEPALIVE_TIMEOUT_SECS;
    struct timeval tv;
    tv.tv_sec = req_timeout;
    tv.tv_usec = 0;
    setsockopt(client_fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));

    /* Get client IP for rate limiting */
    struct sockaddr_in peer_addr;
    socklen_t peer_len = sizeof(peer_addr);
    uint32_t client_ip = 0;
    if (getpeername(client_fd, (struct sockaddr*)&peer_addr, &peer_len) == 0) {
        client_ip = peer_addr.sin_addr.s_addr;
    }

    int max_body = srv->max_body_size > 0 ? srv->max_body_size : (1024 * 1024); /* default 1MB */
    int requests_served = 0;

    while (requests_served < KEEPALIVE_MAX_REQUESTS) {
        /* Read request headers first */
        char buf[65536];
        ssize_t nread = DESI_RECV(client_fd, ssl, buf, sizeof(buf) - 1);
        if (nread <= 0) {
            /* Connection closed by client or timeout */
            break;
        }
        buf[nread] = '\0';

        /* Check if we have complete headers (\r\n\r\n) */
        char* hdr_end = strstr(buf, "\r\n\r\n");
        if (!hdr_end) {
            const char* bad = "HTTP/1.1 400 Bad Request\r\n"
                              "Content-Length: 11\r\nConnection: close\r\n\r\n"
                              "Bad Request";
            DESI_SEND(client_fd, ssl, bad, strlen(bad));
            break;
        }

        /* Check Content-Length: read full body if needed */
        char* raw_buf = buf;
        int raw_len = (int)nread;
        char* heap_buf = NULL;  /* track separately for free */

        int cl = parse_content_length(buf);
        if (cl > max_body) {
            /* Reject BEFORE reading body — prevents memory exhaustion */
            const char* r413 = "HTTP/1.1 413 Payload Too Large\r\n"
                               "Content-Type: application/json\r\n"
                               "Content-Length: 30\r\nConnection: close\r\n\r\n"
                               "{\"error\":\"payload too large\"}";
            DESI_SEND(client_fd, ssl, r413, strlen(r413));
            printf("→ 413 [Content-Length %d > %d, rejected pre-read]\n", cl, max_body);
            fflush(stdout);
            break;
        }
        if (cl > 0) {
            int hdr_size = (int)(hdr_end + 4 - buf);
            int body_received = (int)nread - hdr_size;

            if (body_received < cl) {
                int total_needed = hdr_size + cl;
                heap_buf = (char*)malloc(total_needed + 1);
                memcpy(heap_buf, buf, nread);
                int total_read = (int)nread;

                while (total_read < total_needed) {
                    int to_read = total_needed - total_read;
                    if (to_read > 65536) to_read = 65536;
                    ssize_t n = DESI_RECV(client_fd, ssl, heap_buf + total_read, to_read);
                    if (n <= 0) break;
                    total_read += (int)n;
                }
                heap_buf[total_read] = '\0';
                raw_buf = heap_buf;
                raw_len = total_read;
            }
        }

        /* Parse request from the complete raw data */
        HttpServerRequest* req = parse_request(raw_buf, raw_len);
        if (heap_buf) free(heap_buf);

        if (!req) {
            const char* bad = "HTTP/1.1 400 Bad Request\r\n"
                              "Content-Length: 11\r\nConnection: close\r\n\r\n"
                              "Bad Request";
            DESI_SEND(client_fd, ssl, bad, strlen(bad));
            break;
        }

        /* ---- WebSocket Upgrade Detection ---- */
        if (req->headers) {
            char upgrade_buf[64];
            const char* upgrade = find_header_safe(req->headers, "Upgrade", upgrade_buf, sizeof(upgrade_buf));
            if (upgrade && strcasecmp(upgrade, "websocket") == 0 &&
                (__ws_state.ws_path[0] && strcmp(req->path, __ws_state.ws_path) == 0 ||
                 ws_find_route(req->path) != NULL)) {
                /* Extract Sec-WebSocket-Key */
                char key_buf[128];
                const char* ws_key = find_header_safe(req->headers, "Sec-WebSocket-Key", key_buf, sizeof(key_buf));
                if (ws_key && ws_key[0]) {
                    /* Extract Sec-WebSocket-Extensions header */
                    char ext_buf[256] = {0};
                    find_header_safe(req->headers, "Sec-WebSocket-Extensions", ext_buf, sizeof(ext_buf));

                    /* Find the end of HTTP headers (\r\n\r\n) in the raw recv buffer.
                     * Any bytes after it belong to the WebSocket session — the browser
                     * may pipeline the first frame in the same TCP segment.
                     * We replay them through ws_recv_exact before calling recv() again.
                     * Using a simple portable scan (no memmem dependency). */
                    const uint8_t* prebuf_data = NULL;
                    size_t         prebuf_len  = 0;
                    {
                        const char* p   = buf;
                        const char* end = buf + nread - 3; /* need 4 bytes to match */
                        while (p < end) {
                            if (p[0] == '\r' && p[1] == '\n' && p[2] == '\r' && p[3] == '\n') {
                                const char* ws_start = p + 4;
                                ptrdiff_t   left     = (buf + nread) - ws_start;
                                if (left > 0) {
                                    prebuf_data = (const uint8_t*)ws_start;
                                    prebuf_len  = (size_t)left;
                                }
                                break;
                            }
                            p++;
                        }
                    }



                    printf("%s %s \xe2\x86\x92 101 [ws upgrade]\n", req->method, req->path);
                    fflush(stdout);
                    free_request(req);

                    /* Prepare socket for WebSocket */
                    int nodelay = 1;
                    setsockopt(client_fd, IPPROTO_TCP, TCP_NODELAY, &nodelay, sizeof(nodelay));
                    /* Reset recv timeout to blocking for persistent WS connection.
                     * NOTE: On macOS, {0, 0} means "zero timeout" (non-blocking), NOT
                     * "no timeout". Use a large value (24h) to block effectively forever. */
                    struct timeval ws_timeout = {86400, 0}; /* 24 hours */
                    setsockopt(client_fd, SOL_SOCKET, SO_RCVTIMEO, &ws_timeout, sizeof(ws_timeout));

                    int ws_compressed = 0;
                    ws_do_handshake_ext(client_fd, ws_key, ssl,
                                        ext_buf[0] ? ext_buf : NULL, &ws_compressed);
                    ws_session_loop(client_fd, prebuf_data, prebuf_len, ssl);
                    if (ssl) desi_tls_close(ssl);
                    return; /* connection taken over by WS */
                }
            }
        }

        /* Determine if connection should persist */
        int keep_alive = should_keep_alive(req);

        /* ---- Detect client gzip support ---- */
        int accept_gzip = 0;
        if (req->headers) {
            char ae_buf[128];
            const char* ae = find_header_safe(req->headers, "Accept-Encoding", ae_buf, sizeof(ae_buf));
            if (ae == ae_buf && strstr(ae_buf, "gzip")) {
                accept_gzip = 1;
            }
        }

        /* ---- Body size enforcement ---- */
        if (req->body_len > max_body) {
            HttpServerResponse resp413 = {.status = 413, .body = "{\"error\":\"payload too large\"}",
                .content_type = "application/json", .extra_headers = NULL};
            send_response_ka(client_fd, ssl, &resp413, 0, 0);
            printf("%s %s → 413 [body %d > %d]\n", req->method, req->path, req->body_len, max_body);
            fflush(stdout);
            free_request(req);
            break;  /* close connection on oversized body */
        }

        /* ---- Rate limiting ---- */
        if (!rate_limit_check(srv, client_ip)) {
            HttpServerResponse resp429 = {.status = 429, .body = "{\"error\":\"too many requests\"}",
                .content_type = "application/json",
                .extra_headers = "Retry-After: 60\r\n"};
            send_response_ka(client_fd, ssl, &resp429, keep_alive, 0);
            printf("%s %s → 429 [rate limited]\n", req->method, req->path);
            fflush(stdout);
            requests_served++;
            free_request(req);
            if (!keep_alive) break;
            continue;
        }

        /* ---- CORS preflight auto-handler ---- */
        if (srv->cors_origin && strcmp(req->method, "OPTIONS") == 0) {
            char cors_hdrs[1024];
            snprintf(cors_hdrs, sizeof(cors_hdrs),
                "Access-Control-Allow-Origin: %s\r\n"
                "Access-Control-Allow-Methods: %s\r\n"
                "Access-Control-Allow-Headers: %s\r\n"
                "Access-Control-Max-Age: 86400\r\n",
                srv->cors_origin,
                srv->cors_methods ? srv->cors_methods : "GET, POST, PUT, DELETE, PATCH, OPTIONS",
                srv->cors_headers ? srv->cors_headers : "Content-Type, Authorization");
            HttpServerResponse preflight = {
                .status = 204, .body = "",
                .content_type = "text/plain",
                .extra_headers = cors_hdrs
            };
            send_response_ka(client_fd, ssl, &preflight, keep_alive, 0);
            printf("%s %s → 204 [CORS preflight]\n", req->method, req->path);
            fflush(stdout);
            requests_served++;
            free_request(req);
            if (!keep_alive) break;
            continue;
        }

        /* ---- Middleware chain ---- */
        HttpServerResponse* resp = NULL;
        int resp_owned = 0;

        /* Set connection info on request for SSE access */
        req->_fd = client_fd;
        req->_ssl = (void*)ssl;

        for (int m = 0; m < srv->middleware_count; m++) {
            resp = srv->middlewares[m](req);
            if (resp) {
                resp_owned = 1;
                break;  /* middleware short-circuited */
            }
        }

        /* ---- Route dispatch (if middleware didn't handle) ---- */
        if (!resp) {
            if (__desi_http_handler) {
                resp = __desi_http_handler(req);
                resp_owned = 1;
            } else if (srv->route_count > 0) {
                Route* route = match_route(srv, req->method, req->path, req);
                if (route && route->handler) {
                    resp = route->handler(req);
                    resp_owned = 1;
                } else if (path_exists(srv, req->path)) {
                    resp = &default_405;
                } else if (try_serve_static(srv, client_fd, ssl, req, keep_alive)) {
                    printf("%s %s → 200 [static]%s\n", req->method, req->path,
                           keep_alive ? " [ka]" : "");
                    fflush(stdout);
                    requests_served++;
                    free_request(req);
                    if (!keep_alive) break;
                    continue;
                } else {
                    resp = &default_404;
                }
            } else if (try_serve_static(srv, client_fd, ssl, req, keep_alive)) {
                printf("%s %s → 200 [static]%s\n", req->method, req->path,
                       keep_alive ? " [ka]" : "");
                fflush(stdout);
                requests_served++;
                free_request(req);
                if (!keep_alive) break;
                continue;
            } else {
                /* No routes registered — echo mode */
                char body[2048];
                snprintf(body, sizeof(body),
                    "{\"status\":\"ok\",\"server\":\"desi\","
                    "\"request\":{\"method\":\"%s\",\"path\":\"%s\",\"query\":\"%s\"}}",
                    req->method, req->path, req->query);

                resp = __http_resp_new(200, body, "application/json");
                resp_owned = 1;
            }
        }

        /* ---- SSE sentinel: handler took over the connection ---- */
        if (resp && resp->status == -2) {
            printf("%s %s → SSE [stream] (fd=%d)\n", req->method, req->path, client_fd);
            fflush(stdout);
            if (resp_owned) free_response(resp);
            free_request(req);
            /* Connection is managed by SSE handler — don't close socket */
            return;
        }

        /* ---- Inject CORS origin header if configured ---- */
        if (srv->cors_origin && resp) {
            __http_resp_header(resp, "Access-Control-Allow-Origin", srv->cors_origin);
        }

        send_response_ka(client_fd, ssl, resp, keep_alive, accept_gzip);
        requests_served++;

        /* Log */
        int status = resp ? resp->status : 500;
        printf("%s %s → %d%s (fd=%d)\n", req->method, req->path, status,
               keep_alive ? " [ka]" : "", client_fd);
        fflush(stdout);

        if (resp_owned) free_response(resp);
        free_request(req);

        /* Close connection if not keep-alive */
        if (!keep_alive) break;
    }

    /* --- Cleanup below this point (all paths exit handle_client) --- */
    if (ssl) desi_tls_close(ssl);
    CLOSE_SOCKET(client_fd);
    __sync_fetch_and_sub(&active_connections, 1);
}

/* ---- Client task for supervisor pool dispatch ---- */

typedef struct {
    HttpServer*      srv;
    server_socket_t  client_fd;
    DESI_SSL*        ssl;
} ClientTask;

static void client_task_fn(void* arg) {
    ClientTask* task = (ClientTask*)arg;
    handle_client(task->srv, task->client_fd, task->ssl);
    free(task);
}

/* Default pool size for HTTP server workers */
#define HTTP_SERVER_POOL_SIZE 8

/* ---- Blocking accept loop with Supervisor pool + graceful shutdown ---- */

void __http_server_run(HttpServer* srv) {
    if (!srv) {
        fprintf(stderr, "http_server: server is NULL\n");
        return;
    }

    install_signal_handlers();

    /* Initialize WebSocket state (rwlock) before spawning worker threads */
    ws_state_init();
    /* Create supervisor with worker pool for concurrent request handling */
    Supervisor* sup = supervisor_new(0, HTTP_SERVER_POOL_SIZE);  /* ONE_FOR_ONE, 8 workers */
    if (!sup) {
        fprintf(stderr, "http_server: failed to create supervisor\n");
        free_server(srv);
        return;
    }

    const char* proto = srv->ssl_ctx ? "https" : "http";
    printf("\033[32m✓ Desi HTTP Server listening on %s://0.0.0.0:%d\033[0m\n", proto, srv->port);
    printf("  Workers: %d\n", HTTP_SERVER_POOL_SIZE);
    if (srv->ssl_ctx) printf("  TLS: enabled\n");
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

        /* TLS handshake (if HTTPS mode) */
        DESI_SSL* conn_ssl = NULL;
        if (srv->ssl_ctx) {
            conn_ssl = desi_tls_accept(srv->ssl_ctx, client_fd);
            if (!conn_ssl) {
                /* Handshake failed — close and continue */
                CLOSE_SOCKET(client_fd);
                continue;
            }
        }

        /* Dispatch to supervisor pool for concurrent handling */
        ClientTask* task = (ClientTask*)malloc(sizeof(ClientTask));
        if (task) {
            task->srv = srv;
            task->client_fd = client_fd;
            task->ssl = conn_ssl;
            supervisor_submit(sup, client_task_fn, task);
        } else {
            /* Fallback: handle synchronously if malloc fails */
            handle_client(srv, client_fd, conn_ssl);
        }
    }

    /* Graceful shutdown: drain queue, join workers, free supervisor */
    int active = __sync_fetch_and_add(&active_connections, 0);
    if (active > 0) {
        printf("\033[33m⏳ Draining %d active connection(s)...\033[0m\n", active);
        fflush(stdout);
    }
    supervisor_stop(sup);
    free(sup);  /* supervisor_stop doesn't free the struct itself (idempotent design) */

    /* Cleanup — free server struct, routes, and socket */
    free_server(srv);
    __desi_http_handler = NULL; /* reset global handler */
    printf("\033[32m✓ Server stopped\033[0m\n");
    fflush(stdout);
}

/* Programmatic shutdown — callable from Desi code */
void __http_server_shutdown(void) {
    server_running = 0;
}

/* ============================================================
 * Multipart Form Data Parser (RFC 2046)
 * ============================================================ */

#define MAX_FORM_PARTS 32

typedef struct {
    char name[128];
    char filename[256];     /* empty if not a file upload */
    char content_type[128]; /* for file uploads */
    char* data;
    int  data_len;
} FormPart;

typedef struct {
    FormPart parts[MAX_FORM_PARTS];
    int count;
} FormData;

/* Thread-local form data for current request */
static __thread FormData* _current_form = NULL;

static char* find_boundary(const char* content_type) {
    const char* bp = strstr(content_type, "boundary=");
    if (!bp) return NULL;
    bp += 9;
    /* Skip optional quotes */
    if (*bp == '"') bp++;
    size_t len = 0;
    while (bp[len] && bp[len] != '"' && bp[len] != '\r' && bp[len] != '\n' && bp[len] != ';')
        len++;
    char* result = (char*)malloc(len + 1);
    memcpy(result, bp, len);
    result[len] = '\0';
    return result;
}

static void parse_multipart(HttpServerRequest* req) {
    if (_current_form) return; /* already parsed */
    if (!req->headers || !req->body) return;

    /* Find Content-Type header with boundary */
    const char* ct = NULL;
    const char* p = req->headers;
    while (*p) {
        if (strncasecmp(p, "Content-Type:", 13) == 0) {
            ct = p + 13;
            while (*ct == ' ') ct++;
            break;
        }
        const char* eol = strstr(p, "\r\n");
        if (!eol) break;
        p = eol + 2;
    }
    if (!ct || !strstr(ct, "multipart/form-data")) return;

    char* boundary = find_boundary(ct);
    if (!boundary) return;

    _current_form = (FormData*)calloc(1, sizeof(FormData));

    /* Build full boundary markers */
    size_t blen = strlen(boundary);
    char* delim = (char*)malloc(blen + 5);
    sprintf(delim, "--%s", boundary);
    size_t dlen = strlen(delim);

    const char* body = req->body;
    int body_len = req->body_len;

    /* Find first boundary */
    const char* pos = memmem(body, body_len, delim, dlen);
    if (!pos) { free(boundary); free(delim); return; }

    while (pos && _current_form->count < MAX_FORM_PARTS) {
        pos += dlen;
        if (pos[0] == '-' && pos[1] == '-') break; /* closing boundary */
        if (pos[0] == '\r') pos += 2; /* skip CRLF */

        /* Parse part headers */
        FormPart* part = &_current_form->parts[_current_form->count];
        memset(part, 0, sizeof(FormPart));

        const char* hdr_end = strstr(pos, "\r\n\r\n");
        if (!hdr_end) break;

        /* Extract Content-Disposition fields */
        const char* disp = strstr(pos, "name=\"");
        if (disp && disp < hdr_end) {
            disp += 6;
            const char* end = strchr(disp, '"');
            if (end && end < hdr_end) {
                size_t nlen = (size_t)(end - disp);
                if (nlen >= sizeof(part->name)) nlen = sizeof(part->name) - 1;
                memcpy(part->name, disp, nlen);
            }
        }

        const char* fname = strstr(pos, "filename=\"");
        if (fname && fname < hdr_end) {
            fname += 10;
            const char* end = strchr(fname, '"');
            if (end && end < hdr_end) {
                size_t flen = (size_t)(end - fname);
                if (flen >= sizeof(part->filename)) flen = sizeof(part->filename) - 1;
                memcpy(part->filename, fname, flen);
            }
        }

        /* Extract Content-Type if present */
        const char* pct = strstr(pos, "Content-Type:");
        if (pct && pct < hdr_end) {
            pct += 13;
            while (*pct == ' ') pct++;
            const char* end = strstr(pct, "\r\n");
            if (end && end <= hdr_end) {
                size_t ctlen = (size_t)(end - pct);
                if (ctlen >= sizeof(part->content_type)) ctlen = sizeof(part->content_type) - 1;
                memcpy(part->content_type, pct, ctlen);
            }
        }

        /* Data starts after headers */
        const char* data_start = hdr_end + 4;

        /* Find next boundary */
        const char* remaining = data_start;
        size_t remain_len = body_len - (size_t)(data_start - body);
        const char* next = memmem(remaining, remain_len, delim, dlen);
        if (!next) break;

        /* Data ends 2 bytes before next boundary (CRLF before boundary) */
        size_t data_len = (size_t)(next - data_start);
        if (data_len >= 2) data_len -= 2; /* remove trailing CRLF */

        part->data = (char*)malloc(data_len + 1);
        memcpy(part->data, data_start, data_len);
        part->data[data_len] = '\0';
        part->data_len = (int)data_len;

        _current_form->count++;
        pos = next;
    }

    free(boundary);
    free(delim);
}

static void free_form_data(void) {
    if (!_current_form) return;
    for (int i = 0; i < _current_form->count; i++) {
        free(_current_form->parts[i].data);
    }
    free(_current_form);
    _current_form = NULL;
}

/* Get a form field value by name (for text fields) */
const char* __http_req_form_field(void* raw_req, const char* name) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    parse_multipart(req);
    if (!_current_form || !name) return "";
    for (int i = 0; i < _current_form->count; i++) {
        if (strcmp(_current_form->parts[i].name, name) == 0 &&
            _current_form->parts[i].filename[0] == '\0') {
            return _current_form->parts[i].data ? _current_form->parts[i].data : "";
        }
    }
    return "";
}

/* Get uploaded file data by field name */
const char* __http_req_form_file(void* raw_req, const char* name) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    parse_multipart(req);
    if (!_current_form || !name) return "";
    for (int i = 0; i < _current_form->count; i++) {
        if (strcmp(_current_form->parts[i].name, name) == 0 &&
            _current_form->parts[i].filename[0] != '\0') {
            return _current_form->parts[i].data ? _current_form->parts[i].data : "";
        }
    }
    return "";
}

/* Get uploaded filename by field name */
const char* __http_req_form_filename(void* raw_req, const char* name) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    parse_multipart(req);
    if (!_current_form || !name) return "";
    for (int i = 0; i < _current_form->count; i++) {
        if (strcmp(_current_form->parts[i].name, name) == 0 &&
            _current_form->parts[i].filename[0] != '\0') {
            return _current_form->parts[i].filename;
        }
    }
    return "";
}

/* Get file data length by field name */
int32_t __http_req_form_file_size(void* raw_req, const char* name) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    parse_multipart(req);
    if (!_current_form || !name) return 0;
    for (int i = 0; i < _current_form->count; i++) {
        if (strcmp(_current_form->parts[i].name, name) == 0 &&
            _current_form->parts[i].filename[0] != '\0') {
            return _current_form->parts[i].data_len;
        }
    }
    return 0;
}

/* ============================================================
 * SSE (Server-Sent Events)
 *
 * Usage from Desi:
 *   http.sse_start(req)              → send SSE headers
 *   http.sse_send(req, data)         → send a data event
 *   http.sse_send(req, data, event)  → send a named event
 *   http.sse_close(req)              → close the SSE stream
 *
 * Handler returns http.sse_response() → sentinel (status -2)
 * so handle_client knows the connection is managed by SSE.
 * ============================================================ */

/* Send SSE response headers to begin streaming */
int __http_sse_start(void* raw_req) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    if (!req) return 0;

    const char* headers =
        "HTTP/1.1 200 OK\r\n"
        "Content-Type: text/event-stream\r\n"
        "Cache-Control: no-cache\r\n"
        "Connection: keep-alive\r\n"
        "X-Accel-Buffering: no\r\n"
        "Server: Desi/0.1\r\n"
        "\r\n";

    DESI_SEND(req->_fd, (DESI_SSL*)req->_ssl, headers, strlen(headers));
    return 1;
}

/* Internal: write SSE data lines to buffer, return new len */
static int sse_write_data(char* buf, int len, int buf_size, const char* data) {
    const char* p = data;
    while (*p) {
        const char* nl = strchr(p, '\n');
        if (nl) {
            int line_len = (int)(nl - p);
            len += snprintf(buf + len, buf_size - len, "data: %.*s\n", line_len, p);
            p = nl + 1;
        } else {
            len += snprintf(buf + len, buf_size - len, "data: %s\n", p);
            break;
        }
    }
    /* Empty line terminates the event */
    len += snprintf(buf + len, buf_size - len, "\n");
    return len;
}

/* Send an SSE data-only event (no event type) */
void __http_sse_send_data(void* raw_req, const char* data) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    if (!req || !data) return;

    char buf[8192];
    int len = sse_write_data(buf, 0, sizeof(buf), data);
    DESI_SEND(req->_fd, (DESI_SSL*)req->_ssl, buf, len);
}

/* Send an SSE event with a named event type */
void __http_sse_send(void* raw_req, const char* data, const char* event) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    if (!req || !data) return;

    char buf[8192];
    int len = 0;

    /* Event type line */
    if (event && strlen(event) > 0) {
        len += snprintf(buf + len, sizeof(buf) - len, "event: %s\n", event);
    }

    len = sse_write_data(buf, len, sizeof(buf), data);
    DESI_SEND(req->_fd, (DESI_SSL*)req->_ssl, buf, len);
}

/* Close the SSE stream */
void __http_sse_close(void* raw_req) {
    HttpServerRequest* req = (HttpServerRequest*)raw_req;
    if (!req) return;

    if (req->_ssl) desi_tls_close((DESI_SSL*)req->_ssl);
    CLOSE_SOCKET(req->_fd);
}

/* Create sentinel response for SSE (status -2) */
void* __http_sse_response(void) {
    return __http_resp_new(-2, "", "text/event-stream");
}
