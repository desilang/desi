/*
 * crypto.c — Cryptographic utilities for Desi
 *
 * Self-contained implementations: SHA-256, HMAC-SHA-256, constant-time compare.
 * No external dependencies (no OpenSSL required).
 * SHA-256 and HMAC reuse the implementations from hash.c.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <time.h>

// Forward declarations from hash.c
extern char* __hash_sha256(const char* input);
extern char* __hash_sha512(const char* input);
extern char* __hash_md5(const char* input);
extern char* __hash_sha1(const char* input);
extern char* __hash_hmac_sha256(const char* key, const char* msg);

// Constant-time string comparison (prevents timing attacks)
int32_t __crypto_constant_time_equal(const char* a, const char* b) {
    if (!a || !b) return 0;
    size_t la = strlen(a), lb = strlen(b);
    if (la != lb) return 0;
    
    volatile uint8_t diff = 0;
    for (size_t i = 0; i < la; i++) {
        diff |= (uint8_t)((unsigned char)a[i] ^ (unsigned char)b[i]);
    }
    return diff == 0 ? 1 : 0;
}

// Generate a random hex token of given byte length
// (e.g. token_hex(16) -> 32-char hex string)
const char* __crypto_token_hex(int32_t nbytes) {
    if (nbytes <= 0) nbytes = 32;
    
    // Seed from time + address entropy (not cryptographic, but usable)
    static int seeded = 0;
    if (!seeded) {
        srand((unsigned)(time(NULL) ^ (uintptr_t)&seeded));
        seeded = 1;
    }
    
    char* hex = (char*)malloc(nbytes * 2 + 1);
    for (int i = 0; i < nbytes; i++) {
        sprintf(hex + i * 2, "%02x", rand() & 0xFF);
    }
    hex[nbytes * 2] = '\0';
    return hex;
}

// Generate random bytes as a hex string (alias for token_hex)
const char* __crypto_random_bytes_hex(int32_t nbytes) {
    return __crypto_token_hex(nbytes);
}

// Simple password hashing: SHA-256(salt + password)
// Returns "salt$hash" format
const char* __crypto_hash_password(const char* password) {
    if (!password) return strdup("");
    
    // Generate 16-byte salt
    const char* salt = __crypto_token_hex(16);
    
    // Concatenate salt + password
    size_t slen = strlen(salt);
    size_t plen = strlen(password);
    char* combined = (char*)malloc(slen + plen + 1);
    memcpy(combined, salt, slen);
    memcpy(combined + slen, password, plen);
    combined[slen + plen] = '\0';
    
    // Hash
    char* hash = __hash_sha256(combined);
    free(combined);
    
    // Format: salt$hash
    char* result = (char*)malloc(slen + 1 + strlen(hash) + 1);
    sprintf(result, "%s$%s", salt, hash);
    
    return result;
}

// Verify password against "salt$hash" format
int32_t __crypto_verify_password(const char* password, const char* stored) {
    if (!password || !stored) return 0;
    
    // Split stored at '$'
    const char* dollar = strchr(stored, '$');
    if (!dollar) return 0;
    
    size_t slen = (size_t)(dollar - stored);
    char* salt = (char*)malloc(slen + 1);
    memcpy(salt, stored, slen);
    salt[slen] = '\0';
    
    const char* expected_hash = dollar + 1;
    
    // Rebuild: SHA-256(salt + password)
    size_t plen = strlen(password);
    char* combined = (char*)malloc(slen + plen + 1);
    memcpy(combined, salt, slen);
    memcpy(combined + slen, password, plen);
    combined[slen + plen] = '\0';
    
    char* actual_hash = __hash_sha256(combined);
    free(combined);
    free(salt);
    
    // Constant-time compare
    int32_t result = __crypto_constant_time_equal(actual_hash, expected_hash);
    return result;
}
