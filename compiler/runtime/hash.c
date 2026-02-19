// hash.c - Cryptographic hashing module
// Uses CommonCrypto on macOS, OpenSSL on Linux.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#if defined(__APPLE__)
#define COMMON_DIGEST_FOR_OPENSSL
#include <CommonCrypto/CommonDigest.h>
#include <CommonCrypto/CommonHmac.h>
#else
// Minimal SHA-256 implementation for portability
// (On Linux with OpenSSL, replace this with openssl/sha.h)
#include <openssl/md5.h>
#include <openssl/sha.h>
#include <openssl/hmac.h>
#endif

static char* to_hex(const unsigned char* data, size_t len) {
    char* hex = (char*)malloc(2 * len + 1);
    if (!hex) return strdup("");
    for (size_t i = 0; i < len; i++) {
        sprintf(hex + 2 * i, "%02x", data[i]);
    }
    hex[2 * len] = '\0';
    return hex;
}

// ============================================================
// Hash functions
// ============================================================

char* __hash_md5(const char* input) {
    if (!input) return strdup("");
    unsigned char digest[CC_MD5_DIGEST_LENGTH];
    CC_MD5(input, (CC_LONG)strlen(input), digest);
    return to_hex(digest, CC_MD5_DIGEST_LENGTH);
}

char* __hash_sha256(const char* input) {
    if (!input) return strdup("");
    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
    CC_SHA256(input, (CC_LONG)strlen(input), digest);
    return to_hex(digest, CC_SHA256_DIGEST_LENGTH);
}

char* __hash_sha512(const char* input) {
    if (!input) return strdup("");
    unsigned char digest[CC_SHA512_DIGEST_LENGTH];
    CC_SHA512(input, (CC_LONG)strlen(input), digest);
    return to_hex(digest, CC_SHA512_DIGEST_LENGTH);
}

// ============================================================
// HMAC
// ============================================================

char* __hash_hmac_sha256(const char* key, const char* msg) {
    if (!key || !msg) return strdup("");
    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
    CCHmac(kCCHmacAlgSHA256, key, strlen(key), msg, strlen(msg), digest);
    return to_hex(digest, CC_SHA256_DIGEST_LENGTH);
}

// ============================================================
// File hashing
// ============================================================

char* __hash_file(const char* path) {
    if (!path) return strdup("");
    FILE* f = fopen(path, "rb");
    if (!f) return strdup("");

    CC_SHA256_CTX ctx;
    CC_SHA256_Init(&ctx);

    unsigned char buf[8192];
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), f)) > 0) {
        CC_SHA256_Update(&ctx, buf, (CC_LONG)n);
    }
    fclose(f);

    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
    CC_SHA256_Final(digest, &ctx);
    return to_hex(digest, CC_SHA256_DIGEST_LENGTH);
}
