/*
 * tls.h — TLS abstraction layer for Desi runtime
 *
 * Wraps OpenSSL for HTTPS/WSS support.
 * Guarded by DESI_HAS_OPENSSL — graceful no-op if unavailable.
 *
 * Public API:
 *   desi_tls_ctx_new(cert, key) → SSL_CTX*
 *   desi_tls_accept(ctx, fd)    → SSL*
 *   desi_tls_send(ssl, buf, len)
 *   desi_tls_recv(ssl, buf, len)
 *   desi_tls_close(ssl)
 *
 * Macros:
 *   DESI_SEND(fd, ssl, buf, len) — auto-dispatch TLS or raw
 *   DESI_RECV(fd, ssl, buf, len) — auto-dispatch TLS or raw
 */

#ifndef DESI_TLS_H
#define DESI_TLS_H

#include <stddef.h>
#include <sys/types.h>

/* Forward-declare SSL types to avoid requiring <openssl/ssl.h> in all files */
typedef struct ssl_ctx_st DESI_SSL_CTX;
typedef struct ssl_st     DESI_SSL;

/* ============================================================
 * TLS Context & Connection Lifecycle
 * ============================================================ */

/*
 * Create a TLS server context with the given certificate and private key.
 * Returns NULL on failure (logged to stderr).
 */
DESI_SSL_CTX* desi_tls_ctx_new(const char* cert_path, const char* key_path);

/*
 * Perform TLS handshake on an accepted socket fd.
 * Returns an SSL* on success, NULL on handshake failure.
 */
DESI_SSL* desi_tls_accept(DESI_SSL_CTX* ctx, int fd);

/*
 * Send data over a TLS connection.
 * Returns bytes written, or -1 on error.
 */
ssize_t desi_tls_send(DESI_SSL* ssl, const void* buf, size_t len);

/*
 * Receive data from a TLS connection.
 * Returns bytes read, 0 on clean shutdown, or -1 on error.
 */
ssize_t desi_tls_recv(DESI_SSL* ssl, void* buf, size_t len);

/*
 * Gracefully close a TLS connection (SSL_shutdown + SSL_free).
 */
void desi_tls_close(DESI_SSL* ssl);

/*
 * Free a TLS server context.
 */
void desi_tls_ctx_free(DESI_SSL_CTX* ctx);


/* ============================================================
 * Auto-dispatch Macros: TLS or raw socket
 *
 * Usage:
 *   DESI_SEND(fd, ssl, buf, len)
 *   DESI_RECV(fd, ssl, buf, len)
 *
 * If ssl != NULL, uses TLS. Otherwise, uses raw send/recv.
 * ============================================================ */

#ifdef __linux__
  #define DESI_RAW_SEND(fd, buf, len) send(fd, buf, len, MSG_NOSIGNAL)
#else
  #define DESI_RAW_SEND(fd, buf, len) send(fd, buf, len, 0)
#endif

#define DESI_RAW_RECV(fd, buf, len) recv(fd, buf, len, 0)

#define DESI_SEND(fd, ssl, buf, len) \
    ((ssl) ? desi_tls_send((ssl), (buf), (len)) : DESI_RAW_SEND((fd), (buf), (len)))

#define DESI_RECV(fd, ssl, buf, len) \
    ((ssl) ? desi_tls_recv((ssl), (buf), (len)) : DESI_RAW_RECV((fd), (buf), (len)))


#endif /* DESI_TLS_H */
