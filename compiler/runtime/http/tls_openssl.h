/*
 * tls_openssl.h — Linux TLS via OpenSSL
 *
 * OpenSSL is preinstalled on virtually all Linux distros.
 * Links with -lssl -lcrypto.
 */
#ifndef DESI_TLS_OPENSSL_H
#define DESI_TLS_OPENSSL_H

#include <openssl/ssl.h>
#include <openssl/err.h>

typedef struct {
    SSL     *ssl;
    SSL_CTX *ctx;
} OpenSSLState;

static int _openssl_initialized = 0;

static int tls_handshake(Connection *c, const char *host) {
    if (!_openssl_initialized) {
        SSL_library_init();
        SSL_load_error_strings();
        OpenSSL_add_all_algorithms();
        _openssl_initialized = 1;
    }

    OpenSSLState *st = (OpenSSLState *)calloc(1, sizeof(OpenSSLState));
    if (!st) return -1;

    const SSL_METHOD *method = TLS_client_method();
    st->ctx = SSL_CTX_new(method);
    if (!st->ctx) { free(st); return -1; }

    /* Use system certificate store */
    SSL_CTX_set_default_verify_paths(st->ctx);

    st->ssl = SSL_new(st->ctx);
    if (!st->ssl) {
        SSL_CTX_free(st->ctx);
        free(st);
        return -1;
    }

    SSL_set_fd(st->ssl, (int)c->fd);
    /* SNI (Server Name Indication) */
    SSL_set_tlsext_host_name(st->ssl, host);

    if (SSL_connect(st->ssl) <= 0) {
        SSL_free(st->ssl);
        SSL_CTX_free(st->ctx);
        free(st);
        return -1;
    }

    c->tls_state = st;
    c->use_tls = 1;
    return 0;
}

static ssize_t tls_write(Connection *c, const void *buf, size_t len) {
    OpenSSLState *st = (OpenSSLState *)c->tls_state;
    return (ssize_t)SSL_write(st->ssl, buf, (int)len);
}

static ssize_t tls_read(Connection *c, void *buf, size_t len) {
    OpenSSLState *st = (OpenSSLState *)c->tls_state;
    int n = SSL_read(st->ssl, buf, (int)len);
    if (n <= 0) {
        int err = SSL_get_error(st->ssl, n);
        if (err == SSL_ERROR_ZERO_RETURN) return 0;
        return -1;
    }
    return (ssize_t)n;
}

static void tls_close(Connection *c) {
    if (c->tls_state) {
        OpenSSLState *st = (OpenSSLState *)c->tls_state;
        SSL_shutdown(st->ssl);
        SSL_free(st->ssl);
        SSL_CTX_free(st->ctx);
        free(st);
        c->tls_state = NULL;
    }
}

#endif /* DESI_TLS_OPENSSL_H */
