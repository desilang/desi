// base64.c - Base64 encoding/decoding module
// RFC 4648 compliant. Pure C, no external deps.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

static const char b64_table[] =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

static const char b64url_table[] =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

// ============================================================
// Encode
// ============================================================

static char* base64_encode_with_table(const char* input, const char* table, int pad) {
    if (!input) return strdup("");
    size_t in_len = strlen(input);
    size_t out_len = 4 * ((in_len + 2) / 3);
    if (!pad) {
        // Without padding, calculate exact size
        out_len = (in_len * 4 + 2) / 3;
    }

    char* output = (char*)malloc(out_len + 1);
    if (!output) return strdup("");

    size_t i, j;
    for (i = 0, j = 0; i < in_len; ) {
        uint32_t a = (i < in_len) ? (unsigned char)input[i++] : 0;
        uint32_t b = (i < in_len) ? (unsigned char)input[i++] : 0;
        uint32_t c = (i < in_len) ? (unsigned char)input[i++] : 0;

        uint32_t triple = (a << 16) | (b << 8) | c;

        output[j++] = table[(triple >> 18) & 0x3F];
        output[j++] = table[(triple >> 12) & 0x3F];
        output[j++] = table[(triple >> 6) & 0x3F];
        output[j++] = table[triple & 0x3F];
    }

    // Add padding or trim
    size_t mod = in_len % 3;
    if (pad) {
        if (mod == 1) {
            output[j - 1] = '=';
            output[j - 2] = '=';
        } else if (mod == 2) {
            output[j - 1] = '=';
        }
        output[j] = '\0';
    } else {
        // Trim excess for no-pad
        if (mod == 1) j -= 2;
        else if (mod == 2) j -= 1;
        output[j] = '\0';
    }

    return output;
}

// ============================================================
// Decode
// ============================================================

static int b64_decode_char(char c, const char* table) {
    const char* p = strchr(table, c);
    if (p) return (int)(p - table);
    return -1;
}

static char* base64_decode_with_table(const char* input, const char* table) {
    if (!input) return strdup("");
    size_t in_len = strlen(input);
    if (in_len == 0) return strdup("");

    // Pad input to multiple of 4 if needed (for URL-safe without padding)
    size_t padded_len = in_len;
    while (padded_len % 4 != 0) padded_len++;

    char* padded = (char*)malloc(padded_len + 1);
    if (!padded) return strdup("");
    memcpy(padded, input, in_len);
    for (size_t i = in_len; i < padded_len; i++) padded[i] = '=';
    padded[padded_len] = '\0';

    size_t out_len = (padded_len / 4) * 3;
    char* output = (char*)malloc(out_len + 1);
    if (!output) { free(padded); return strdup(""); }

    size_t j = 0;
    for (size_t i = 0; i < padded_len; i += 4) {
        int a = (padded[i] == '=') ? 0 : b64_decode_char(padded[i], table);
        int b = (padded[i+1] == '=') ? 0 : b64_decode_char(padded[i+1], table);
        int c = (padded[i+2] == '=') ? 0 : b64_decode_char(padded[i+2], table);
        int d = (padded[i+3] == '=') ? 0 : b64_decode_char(padded[i+3], table);

        if (a < 0 || b < 0 || c < 0 || d < 0) {
            // Invalid character
            free(padded);
            output[j] = '\0';
            return output;
        }

        uint32_t triple = ((uint32_t)a << 18) | ((uint32_t)b << 12) |
                           ((uint32_t)c << 6) | (uint32_t)d;

        output[j++] = (char)((triple >> 16) & 0xFF);
        if (padded[i+2] != '=') output[j++] = (char)((triple >> 8) & 0xFF);
        if (padded[i+3] != '=') output[j++] = (char)(triple & 0xFF);
    }

    output[j] = '\0';
    free(padded);
    return output;
}

// ============================================================
// Public API
// ============================================================

char* __base64_encode(const char* input) {
    return base64_encode_with_table(input, b64_table, 1);
}

char* __base64_decode(const char* input) {
    return base64_decode_with_table(input, b64_table);
}

char* __base64_url_encode(const char* input) {
    return base64_encode_with_table(input, b64url_table, 0);
}

char* __base64_url_decode(const char* input) {
    return base64_decode_with_table(input, b64url_table);
}
