// encoding.c — Hex encoding/decoding for Desi stdlib
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

// Hex encode: bytes (str) → hex string
char* __encoding_hex_encode(const char* input) {
    if (!input) return strdup("");
    size_t len = strlen(input);
    char* out = (char*)malloc(len * 2 + 1);
    if (!out) return strdup("");
    for (size_t i = 0; i < len; i++) {
        sprintf(out + i * 2, "%02x", (unsigned char)input[i]);
    }
    out[len * 2] = '\0';
    return out;
}

// Hex decode: hex string → bytes (str)
char* __encoding_hex_decode(const char* input) {
    if (!input) return strdup("");
    size_t len = strlen(input);
    if (len % 2 != 0) return strdup(""); // invalid hex
    size_t out_len = len / 2;
    char* out = (char*)malloc(out_len + 1);
    if (!out) return strdup("");
    for (size_t i = 0; i < out_len; i++) {
        char hi = input[i * 2];
        char lo = input[i * 2 + 1];
        if (!isxdigit(hi) || !isxdigit(lo)) {
            free(out);
            return strdup(""); // invalid hex char
        }
        unsigned int byte;
        sscanf(input + i * 2, "%2x", &byte);
        out[i] = (char)byte;
    }
    out[out_len] = '\0';
    return out;
}

// Hex encode uppercase
char* __encoding_hex_encode_upper(const char* input) {
    if (!input) return strdup("");
    size_t len = strlen(input);
    char* out = (char*)malloc(len * 2 + 1);
    if (!out) return strdup("");
    for (size_t i = 0; i < len; i++) {
        sprintf(out + i * 2, "%02X", (unsigned char)input[i]);
    }
    out[len * 2] = '\0';
    return out;
}
