/*
 * url.c — URL parsing, building, and encoding for Desi stdlib
 *
 * RFC 3986 compliant URL parser. No external dependencies.
 *
 * Public API:
 *   __url_parse(url_str)        → parse into components
 *   __url_build(scheme,host,port,path,query,fragment) → build URL string
 *   __url_encode(str)           → percent-encode
 *   __url_decode(str)           → percent-decode
 *   __url_query_get(query,key)  → get value for key from query string
 *   __url_join(base,ref)        → resolve relative URL against base
 *
 *   Parsed URL struct fields (accessed via __url_get_*):
 *     scheme, userinfo, host, port, path, query, fragment
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdint.h>

/* ============================================================
 * Parsed URL structure
 * ============================================================ */

typedef struct {
    char* scheme;     /* "https" */
    char* userinfo;   /* "user:pass" or "" */
    char* host;       /* "example.com" */
    int   port;       /* 8080, or -1 if unspecified */
    char* path;       /* "/api/v1/users" */
    char* query;      /* "page=2&limit=10" (without ?) */
    char* fragment;   /* "section1" (without #) */
    char* raw;        /* original URL string */
} DesiUrl;

static char* safe_strdup(const char* s) {
    return s ? strdup(s) : strdup("");
}

static char* strndup_safe(const char* s, size_t n) {
    char* r = (char*)malloc(n + 1);
    memcpy(r, s, n);
    r[n] = '\0';
    return r;
}

/* ============================================================
 * URL Parser (RFC 3986)
 * ============================================================ */

DesiUrl* __url_parse(const char* url_str) {
    if (!url_str) url_str = "";
    DesiUrl* u = (DesiUrl*)calloc(1, sizeof(DesiUrl));
    u->port = -1;
    u->raw = strdup(url_str);

    const char* p = url_str;
    const char* end = url_str + strlen(url_str);

    /* 1. Scheme — look for "://" */
    const char* scheme_end = strstr(p, "://");
    if (scheme_end && scheme_end > p) {
        u->scheme = strndup_safe(p, scheme_end - p);
        p = scheme_end + 3;
    } else {
        u->scheme = strdup("");
    }

    /* 2. Fragment — split off # first */
    const char* frag = strchr(p, '#');
    if (frag) {
        u->fragment = strdup(frag + 1);
        end = frag;
    } else {
        u->fragment = strdup("");
    }

    /* 3. Query — split off ? */
    const char* qmark = NULL;
    for (const char* scan = p; scan < end; scan++) {
        if (*scan == '?') { qmark = scan; break; }
    }
    if (qmark) {
        u->query = strndup_safe(qmark + 1, end - qmark - 1);
        end = qmark;
    } else {
        u->query = strdup("");
    }

    /* 4. Authority (userinfo@host:port) vs path */
    if (u->scheme[0] != '\0') {
        /* Has scheme → look for authority */
        const char* path_start = NULL;
        for (const char* scan = p; scan < end; scan++) {
            if (*scan == '/') { path_start = scan; break; }
        }

        const char* auth_end = path_start ? path_start : end;

        /* Parse authority: userinfo@host:port */
        const char* at = NULL;
        for (const char* scan = p; scan < auth_end; scan++) {
            if (*scan == '@') { at = scan; break; }
        }

        const char* host_start;
        if (at) {
            u->userinfo = strndup_safe(p, at - p);
            host_start = at + 1;
        } else {
            u->userinfo = strdup("");
            host_start = p;
        }

        /* Host:port — scan backwards for last colon (handle IPv6 brackets) */
        const char* colon = NULL;
        if (*host_start == '[') {
            /* IPv6: [::1]:8080 */
            const char* bracket = strchr(host_start, ']');
            if (bracket && bracket + 1 < auth_end && *(bracket + 1) == ':') {
                colon = bracket + 1;
            }
            if (bracket) {
                u->host = strndup_safe(host_start, (bracket + 1) - host_start);
            } else {
                u->host = strndup_safe(host_start, auth_end - host_start);
            }
        } else {
            /* Find last colon for port */
            for (const char* scan = auth_end - 1; scan >= host_start; scan--) {
                if (*scan == ':') { colon = scan; break; }
            }
            if (colon) {
                u->host = strndup_safe(host_start, colon - host_start);
            } else {
                u->host = strndup_safe(host_start, auth_end - host_start);
            }
        }

        if (colon && colon + 1 < auth_end) {
            u->port = atoi(colon + 1);
        }

        /* Path */
        if (path_start) {
            u->path = strndup_safe(path_start, end - path_start);
        } else {
            u->path = strdup("");
        }
    } else {
        /* No scheme → entire thing is a path (or relative URL) */
        u->userinfo = strdup("");
        u->host = strdup("");
        u->path = strndup_safe(p, end - p);
    }

    return u;
}

/* ============================================================
 * Field Accessors
 * ============================================================ */

const char* __url_get_scheme(DesiUrl* u)   { return u ? u->scheme   : ""; }
const char* __url_get_userinfo(DesiUrl* u) { return u ? u->userinfo : ""; }
const char* __url_get_host(DesiUrl* u)     { return u ? u->host     : ""; }
int32_t     __url_get_port(DesiUrl* u)     { return u ? u->port     : -1; }
const char* __url_get_path(DesiUrl* u)     { return u ? u->path     : ""; }
const char* __url_get_query(DesiUrl* u)    { return u ? u->query    : ""; }
const char* __url_get_fragment(DesiUrl* u) { return u ? u->fragment : ""; }
const char* __url_get_raw(DesiUrl* u)      { return u ? u->raw      : ""; }

void __url_free(DesiUrl* u) {
    if (!u) return;
    free(u->scheme); free(u->userinfo); free(u->host);
    free(u->path); free(u->query); free(u->fragment); free(u->raw);
    free(u);
}

/* ============================================================
 * URL Building
 * ============================================================ */

char* __url_build(const char* scheme, const char* host, int port,
                  const char* path, const char* query, const char* fragment) {
    char buf[4096];
    int len = 0;

    if (scheme && scheme[0])
        len += snprintf(buf + len, sizeof(buf) - len, "%s://", scheme);
    if (host && host[0])
        len += snprintf(buf + len, sizeof(buf) - len, "%s", host);
    if (port > 0)
        len += snprintf(buf + len, sizeof(buf) - len, ":%d", port);
    if (path && path[0]) {
        if (path[0] != '/' && host && host[0])
            len += snprintf(buf + len, sizeof(buf) - len, "/");
        len += snprintf(buf + len, sizeof(buf) - len, "%s", path);
    }
    if (query && query[0])
        len += snprintf(buf + len, sizeof(buf) - len, "?%s", query);
    if (fragment && fragment[0])
        len += snprintf(buf + len, sizeof(buf) - len, "#%s", fragment);

    return strdup(buf);
}

/* ============================================================
 * Percent Encoding / Decoding (RFC 3986)
 * ============================================================ */

static int is_unreserved(unsigned char c) {
    return isalnum(c) || c == '-' || c == '_' || c == '.' || c == '~';
}

char* __url_encode(const char* input) {
    if (!input) return strdup("");
    size_t len = strlen(input);
    /* Worst case: every char becomes %XX → 3x */
    char* out = (char*)malloc(len * 3 + 1);
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)input[i];
        if (is_unreserved(c)) {
            out[j++] = c;
        } else {
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
    return 0;
}

char* __url_decode(const char* input) {
    if (!input) return strdup("");
    size_t len = strlen(input);
    char* out = (char*)malloc(len + 1);
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        if (input[i] == '%' && i + 2 < len && isxdigit(input[i+1]) && isxdigit(input[i+2])) {
            out[j++] = (char)((hex_val(input[i+1]) << 4) | hex_val(input[i+2]));
            i += 2;
        } else if (input[i] == '+') {
            out[j++] = ' ';
        } else {
            out[j++] = input[i];
        }
    }
    out[j] = '\0';
    return out;
}

/* ============================================================
 * Query String Helpers
 * ============================================================ */

/* Get a single value from a query string: "key1=val1&key2=val2" */
char* __url_query_get(const char* query, const char* key) {
    if (!query || !key) return strdup("");
    size_t klen = strlen(key);
    const char* p = query;

    while (*p) {
        /* Check if current position matches key= */
        if (strncmp(p, key, klen) == 0 && p[klen] == '=') {
            const char* val = p + klen + 1;
            const char* end = strchr(val, '&');
            if (!end) end = val + strlen(val);
            char* decoded_val = strndup_safe(val, end - val);
            char* result = __url_decode(decoded_val);
            free(decoded_val);
            return result;
        }
        /* Skip to next & */
        const char* amp = strchr(p, '&');
        if (!amp) break;
        p = amp + 1;
    }
    return strdup("");
}

/* Build a query string from parallel key/value arrays */
char* __url_query_build(const char** keys, const char** vals, int count) {
    if (count <= 0) return strdup("");
    char buf[8192];
    int len = 0;
    for (int i = 0; i < count; i++) {
        if (i > 0) buf[len++] = '&';
        char* ek = __url_encode(keys[i]);
        char* ev = __url_encode(vals[i]);
        len += snprintf(buf + len, sizeof(buf) - len, "%s=%s", ek, ev);
        free(ek);
        free(ev);
    }
    buf[len] = '\0';
    return strdup(buf);
}

/* ============================================================
 * URL Join (resolve relative against base)
 * ============================================================ */

char* __url_join(const char* base_str, const char* ref_str) {
    if (!base_str || !ref_str) return strdup(ref_str ? ref_str : "");

    /* If ref has scheme, it's absolute */
    if (strstr(ref_str, "://")) return strdup(ref_str);

    DesiUrl* base = __url_parse(base_str);

    if (ref_str[0] == '/') {
        /* Absolute path reference */
        char* result = __url_build(base->scheme, base->host, base->port,
                                    ref_str, "", "");
        __url_free(base);
        return result;
    }

    /* Relative path — merge with base path */
    char merged[4096];
    const char* last_slash = strrchr(base->path, '/');
    if (last_slash) {
        int prefix_len = (int)(last_slash - base->path + 1);
        snprintf(merged, sizeof(merged), "%.*s%s", prefix_len, base->path, ref_str);
    } else {
        snprintf(merged, sizeof(merged), "/%s", ref_str);
    }

    char* result = __url_build(base->scheme, base->host, base->port,
                                merged, "", "");
    __url_free(base);
    return result;
}
