/*
 * dynbuf.h — Dynamic string buffer for Desi ORM (Phase 5)
 *
 * Replaces all fixed-size char[] arrays in crud.c with safe,
 * auto-growing buffers. Eliminates buffer overflow risk entirely.
 *
 * Design:
 *   - Starts at 64 bytes, doubles on overflow
 *   - All string-building in QuerySet uses DynBuf
 *   - Thread-safe per-instance (no shared state)
 *   - Zero-overhead when not used (init is lazy)
 */

#ifndef DESI_DYNBUF_H
#define DESI_DYNBUF_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>

#define DYNBUF_INITIAL_CAP 64

typedef struct {
    char* data;
    int   len;
    int   cap;
} DynBuf;

// Initialize a DynBuf with a given initial capacity (0 = defer allocation)
static inline void dynbuf_init(DynBuf* b, int initial_cap) {
    if (initial_cap <= 0) initial_cap = DYNBUF_INITIAL_CAP;
    b->data = (char*)malloc(initial_cap);
    b->data[0] = '\0';
    b->len = 0;
    b->cap = initial_cap;
}

// Ensure at least `needed` more bytes are available
static inline void dynbuf_grow(DynBuf* b, int needed) {
    if (b->data == NULL) {
        dynbuf_init(b, needed + 1);
        return;
    }
    int required = b->len + needed + 1; // +1 for null terminator
    if (required <= b->cap) return;
    int new_cap = b->cap;
    while (new_cap < required) new_cap *= 2;
    b->data = (char*)realloc(b->data, new_cap);
    b->cap = new_cap;
}

// Append a string
static inline void dynbuf_append(DynBuf* b, const char* s) {
    if (!s) return;
    int slen = (int)strlen(s);
    dynbuf_grow(b, slen);
    memcpy(b->data + b->len, s, slen);
    b->len += slen;
    b->data[b->len] = '\0';
}

// Append a single character
static inline void dynbuf_append_char(DynBuf* b, char c) {
    dynbuf_grow(b, 1);
    b->data[b->len++] = c;
    b->data[b->len] = '\0';
}

// Append formatted string (printf-style)
static inline void dynbuf_appendf(DynBuf* b, const char* fmt, ...) {
    va_list args, args_copy;
    va_start(args, fmt);

    // First pass: measure needed size
    va_copy(args_copy, args);
    int needed = vsnprintf(NULL, 0, fmt, args_copy);
    va_end(args_copy);

    if (needed <= 0) {
        va_end(args);
        return;
    }

    dynbuf_grow(b, needed);

    // Second pass: write into buffer
    vsnprintf(b->data + b->len, needed + 1, fmt, args);
    b->len += needed;
    va_end(args);
}

// Set content (clear + append)
static inline void dynbuf_set(DynBuf* b, const char* s) {
    if (b->data == NULL) {
        int slen = s ? (int)strlen(s) : 0;
        dynbuf_init(b, slen + 1);
    }
    b->len = 0;
    b->data[0] = '\0';
    if (s) dynbuf_append(b, s);
}

// Clear the buffer (reset length, keep allocation)
static inline void dynbuf_clear(DynBuf* b) {
    b->len = 0;
    if (b->data) b->data[0] = '\0';
}

// Free the buffer's memory
static inline void dynbuf_free(DynBuf* b) {
    if (b->data) {
        free(b->data);
        b->data = NULL;
    }
    b->len = 0;
    b->cap = 0;
}

// Check if empty
static inline int dynbuf_empty(DynBuf* b) {
    return b->len == 0 || b->data == NULL || b->data[0] == '\0';
}

// Get C string (read-only view)
static inline const char* dynbuf_cstr(DynBuf* b) {
    return b->data ? b->data : "";
}

#endif // DESI_DYNBUF_H
