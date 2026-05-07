/*
 * jwt.c — JSON Web Token (JWT) for Desi stdlib
 *
 * Implements JWT encode/decode with HMAC-SHA256 (HS256).
 * Uses existing Desi crypto (HMAC) and base64 modules.
 *
 * Public API:
 *   __jwt_encode(payload_json, secret)       → JWT string
 *   __jwt_decode(token, secret)              → payload JSON string (or "" on failure)
 *   __jwt_verify(token, secret)              → 1 if valid, 0 if not
 *   __jwt_get_payload(token)                 → payload JSON (no verification)
 *   __jwt_get_header(token)                  → header JSON
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <time.h>

/* We reuse the runtime's built-in HMAC and base64 */
extern char* __hash_hmac_sha256(const char* key, const char* data);
extern char* __base64_encode(const char* input);
extern char* __base64_decode(const char* input);

/* ============================================================
 * Base64URL helpers (JWT uses base64url, not standard base64)
 * ============================================================ */

static char* base64_to_base64url(const char* b64) {
    size_t len = strlen(b64);
    char* out = (char*)malloc(len + 1);
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        if (b64[i] == '+') out[j++] = '-';
        else if (b64[i] == '/') out[j++] = '_';
        else if (b64[i] == '=') continue; /* strip padding */
        else out[j++] = b64[i];
    }
    out[j] = '\0';
    return out;
}

static char* base64url_to_base64(const char* b64url) {
    size_t len = strlen(b64url);
    /* Add padding */
    int pad = (4 - (len % 4)) % 4;
    char* out = (char*)malloc(len + pad + 1);
    for (size_t i = 0; i < len; i++) {
        if (b64url[i] == '-') out[i] = '+';
        else if (b64url[i] == '_') out[i] = '/';
        else out[i] = b64url[i];
    }
    for (int i = 0; i < pad; i++) out[len + i] = '=';
    out[len + pad] = '\0';
    return out;
}

static char* base64url_encode(const char* data, size_t len) {
    /* Use runtime base64 encode */
    char* b64 = __base64_encode(data);
    char* b64url = base64_to_base64url(b64);
    free(b64);
    return b64url;
}

static char* base64url_decode(const char* b64url) {
    char* b64 = base64url_to_base64(b64url);
    char* decoded = __base64_decode(b64);
    free(b64);
    return decoded;
}

/* ============================================================
 * HMAC-SHA256 signature (hex → raw bytes → base64url)
 * ============================================================ */

static int hex_char_val(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return 0;
}

static char* sign_hs256(const char* input, const char* secret) {
    /* Get HMAC-SHA256 as hex string */
    char* hex = __hash_hmac_sha256(secret, input);
    if (!hex || !hex[0]) return strdup("");

    /* Convert hex to raw bytes */
    size_t hex_len = strlen(hex);
    size_t raw_len = hex_len / 2;
    char* raw = (char*)malloc(raw_len + 1);
    for (size_t i = 0; i < raw_len; i++) {
        raw[i] = (char)((hex_char_val(hex[i*2]) << 4) | hex_char_val(hex[i*2+1]));
    }
    raw[raw_len] = '\0';
    free(hex);

    /* Base64url encode the raw bytes */
    char* sig = base64url_encode(raw, raw_len);
    free(raw);
    return sig;
}

/* ============================================================
 * Constant-time string comparison
 * ============================================================ */

static int secure_compare(const char* a, const char* b) {
    size_t alen = strlen(a), blen = strlen(b);
    if (alen != blen) return 0;
    unsigned char result = 0;
    for (size_t i = 0; i < alen; i++) {
        result |= (unsigned char)a[i] ^ (unsigned char)b[i];
    }
    return result == 0;
}

/* ============================================================
 * Public API
 * ============================================================ */

/* Forward declarations */
char* __jwt_get_payload(const char* token);

/* Standard JWT header for HS256 */
static const char* JWT_HEADER = "{\"alg\":\"HS256\",\"typ\":\"JWT\"}";

char* __jwt_encode(const char* payload_json, const char* secret) {
    if (!payload_json || !secret) return strdup("");

    /* Encode header */
    char* header_b64 = base64url_encode(JWT_HEADER, strlen(JWT_HEADER));

    /* Encode payload */
    char* payload_b64 = base64url_encode(payload_json, strlen(payload_json));

    /* Create signing input: header.payload */
    size_t input_len = strlen(header_b64) + 1 + strlen(payload_b64);
    char* signing_input = (char*)malloc(input_len + 1);
    snprintf(signing_input, input_len + 1, "%s.%s", header_b64, payload_b64);

    /* Sign */
    char* signature = sign_hs256(signing_input, secret);

    /* Assemble: header.payload.signature */
    size_t total_len = input_len + 1 + strlen(signature);
    char* token = (char*)malloc(total_len + 1);
    snprintf(token, total_len + 1, "%s.%s", signing_input, signature);

    free(header_b64);
    free(payload_b64);
    free(signing_input);
    free(signature);
    return token;
}

int32_t __jwt_verify(const char* token, const char* secret) {
    if (!token || !secret) return 0;

    /* Find the two dots */
    const char* dot1 = strchr(token, '.');
    if (!dot1) return 0;
    const char* dot2 = strchr(dot1 + 1, '.');
    if (!dot2) return 0;

    /* Extract signing input (header.payload) */
    size_t input_len = dot2 - token;
    char* signing_input = (char*)malloc(input_len + 1);
    strncpy(signing_input, token, input_len);
    signing_input[input_len] = '\0';

    /* Expected signature */
    const char* given_sig = dot2 + 1;

    /* Compute expected signature */
    char* expected_sig = sign_hs256(signing_input, secret);
    free(signing_input);

    int valid = secure_compare(expected_sig, given_sig);
    free(expected_sig);

    return valid;
}

char* __jwt_decode(const char* token, const char* secret) {
    if (!__jwt_verify(token, secret)) return strdup("");
    return __jwt_get_payload(token);
}

char* __jwt_get_payload(const char* token) {
    if (!token) return strdup("");
    const char* dot1 = strchr(token, '.');
    if (!dot1) return strdup("");
    const char* dot2 = strchr(dot1 + 1, '.');
    if (!dot2) return strdup("");

    /* Extract payload part */
    size_t plen = dot2 - (dot1 + 1);
    char* payload_b64 = (char*)malloc(plen + 1);
    strncpy(payload_b64, dot1 + 1, plen);
    payload_b64[plen] = '\0';

    char* payload = base64url_decode(payload_b64);
    free(payload_b64);
    return payload;
}

char* __jwt_get_header(const char* token) {
    if (!token) return strdup("");
    const char* dot1 = strchr(token, '.');
    if (!dot1) return strdup("");

    size_t hlen = dot1 - token;
    char* header_b64 = (char*)malloc(hlen + 1);
    strncpy(header_b64, token, hlen);
    header_b64[hlen] = '\0';

    char* header = base64url_decode(header_b64);
    free(header_b64);
    return header;
}
