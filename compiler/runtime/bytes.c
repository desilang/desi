/*
 * bytes.c — Length-prefixed binary data type for Desi
 *
 * Provides a bytes type that stores data + length, avoiding null-byte
 * truncation that would occur with C strings. The DesiBytes struct is
 * allocated on the heap and passed as an opaque ptr to Desi code.
 *
 * API:
 *   __bytes_new(data, len)       → create from raw data (copies data)
 *   __bytes_from_str(str)        → convert string to bytes
 *   __bytes_to_str(bytes)        → convert bytes to string (may truncate at \0)
 *   __bytes_len(bytes)           → get byte count
 *   __bytes_get(bytes, i)        → get byte at index
 *   __bytes_set(bytes, i, val)   → set byte at index
 *   __bytes_slice(bytes, s, e)   → slice [start, end)
 *   __bytes_concat(a, b)         → concatenate
 *   __bytes_equal(a, b)          → compare
 *   __bytes_free(bytes)          → free
 *   __bytes_to_hex(bytes)        → hex string
 *   __bytes_from_hex(str)        → bytes from hex
 */

#include <stdlib.h>
#include <string.h>
#include <stdio.h>

typedef struct {
    unsigned char* data;
    size_t len;
} DesiBytes;

/* Create bytes from raw data (copies) */
DesiBytes* __bytes_new(const unsigned char* data, int32_t len) {
    DesiBytes* b = (DesiBytes*)calloc(1, sizeof(DesiBytes));
    if (!b) return NULL;
    if (len > 0 && data) {
        b->data = (unsigned char*)malloc(len);
        if (!b->data) { free(b); return NULL; }
        memcpy(b->data, data, len);
        b->len = len;
    } else {
        b->data = NULL;
        b->len = 0;
    }
    return b;
}

/* Create bytes from a Desi string (strlen-based) */
DesiBytes* __bytes_from_str(const char* s) {
    if (!s) return __bytes_new(NULL, 0);
    size_t len = strlen(s);
    return __bytes_new((const unsigned char*)s, (int32_t)len);
}

/* Convert bytes to null-terminated string (may truncate if data has \0) */
const char* __bytes_to_str(DesiBytes* b) {
    if (!b || !b->data || b->len == 0) return strdup("");
    char* s = (char*)malloc(b->len + 1);
    if (!s) return strdup("");
    memcpy(s, b->data, b->len);
    s[b->len] = '\0';
    return s;
}

/* Get length */
int32_t __bytes_len(DesiBytes* b) {
    return b ? (int32_t)b->len : 0;
}

/* Get byte at index (returns -1 for out of bounds) */
int32_t __bytes_get(DesiBytes* b, int32_t index) {
    if (!b || index < 0 || (size_t)index >= b->len) return -1;
    return (int32_t)b->data[index];
}

/* Set byte at index */
void __bytes_set(DesiBytes* b, int32_t index, int32_t value) {
    if (!b || index < 0 || (size_t)index >= b->len) return;
    b->data[index] = (unsigned char)(value & 0xFF);
}

/* Slice bytes [start, end) — returns new bytes */
DesiBytes* __bytes_slice(DesiBytes* b, int32_t start, int32_t end) {
    if (!b || !b->data) return __bytes_new(NULL, 0);
    if (start < 0) start = 0;
    if (end > (int32_t)b->len) end = (int32_t)b->len;
    if (start >= end) return __bytes_new(NULL, 0);
    return __bytes_new(b->data + start, end - start);
}

/* Concatenate two bytes — returns new bytes */
DesiBytes* __bytes_concat(DesiBytes* a, DesiBytes* b) {
    size_t a_len = (a && a->data) ? a->len : 0;
    size_t b_len = (b && b->data) ? b->len : 0;
    size_t total = a_len + b_len;

    DesiBytes* result = (DesiBytes*)calloc(1, sizeof(DesiBytes));
    if (!result) return NULL;

    if (total == 0) {
        result->data = NULL;
        result->len = 0;
        return result;
    }

    result->data = (unsigned char*)malloc(total);
    if (!result->data) { free(result); return NULL; }
    if (a_len > 0) memcpy(result->data, a->data, a_len);
    if (b_len > 0) memcpy(result->data + a_len, b->data, b_len);
    result->len = total;
    return result;
}

/* Compare two bytes for equality */
int __bytes_equal(DesiBytes* a, DesiBytes* b) {
    if (a == b) return 1;
    if (!a || !b) return 0;
    if (a->len != b->len) return 0;
    if (a->len == 0) return 1;
    return memcmp(a->data, b->data, a->len) == 0;
}

/* Free bytes */
void __bytes_free(DesiBytes* b) {
    if (!b) return;
    free(b->data);
    free(b);
}

/* Convert bytes to hex string */
const char* __bytes_to_hex(DesiBytes* b) {
    if (!b || !b->data || b->len == 0) return strdup("");
    char* hex = (char*)malloc(b->len * 2 + 1);
    if (!hex) return strdup("");
    for (size_t i = 0; i < b->len; i++) {
        sprintf(hex + i * 2, "%02x", b->data[i]);
    }
    hex[b->len * 2] = '\0';
    return hex;
}

/* Create bytes from hex string */
DesiBytes* __bytes_from_hex(const char* hex) {
    if (!hex) return __bytes_new(NULL, 0);
    size_t hex_len = strlen(hex);
    if (hex_len % 2 != 0) return __bytes_new(NULL, 0);
    size_t byte_len = hex_len / 2;

    unsigned char* data = (unsigned char*)malloc(byte_len);
    if (!data) return __bytes_new(NULL, 0);

    for (size_t i = 0; i < byte_len; i++) {
        unsigned int val;
        if (sscanf(hex + i * 2, "%2x", &val) != 1) {
            free(data);
            return __bytes_new(NULL, 0);
        }
        data[i] = (unsigned char)val;
    }

    DesiBytes* b = (DesiBytes*)calloc(1, sizeof(DesiBytes));
    if (!b) { free(data); return NULL; }
    b->data = data;
    b->len = byte_len;
    return b;
}

/* Create bytes of given size filled with a value */
DesiBytes* __bytes_repeat(int32_t value, int32_t count) {
    if (count <= 0) return __bytes_new(NULL, 0);
    DesiBytes* b = (DesiBytes*)calloc(1, sizeof(DesiBytes));
    if (!b) return NULL;
    b->data = (unsigned char*)malloc(count);
    if (!b->data) { free(b); return NULL; }
    memset(b->data, value & 0xFF, count);
    b->len = count;
    return b;
}

/* Find first occurrence of byte value, returns -1 if not found */
int32_t __bytes_index_of(DesiBytes* b, int32_t value) {
    if (!b || !b->data) return -1;
    unsigned char needle = (unsigned char)(value & 0xFF);
    for (size_t i = 0; i < b->len; i++) {
        if (b->data[i] == needle) return (int32_t)i;
    }
    return -1;
}
