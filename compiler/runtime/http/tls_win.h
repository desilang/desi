/*
 * tls_win.h — Windows TLS via Schannel (system API)
 *
 * No external dependencies. Links with secur32.lib (system).
 *
 * Phase 1: Stub — returns error for HTTPS.
 * Phase 2: Full Schannel SSPI implementation.
 */
#ifndef DESI_TLS_WIN_H
#define DESI_TLS_WIN_H

/*
 * TODO: Implement full Schannel support using SSPI:
 *   - AcquireCredentialsHandle (UNISP_NAME, SECPKG_CRED_OUTBOUND)
 *   - InitializeSecurityContext for handshake loop
 *   - EncryptMessage / DecryptMessage for data transfer
 *
 * For now, HTTP works fine. HTTPS returns a clear error.
 */

static int tls_handshake(Connection *c, const char *host) {
    (void)c; (void)host;
    /* Schannel TLS not yet implemented — HTTPS unavailable on Windows */
    return -1;
}

static ssize_t tls_write(Connection *c, const void *buf, size_t len) {
    (void)c; (void)buf; (void)len;
    return -1;
}

static ssize_t tls_read(Connection *c, void *buf, size_t len) {
    (void)c; (void)buf; (void)len;
    return -1;
}

static void tls_close(Connection *c) {
    (void)c;
}

#endif /* DESI_TLS_WIN_H */
