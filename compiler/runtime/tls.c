/*
 * tls.c — TLS implementation for Desi runtime (OpenSSL)
 *
 * Provides server-side TLS for HTTPS and WSS.
 * Compiled only when OpenSSL headers are available.
 */

#include <stdio.h>
#include <string.h>
#include <errno.h>

/* Detect OpenSSL availability at compile time */
#if __has_include(<openssl/ssl.h>)
  #define DESI_HAS_OPENSSL 1
  #include <openssl/ssl.h>
  #include <openssl/err.h>
#else
  #define DESI_HAS_OPENSSL 0
#endif

#include "tls.h"

#if DESI_HAS_OPENSSL

/* One-time OpenSSL initialization (thread-safe via static flag) */
static int _tls_initialized = 0;
static void tls_init_once(void) {
    if (!_tls_initialized) {
        /* Modern OpenSSL 3.x init (also works with 1.1.x) */
        OPENSSL_init_ssl(OPENSSL_INIT_LOAD_SSL_STRINGS | OPENSSL_INIT_LOAD_CRYPTO_STRINGS, NULL);
        _tls_initialized = 1;
    }
}

/* ============================================================
 * TLS Context (per-server, holds cert + key)
 * ============================================================ */

DESI_SSL_CTX* desi_tls_ctx_new(const char* cert_path, const char* key_path) {
    tls_init_once();

    const SSL_METHOD* method = TLS_server_method();
    SSL_CTX* ctx = SSL_CTX_new(method);
    if (!ctx) {
        fprintf(stderr, "[tls] Failed to create SSL_CTX\n");
        ERR_print_errors_fp(stderr);
        return NULL;
    }

    /* Set minimum TLS version to 1.2 (no SSLv3/TLS1.0/1.1) */
    SSL_CTX_set_min_proto_version(ctx, TLS1_2_VERSION);

    /* Load server certificate */
    if (SSL_CTX_use_certificate_file(ctx, cert_path, SSL_FILETYPE_PEM) <= 0) {
        fprintf(stderr, "[tls] Failed to load certificate: %s\n", cert_path);
        ERR_print_errors_fp(stderr);
        SSL_CTX_free(ctx);
        return NULL;
    }

    /* Load private key */
    if (SSL_CTX_use_PrivateKey_file(ctx, key_path, SSL_FILETYPE_PEM) <= 0) {
        fprintf(stderr, "[tls] Failed to load private key: %s\n", key_path);
        ERR_print_errors_fp(stderr);
        SSL_CTX_free(ctx);
        return NULL;
    }

    /* Verify private key matches certificate */
    if (!SSL_CTX_check_private_key(ctx)) {
        fprintf(stderr, "[tls] Private key does not match certificate\n");
        SSL_CTX_free(ctx);
        return NULL;
    }

    return (DESI_SSL_CTX*)ctx;
}

/* ============================================================
 * Per-Connection TLS Handshake
 * ============================================================ */

DESI_SSL* desi_tls_accept(DESI_SSL_CTX* ctx, int fd) {
    if (!ctx || fd < 0) return NULL;

    SSL* ssl = SSL_new((SSL_CTX*)ctx);
    if (!ssl) {
        fprintf(stderr, "[tls] Failed to create SSL object\n");
        ERR_print_errors_fp(stderr);
        return NULL;
    }

    SSL_set_fd(ssl, fd);

    int ret = SSL_accept(ssl);
    if (ret <= 0) {
        int err = SSL_get_error(ssl, ret);
        /* Log real TLS protocol errors (bad version, cipher mismatch, etc.)
         * but suppress connection-reset noise from plain-HTTP probes.
         * SSL_ERROR_SYSCALL with errno=0 → client sent non-TLS data (harmless)
         * SSL_ERROR_SSL → actual TLS protocol error (worth logging) */
        if (err == SSL_ERROR_SSL) {
            unsigned long ossl_err = ERR_get_error();
            char errbuf[256];
            ERR_error_string_n(ossl_err, errbuf, sizeof(errbuf));
            fprintf(stderr, "[tls] Handshake error (fd=%d): %s\n", fd, errbuf);
        }
        SSL_free(ssl);
        return NULL;
    }

    return (DESI_SSL*)ssl;
}

/* ============================================================
 * TLS Send / Recv
 * ============================================================ */

ssize_t desi_tls_send(DESI_SSL* ssl, const void* buf, size_t len) {
    if (!ssl || !buf) return -1;
    int ret = SSL_write((SSL*)ssl, buf, (int)len);
    if (ret <= 0) {
        int err = SSL_get_error((SSL*)ssl, ret);
        if (err == SSL_ERROR_WANT_WRITE) return 0; /* retry */
        return -1;
    }
    return (ssize_t)ret;
}

ssize_t desi_tls_recv(DESI_SSL* ssl, void* buf, size_t len) {
    if (!ssl || !buf) return -1;
    int ret = SSL_read((SSL*)ssl, buf, (int)len);
    if (ret <= 0) {
        int err = SSL_get_error((SSL*)ssl, ret);
        if (err == SSL_ERROR_ZERO_RETURN) return 0; /* clean shutdown */
        if (err == SSL_ERROR_WANT_READ) return 0;   /* retry */
        return -1;
    }
    return (ssize_t)ret;
}

/* ============================================================
 * TLS Cleanup
 * ============================================================ */

void desi_tls_close(DESI_SSL* ssl) {
    if (!ssl) return;
    SSL_shutdown((SSL*)ssl);
    SSL_free((SSL*)ssl);
}

void desi_tls_ctx_free(DESI_SSL_CTX* ctx) {
    if (!ctx) return;
    SSL_CTX_free((SSL_CTX*)ctx);
}

#else /* !DESI_HAS_OPENSSL — stub implementations */

DESI_SSL_CTX* desi_tls_ctx_new(const char* cert_path, const char* key_path) {
    (void)cert_path; (void)key_path;
    fprintf(stderr, "[tls] OpenSSL not available — TLS disabled\n");
    return NULL;
}

DESI_SSL* desi_tls_accept(DESI_SSL_CTX* ctx, int fd) {
    (void)ctx; (void)fd;
    return NULL;
}

ssize_t desi_tls_send(DESI_SSL* ssl, const void* buf, size_t len) {
    (void)ssl; (void)buf; (void)len;
    return -1;
}

ssize_t desi_tls_recv(DESI_SSL* ssl, void* buf, size_t len) {
    (void)ssl; (void)buf; (void)len;
    return -1;
}

void desi_tls_close(DESI_SSL* ssl) { (void)ssl; }
void desi_tls_ctx_free(DESI_SSL_CTX* ctx) { (void)ctx; }

#endif /* DESI_HAS_OPENSSL */
