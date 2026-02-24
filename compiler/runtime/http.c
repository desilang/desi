/*
 * http.c — Self-contained HTTP client for Desi
 *
 * Architecture:
 *   http.c            — Core HTTP logic (URL parsing, request/response, public API)
 *   http/http_internal.h  — Shared types (Buffer, Connection, ParsedURL, socket I/O)
 *   http/tls_apple.h      — macOS TLS (SecureTransport, system framework)
 *   http/tls_openssl.h    — Linux TLS (OpenSSL, system library)
 *   http/tls_win.h        — Windows TLS (Schannel stub, Phase 2)
 *
 * Cross-platform:
 *   macOS   — POSIX sockets + SecureTransport    (zero external deps)
 *   Linux   — POSIX sockets + OpenSSL            (system library)
 *   Windows — WinSock2 + Schannel                (HTTP only in Phase 1)
 *
 * Supports:  HTTP/1.1, HTTPS, IPv4+IPv6, chunked encoding,
 *            redirects (up to 10), URL encode/decode
 */

#include "http/http_internal.h"

/* Include the correct TLS backend for this platform */
#if defined(__APPLE__)
  #include "http/tls_apple.h"
#elif defined(_WIN32)
  #include "http/tls_win.h"
#else
  #include "http/tls_openssl.h"
#endif

#ifdef _WIN32
static int _wsa_initialized = 0;
static void ensure_wsa(void) {
    if (!_wsa_initialized) {
        WSADATA wsa;
        WSAStartup(MAKEWORD(2, 2), &wsa);
        _wsa_initialized = 1;
    }
}
#else
#define ensure_wsa() ((void)0)
#endif

/* ---- TCP connection ---- */

static DESI_SOCKET tcp_connect(const char *host, const char *port) {
    ensure_wsa();
    struct addrinfo hints, *res, *rp;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;     /* IPv4 + IPv6 dual-stack */
    hints.ai_socktype = SOCK_STREAM;

    if (getaddrinfo(host, port, &hints, &res) != 0)
        return DESI_INVALID_SOCKET;

    DESI_SOCKET fd = DESI_INVALID_SOCKET;
    for (rp = res; rp; rp = rp->ai_next) {
        fd = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (fd == DESI_INVALID_SOCKET) continue;
        if (connect(fd, rp->ai_addr, (int)rp->ai_addrlen) == 0) break;
        DESI_CLOSE_SOCKET(fd);
        fd = DESI_INVALID_SOCKET;
    }
    freeaddrinfo(res);
    return fd;
}

/* ---- URL parsing ---- */

static int parse_url(const char *url, ParsedURL *out) {
    memset(out, 0, sizeof(*out));

    const char *p = strstr(url, "://");
    if (!p) return -1;
    size_t slen = (size_t)(p - url);
    if (slen >= sizeof(out->scheme)) return -1;
    memcpy(out->scheme, url, slen);
    p += 3;

    const char *slash = strchr(p, '/');
    const char *hostend = slash ? slash : p + strlen(p);

    if (*p == '[') {
        /* IPv6 bracket notation: [::1] */
        const char *bracket = strchr(p, ']');
        if (!bracket || bracket > hostend) return -1;
        size_t hlen = (size_t)(bracket - p - 1);
        if (hlen >= sizeof(out->host)) return -1;
        memcpy(out->host, p + 1, hlen);
        p = bracket + 1;
        if (*p == ':') {
            p++;
            size_t plen = (size_t)(hostend - p);
            if (plen >= sizeof(out->port)) return -1;
            memcpy(out->port, p, plen);
        }
    } else {
        const char *colon = NULL;
        for (const char *c = p; c < hostend; c++)
            if (*c == ':') colon = c;
        if (colon) {
            size_t hlen = (size_t)(colon - p);
            if (hlen >= sizeof(out->host)) return -1;
            memcpy(out->host, p, hlen);
            size_t plen = (size_t)(hostend - colon - 1);
            if (plen >= sizeof(out->port)) return -1;
            memcpy(out->port, colon + 1, plen);
        } else {
            size_t hlen = (size_t)(hostend - p);
            if (hlen >= sizeof(out->host)) return -1;
            memcpy(out->host, p, hlen);
        }
    }

    if (out->port[0] == '\0')
        strcpy(out->port, strcmp(out->scheme, "https") == 0 ? "443" : "80");

    if (slash) {
        size_t pathlen = strlen(slash);
        if (pathlen >= sizeof(out->path)) pathlen = sizeof(out->path) - 1;
        memcpy(out->path, slash, pathlen);
    } else {
        strcpy(out->path, "/");
    }
    return 0;
}

/* ---- Connection I/O (dispatches to TLS or plain) ---- */

static ssize_t conn_write(Connection *c, const void *buf, size_t len) {
    if (c->use_tls) return tls_write(c, buf, len);
    return sock_send(c->fd, buf, len);
}

static ssize_t conn_read(Connection *c, void *buf, size_t len) {
    if (c->use_tls) return tls_read(c, buf, len);
    return sock_recv(c->fd, buf, len);
}

static void conn_close(Connection *c) {
    if (c->use_tls) tls_close(c);
    if (c->fd != DESI_INVALID_SOCKET) {
        DESI_CLOSE_SOCKET(c->fd);
        c->fd = DESI_INVALID_SOCKET;
    }
}

/* ---- HTTP protocol helpers ---- */

static int send_all(Connection *c, const char *data, size_t len) {
    size_t sent = 0;
    while (sent < len) {
        ssize_t n = conn_write(c, data + sent, len - sent);
        if (n <= 0) return -1;
        sent += (size_t)n;
    }
    return 0;
}

static int read_headers(Connection *c, Buffer *buf) {
    char tmp[4096];
    while (1) {
        ssize_t n = conn_read(c, tmp, sizeof(tmp));
        if (n <= 0) break;
        buf_append(buf, tmp, (size_t)n);
        if (strstr(buf->data, "\r\n\r\n")) return 0;
    }
    return (buf->len > 0 && strstr(buf->data, "\r\n\r\n")) ? 0 : -1;
}

static int parse_status(const char *line) {
    const char *p = strchr(line, ' ');
    return p ? atoi(p + 1) : -1;
}

static const char *find_header(const char *headers, const char *name) {
    size_t nlen = strlen(name);
    const char *p = headers;
    while (*p) {
#ifdef _WIN32
        if (_strnicmp(p, name, nlen) == 0 && p[nlen] == ':') {
#else
        if (strncasecmp(p, name, nlen) == 0 && p[nlen] == ':') {
#endif
            const char *val = p + nlen + 1;
            while (*val == ' ') val++;
            return val;
        }
        const char *eol = strstr(p, "\r\n");
        if (!eol) break;
        p = eol + 2;
    }
    return NULL;
}

static void read_body_content_length(Connection *c, Buffer *body, const char *left, size_t left_len, long cl) {
    if (left_len > 0) {
        size_t take = left_len < (size_t)cl ? left_len : (size_t)cl;
        buf_append(body, left, take);
    }
    char tmp[4096];
    while ((long)body->len < cl) {
        size_t want = (size_t)(cl - (long)body->len);
        if (want > sizeof(tmp)) want = sizeof(tmp);
        ssize_t n = conn_read(c, tmp, want);
        if (n <= 0) break;
        buf_append(body, tmp, (size_t)n);
    }
}

static void read_body_chunked(Connection *c, Buffer *body, Buffer *raw) {
    const char *p = raw->data;
    const char *end = raw->data + raw->len;

    while (1) {
        const char *crlf = strstr(p, "\r\n");
        if (!crlf) {
            char tmp[4096];
            ssize_t n = conn_read(c, tmp, sizeof(tmp));
            if (n <= 0) break;
            size_t off = (size_t)(p - raw->data);
            buf_append(raw, tmp, (size_t)n);
            p = raw->data + off;
            end = raw->data + raw->len;
            continue;
        }
        long sz = strtol(p, NULL, 16);
        if (sz == 0) break;
        p = crlf + 2;
        size_t avail = (size_t)(end - p);
        while (avail < (size_t)sz + 2) {
            char tmp[4096];
            ssize_t n = conn_read(c, tmp, sizeof(tmp));
            if (n <= 0) return;
            size_t off = (size_t)(p - raw->data);
            buf_append(raw, tmp, (size_t)n);
            p = raw->data + off;
            end = raw->data + raw->len;
            avail = (size_t)(end - p);
        }
        buf_append(body, p, (size_t)sz);
        p += sz + 2;
    }
}

static void read_body_until_close(Connection *c, Buffer *body, const char *left, size_t left_len) {
    if (left_len > 0) buf_append(body, left, left_len);
    char tmp[4096];
    while (1) {
        ssize_t n = conn_read(c, tmp, sizeof(tmp));
        if (n <= 0) break;
        buf_append(body, tmp, (size_t)n);
    }
}

static HttpResponse *make_error(const char *msg) {
    HttpResponse *r = (HttpResponse *)calloc(1, sizeof(HttpResponse));
    r->status = -1;
    r->body = strdup(msg);
    r->headers = strdup("");
    return r;
}

/* ---- Core request ---- */

static HttpResponse *http_do_request(const char *method, const char *url,
                                     const char *body_data, const char *extra_headers,
                                     int max_redirects) {
    ParsedURL parsed;
    if (parse_url(url, &parsed) != 0)
        return make_error("invalid URL");

    Connection conn;
    memset(&conn, 0, sizeof(conn));
    conn.fd = DESI_INVALID_SOCKET;

    conn.fd = tcp_connect(parsed.host, parsed.port);
    if (conn.fd == DESI_INVALID_SOCKET)
        return make_error("connection failed");

    if (strcmp(parsed.scheme, "https") == 0) {
        if (tls_handshake(&conn, parsed.host) != 0) {
            DESI_CLOSE_SOCKET(conn.fd);
            return make_error("TLS handshake failed");
        }
    }

    /* Build request */
    Buffer req;
    buf_init(&req);
    buf_append(&req, method, strlen(method));
    buf_append(&req, " ", 1);
    buf_append(&req, parsed.path, strlen(parsed.path));
    buf_append(&req, " HTTP/1.1\r\n", 11);

    buf_append(&req, "Host: ", 6);
    buf_append(&req, parsed.host, strlen(parsed.host));
    if ((strcmp(parsed.scheme, "http") == 0 && strcmp(parsed.port, "80") != 0) ||
        (strcmp(parsed.scheme, "https") == 0 && strcmp(parsed.port, "443") != 0)) {
        buf_append(&req, ":", 1);
        buf_append(&req, parsed.port, strlen(parsed.port));
    }
    buf_append(&req, "\r\n", 2);

    buf_append(&req, "User-Agent: Desi/1.0\r\n", 22);
    buf_append(&req, "Accept: */*\r\n", 13);
    buf_append(&req, "Accept-Encoding: identity\r\n", 27);
    buf_append(&req, "Connection: close\r\n", 19);

    if (body_data && strlen(body_data) > 0) {
        char cl[64];
        snprintf(cl, sizeof(cl), "Content-Length: %zu\r\n", strlen(body_data));
        buf_append(&req, cl, strlen(cl));
    }

    if (extra_headers && strlen(extra_headers) > 0) {
        buf_append(&req, extra_headers, strlen(extra_headers));
        if (req.len >= 2 && (req.data[req.len-2] != '\r' || req.data[req.len-1] != '\n'))
            buf_append(&req, "\r\n", 2);
    }

    buf_append(&req, "\r\n", 2);
    if (body_data && strlen(body_data) > 0)
        buf_append(&req, body_data, strlen(body_data));

    if (send_all(&conn, req.data, req.len) != 0) {
        buf_free(&req); conn_close(&conn);
        return make_error("send failed");
    }
    buf_free(&req);

    /* Read response */
    Buffer raw;
    buf_init(&raw);
    if (read_headers(&conn, &raw) != 0) {
        buf_free(&raw); conn_close(&conn);
        return make_error("no response");
    }

    char *hdr_end = strstr(raw.data, "\r\n\r\n");
    size_t hdr_len = (size_t)(hdr_end - raw.data);
    char *leftover = hdr_end + 4;
    size_t left_len = raw.len - hdr_len - 4;

    int status = parse_status(raw.data);

    char *resp_hdrs = (char *)malloc(hdr_len + 1);
    memcpy(resp_hdrs, raw.data, hdr_len);
    resp_hdrs[hdr_len] = '\0';

    /* Redirects */
    if (max_redirects > 0 && (status == 301 || status == 302 || status == 303 || status == 307 || status == 308)) {
        const char *loc = find_header(resp_hdrs, "Location");
        if (loc) {
            const char *loc_end = strstr(loc, "\r\n");
            size_t loc_len = loc_end ? (size_t)(loc_end - loc) : strlen(loc);
            char *redir_url = (char *)malloc(loc_len + 1);
            memcpy(redir_url, loc, loc_len);
            redir_url[loc_len] = '\0';

            char abs_buf[4096];
            char *final_url = redir_url;
            if (redir_url[0] == '/') {
                snprintf(abs_buf, sizeof(abs_buf), "%s://%s%s", parsed.scheme, parsed.host, redir_url);
                final_url = abs_buf;
            }

            buf_free(&raw); free(resp_hdrs); conn_close(&conn);

            const char *rm = (status == 303) ? "GET" : method;
            const char *rb = (status == 303) ? NULL : body_data;
            HttpResponse *r = http_do_request(rm, final_url, rb, extra_headers, max_redirects - 1);
            free(redir_url);
            return r;
        }
    }

    /* Read body */
    Buffer resp_body;
    buf_init(&resp_body);

    if (strcmp(method, "HEAD") != 0) {
        const char *te = find_header(resp_hdrs, "Transfer-Encoding");
        const char *cl = find_header(resp_hdrs, "Content-Length");

        if (te && strncasecmp(te, "chunked", 7) == 0) {
            Buffer chunk_raw;
            buf_init(&chunk_raw);
            buf_append(&chunk_raw, leftover, left_len);
            read_body_chunked(&conn, &resp_body, &chunk_raw);
            buf_free(&chunk_raw);
        } else if (cl) {
            read_body_content_length(&conn, &resp_body, leftover, left_len, atol(cl));
        } else {
            read_body_until_close(&conn, &resp_body, leftover, left_len);
        }
    }

    buf_free(&raw);
    conn_close(&conn);

    HttpResponse *r = (HttpResponse *)calloc(1, sizeof(HttpResponse));
    r->status = status;
    r->body = resp_body.data ? resp_body.data : strdup("");
    r->headers = resp_hdrs;
    return r;
}

/* ==== Public API (called from Desi via @extern) ==== */

void *__http_request(const char *method, const char *url, const char *body, const char *headers) {
    return http_do_request(method, url, body, headers, 10);
}

void *__http_get(const char *url)               { return http_do_request("GET",     url, NULL, NULL, 10); }
void *__http_post(const char *url, const char *body)  { return http_do_request("POST",    url, body, "Content-Type: application/json\r\n", 10); }
void *__http_put(const char *url, const char *body)   { return http_do_request("PUT",     url, body, "Content-Type: application/json\r\n", 10); }
void *__http_patch(const char *url, const char *body)  { return http_do_request("PATCH",   url, body, "Content-Type: application/json\r\n", 10); }
void *__http_delete(const char *url)            { return http_do_request("DELETE",  url, NULL, NULL, 10); }
void *__http_head(const char *url)              { return http_do_request("HEAD",    url, NULL, NULL, 10); }
void *__http_options(const char *url)           { return http_do_request("OPTIONS", url, NULL, NULL, 10); }

int32_t __http_response_status(void *resp) {
    return resp ? ((HttpResponse *)resp)->status : -1;
}

const char *__http_response_body(void *resp) {
    if (!resp) return "";
    const char *b = ((HttpResponse *)resp)->body;
    return b ? b : "";
}

const char *__http_response_headers(void *resp) {
    if (!resp) return "";
    const char *h = ((HttpResponse *)resp)->headers;
    return h ? h : "";
}

int32_t __http_response_free(void *resp) {
    if (!resp) return 0;
    HttpResponse *r = (HttpResponse *)resp;
    free(r->body);
    free(r->headers);
    free(r);
    return 0;
}

/* ---- URL encode/decode ---- */

const char *__http_url_encode(const char *s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char *out = (char *)malloc(len * 3 + 1);
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if (isalnum(c) || c == '-' || c == '_' || c == '.' || c == '~')
            out[j++] = (char)c;
        else {
            sprintf(out + j, "%%%02X", c);
            j += 3;
        }
    }
    out[j] = '\0';
    return out;
}

static int hex_val(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return -1;
}

const char *__http_url_decode(const char *s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char *out = (char *)malloc(len + 1);
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        if (s[i] == '%' && i + 2 < len) {
            int h = hex_val(s[i+1]), l = hex_val(s[i+2]);
            if (h >= 0 && l >= 0) { out[j++] = (char)(h * 16 + l); i += 2; continue; }
        }
        out[j++] = (s[i] == '+') ? ' ' : s[i];
    }
    out[j] = '\0';
    return out;
}
