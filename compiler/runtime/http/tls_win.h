/*
 * tls_win.h — Windows TLS via Schannel (SSPI)
 *
 * No external dependencies. Links with secur32.lib + crypt32.lib (system).
 *
 * Uses the SSPI (Security Support Provider Interface) with Schannel to provide
 * TLS client support on Windows. This enables HTTPS requests without OpenSSL.
 *
 * Public API (matches tls_openssl.h interface):
 *   tls_handshake(conn, host) — perform TLS handshake
 *   tls_write(conn, buf, len) — send encrypted data
 *   tls_read(conn, buf, len)  — receive and decrypt data
 *   tls_close(conn)           — shutdown TLS and free resources
 */
#ifndef DESI_TLS_WIN_H
#define DESI_TLS_WIN_H

#ifdef _WIN32

#define SECURITY_WIN32
#include <windows.h>
#include <security.h>
#include <schannel.h>
#include <wincrypt.h>
#include <winsock2.h>

#pragma comment(lib, "secur32.lib")
#pragma comment(lib, "crypt32.lib")
#pragma comment(lib, "ws2_32.lib")

/* Schannel TLS state */
typedef struct {
    CredHandle   cred;       /* SSPI credential handle */
    CtxtHandle   ctx;        /* security context */
    int          ctx_valid;  /* 1 if ctx has been initialized */
    SecPkgContext_StreamSizes sizes; /* stream sizes for encrypt/decrypt */
    int          sizes_valid;
    unsigned char *recv_buf; /* buffer for received encrypted data */
    int          recv_len;   /* bytes in recv_buf */
    int          recv_cap;   /* capacity of recv_buf */
    unsigned char *extra_buf;/* leftover decrypted data from previous read */
    int          extra_len;
} SchannelState;

#define SCHANNEL_RECV_BUF_SIZE 65536

static SchannelState* schannel_state_new(void) {
    SchannelState* st = (SchannelState*)calloc(1, sizeof(SchannelState));
    if (!st) return NULL;
    st->recv_cap = SCHANNEL_RECV_BUF_SIZE;
    st->recv_buf = (unsigned char*)malloc(st->recv_cap);
    if (!st->recv_buf) { free(st); return NULL; }
    st->recv_len = 0;
    st->extra_buf = NULL;
    st->extra_len = 0;
    return st;
}

/* ---- TLS Handshake ---- */

static int tls_handshake(Connection *c, const char *host) {
    SchannelState* st = schannel_state_new();
    if (!st) return -1;

    /* Acquire credentials — use default system TLS settings */
    SCHANNEL_CRED sc = {0};
    sc.dwVersion = SCHANNEL_CRED_VERSION;
    sc.dwFlags = SCH_CRED_AUTO_CRED_VALIDATION |
                 SCH_CRED_NO_DEFAULT_CREDS |
                 SCH_CRED_REVOCATION_CHECK_CHAIN;
    sc.grbitEnabledProtocols = SP_PROT_TLS1_2_CLIENT | SP_PROT_TLS1_3_CLIENT;

    SECURITY_STATUS ss = AcquireCredentialsHandleA(
        NULL, UNISP_NAME_A, SECPKG_CRED_OUTBOUND,
        NULL, &sc, NULL, NULL, &st->cred, NULL);

    if (ss != SEC_E_OK) {
        free(st->recv_buf);
        free(st);
        return -1;
    }

    /* InitializeSecurityContext handshake loop */
    SecBufferDesc out_desc;
    SecBuffer out_buf;
    DWORD ctx_flags = ISC_REQ_ALLOCATE_MEMORY |
                      ISC_REQ_CONFIDENTIALITY |
                      ISC_REQ_REPLAY_DETECT |
                      ISC_REQ_SEQUENCE_DETECT |
                      ISC_REQ_STREAM;
    DWORD out_flags = 0;

    /* First call — initiate handshake */
    out_buf.cbBuffer = 0;
    out_buf.BufferType = SECBUFFER_TOKEN;
    out_buf.pvBuffer = NULL;
    out_desc.ulVersion = SECBUFFER_VERSION;
    out_desc.cBuffers = 1;
    out_desc.pBuffers = &out_buf;

    ss = InitializeSecurityContextA(
        &st->cred, NULL, (SEC_CHAR*)host, ctx_flags,
        0, 0, NULL, 0, &st->ctx, &out_desc, &out_flags, NULL);

    st->ctx_valid = 1;

    if (ss != SEC_I_CONTINUE_NEEDED && ss != SEC_E_OK) {
        FreeCredentialsHandle(&st->cred);
        free(st->recv_buf);
        free(st);
        return -1;
    }

    /* Send initial token to server */
    if (out_buf.cbBuffer > 0 && out_buf.pvBuffer) {
        send(c->fd, (const char*)out_buf.pvBuffer, out_buf.cbBuffer, 0);
        FreeContextBuffer(out_buf.pvBuffer);
    }

    /* Handshake loop — read server responses and continue */
    unsigned char hs_buf[65536];
    int hs_len = 0;

    while (ss == SEC_I_CONTINUE_NEEDED || ss == SEC_I_INCOMPLETE_CREDENTIALS) {
        /* Receive data from server */
        int n = recv(c->fd, (char*)hs_buf + hs_len, sizeof(hs_buf) - hs_len, 0);
        if (n <= 0) {
            DeleteSecurityContext(&st->ctx);
            FreeCredentialsHandle(&st->cred);
            free(st->recv_buf);
            free(st);
            return -1;
        }
        hs_len += n;

        /* Set up input buffers */
        SecBuffer in_bufs[2];
        in_bufs[0].cbBuffer = hs_len;
        in_bufs[0].BufferType = SECBUFFER_TOKEN;
        in_bufs[0].pvBuffer = hs_buf;
        in_bufs[1].cbBuffer = 0;
        in_bufs[1].BufferType = SECBUFFER_EMPTY;
        in_bufs[1].pvBuffer = NULL;

        SecBufferDesc in_desc;
        in_desc.ulVersion = SECBUFFER_VERSION;
        in_desc.cBuffers = 2;
        in_desc.pBuffers = in_bufs;

        out_buf.cbBuffer = 0;
        out_buf.BufferType = SECBUFFER_TOKEN;
        out_buf.pvBuffer = NULL;
        out_desc.cBuffers = 1;
        out_desc.pBuffers = &out_buf;

        ss = InitializeSecurityContextA(
            &st->cred, &st->ctx, (SEC_CHAR*)host, ctx_flags,
            0, 0, &in_desc, 0, NULL, &out_desc, &out_flags, NULL);

        /* Send outgoing token if any */
        if (out_buf.cbBuffer > 0 && out_buf.pvBuffer) {
            send(c->fd, (const char*)out_buf.pvBuffer, out_buf.cbBuffer, 0);
            FreeContextBuffer(out_buf.pvBuffer);
        }

        /* Handle extra data (leftover from handshake) */
        if (in_bufs[1].BufferType == SECBUFFER_EXTRA && in_bufs[1].cbBuffer > 0) {
            memmove(hs_buf, hs_buf + (hs_len - in_bufs[1].cbBuffer), in_bufs[1].cbBuffer);
            hs_len = in_bufs[1].cbBuffer;
        } else {
            hs_len = 0;
        }
    }

    if (ss != SEC_E_OK) {
        DeleteSecurityContext(&st->ctx);
        FreeCredentialsHandle(&st->cred);
        free(st->recv_buf);
        free(st);
        return -1;
    }

    /* Get stream sizes for encrypt/decrypt */
    ss = QueryContextAttributes(&st->ctx, SECPKG_ATTR_STREAM_SIZES, &st->sizes);
    if (ss == SEC_E_OK) {
        st->sizes_valid = 1;
    }

    /* Store any leftover handshake data for first read */
    if (hs_len > 0) {
        st->extra_buf = (unsigned char*)malloc(hs_len);
        if (st->extra_buf) {
            memcpy(st->extra_buf, hs_buf, hs_len);
            st->extra_len = hs_len;
        }
    }

    c->tls_state = st;
    c->use_tls = 1;
    return 0;
}

/* ---- TLS Write (Encrypt + Send) ---- */

static ssize_t tls_write(Connection *c, const void *buf, size_t len) {
    SchannelState* st = (SchannelState*)c->tls_state;
    if (!st || !st->sizes_valid) return -1;

    /* Limit to max message size */
    DWORD max_msg = st->sizes.cbMaximumMessage;
    if (len > max_msg) len = max_msg;

    /* Allocate buffer: header + data + trailer */
    DWORD total = st->sizes.cbHeader + (DWORD)len + st->sizes.cbTrailer;
    unsigned char* msg = (unsigned char*)malloc(total);
    if (!msg) return -1;

    /* Copy plaintext into the data portion */
    memcpy(msg + st->sizes.cbHeader, buf, len);

    /* Set up encryption buffers */
    SecBuffer enc_bufs[4];
    enc_bufs[0].cbBuffer = st->sizes.cbHeader;
    enc_bufs[0].BufferType = SECBUFFER_STREAM_HEADER;
    enc_bufs[0].pvBuffer = msg;

    enc_bufs[1].cbBuffer = (DWORD)len;
    enc_bufs[1].BufferType = SECBUFFER_DATA;
    enc_bufs[1].pvBuffer = msg + st->sizes.cbHeader;

    enc_bufs[2].cbBuffer = st->sizes.cbTrailer;
    enc_bufs[2].BufferType = SECBUFFER_STREAM_TRAILER;
    enc_bufs[2].pvBuffer = msg + st->sizes.cbHeader + len;

    enc_bufs[3].cbBuffer = 0;
    enc_bufs[3].BufferType = SECBUFFER_EMPTY;
    enc_bufs[3].pvBuffer = NULL;

    SecBufferDesc enc_desc;
    enc_desc.ulVersion = SECBUFFER_VERSION;
    enc_desc.cBuffers = 4;
    enc_desc.pBuffers = enc_bufs;

    SECURITY_STATUS ss = EncryptMessage(&st->ctx, 0, &enc_desc, 0);
    if (ss != SEC_E_OK) {
        free(msg);
        return -1;
    }

    /* Send all encrypted buffers */
    DWORD send_len = enc_bufs[0].cbBuffer + enc_bufs[1].cbBuffer + enc_bufs[2].cbBuffer;
    int sent = 0;
    while (sent < (int)send_len) {
        int n = send(c->fd, (const char*)msg + sent, send_len - sent, 0);
        if (n <= 0) { free(msg); return -1; }
        sent += n;
    }

    free(msg);
    return (ssize_t)len;
}

/* ---- TLS Read (Receive + Decrypt) ---- */

static ssize_t tls_read(Connection *c, void *buf, size_t len) {
    SchannelState* st = (SchannelState*)c->tls_state;
    if (!st) return -1;

    /* If we have extra decrypted data from a previous read, return it first */
    if (st->extra_buf && st->extra_len > 0) {
        int copy = st->extra_len < (int)len ? st->extra_len : (int)len;
        memcpy(buf, st->extra_buf, copy);
        if (copy < st->extra_len) {
            memmove(st->extra_buf, st->extra_buf + copy, st->extra_len - copy);
            st->extra_len -= copy;
        } else {
            free(st->extra_buf);
            st->extra_buf = NULL;
            st->extra_len = 0;
        }
        return (ssize_t)copy;
    }

    /* Read encrypted data from socket */
    while (1) {
        /* Try to decrypt what we have */
        if (st->recv_len > 0) {
            SecBuffer dec_bufs[4];
            dec_bufs[0].cbBuffer = st->recv_len;
            dec_bufs[0].BufferType = SECBUFFER_DATA;
            dec_bufs[0].pvBuffer = st->recv_buf;
            dec_bufs[1].cbBuffer = 0;
            dec_bufs[1].BufferType = SECBUFFER_EMPTY;
            dec_bufs[1].pvBuffer = NULL;
            dec_bufs[2].cbBuffer = 0;
            dec_bufs[2].BufferType = SECBUFFER_EMPTY;
            dec_bufs[2].pvBuffer = NULL;
            dec_bufs[3].cbBuffer = 0;
            dec_bufs[3].BufferType = SECBUFFER_EMPTY;
            dec_bufs[3].pvBuffer = NULL;

            SecBufferDesc dec_desc;
            dec_desc.ulVersion = SECBUFFER_VERSION;
            dec_desc.cBuffers = 4;
            dec_desc.pBuffers = dec_bufs;

            SECURITY_STATUS ss = DecryptMessage(&st->ctx, &dec_desc, 0, NULL);

            if (ss == SEC_E_OK) {
                /* Find the decrypted data buffer */
                SecBuffer* data_buf = NULL;
                SecBuffer* extra_buf = NULL;
                for (int i = 0; i < 4; i++) {
                    if (dec_bufs[i].BufferType == SECBUFFER_DATA)
                        data_buf = &dec_bufs[i];
                    if (dec_bufs[i].BufferType == SECBUFFER_EXTRA)
                        extra_buf = &dec_bufs[i];
                }

                if (data_buf && data_buf->cbBuffer > 0) {
                    int copy = (int)data_buf->cbBuffer < (int)len ? (int)data_buf->cbBuffer : (int)len;
                    memcpy(buf, data_buf->pvBuffer, copy);

                    /* Save leftover decrypted data */
                    if (copy < (int)data_buf->cbBuffer) {
                        int leftover = (int)data_buf->cbBuffer - copy;
                        st->extra_buf = (unsigned char*)malloc(leftover);
                        if (st->extra_buf) {
                            memcpy(st->extra_buf, (unsigned char*)data_buf->pvBuffer + copy, leftover);
                            st->extra_len = leftover;
                        }
                    }

                    /* Handle extra encrypted data */
                    if (extra_buf && extra_buf->cbBuffer > 0) {
                        memmove(st->recv_buf, extra_buf->pvBuffer, extra_buf->cbBuffer);
                        st->recv_len = extra_buf->cbBuffer;
                    } else {
                        st->recv_len = 0;
                    }

                    return (ssize_t)copy;
                }
            } else if (ss == SEC_E_INCOMPLETE_MESSAGE) {
                /* Need more data — fall through to recv() */
            } else if (ss == SEC_I_CONTEXT_EXPIRED) {
                /* TLS shutdown */
                return 0;
            } else {
                /* Error */
                return -1;
            }
        }

        /* Read more encrypted data from socket */
        if (st->recv_len >= st->recv_cap) {
            /* Grow buffer */
            st->recv_cap *= 2;
            unsigned char* new_buf = (unsigned char*)realloc(st->recv_buf, st->recv_cap);
            if (!new_buf) return -1;
            st->recv_buf = new_buf;
        }

        int n = recv(c->fd, (char*)st->recv_buf + st->recv_len,
                     st->recv_cap - st->recv_len, 0);
        if (n <= 0) return n;
        st->recv_len += n;
    }
}

/* ---- TLS Close ---- */

static void tls_close(Connection *c) {
    if (!c->tls_state) return;
    SchannelState* st = (SchannelState*)c->tls_state;

    /* Send TLS shutdown notification */
    DWORD shutdown_type = SCHANNEL_SHUTDOWN;
    SecBuffer shut_buf;
    shut_buf.cbBuffer = sizeof(shutdown_type);
    shut_buf.BufferType = SECBUFFER_TOKEN;
    shut_buf.pvBuffer = &shutdown_type;

    SecBufferDesc shut_desc;
    shut_desc.ulVersion = SECBUFFER_VERSION;
    shut_desc.cBuffers = 1;
    shut_desc.pBuffers = &shut_buf;

    if (st->ctx_valid) {
        SECURITY_STATUS ss = ApplyControlToken(&st->ctx, &shut_desc);
        if (ss == SEC_E_OK) {
            SecBuffer out_buf;
            out_buf.cbBuffer = 0;
            out_buf.BufferType = SECBUFFER_TOKEN;
            out_buf.pvBuffer = NULL;

            SecBufferDesc out_desc;
            out_desc.ulVersion = SECBUFFER_VERSION;
            out_desc.cBuffers = 1;
            out_desc.pBuffers = &out_buf;

            DWORD flags = ISC_REQ_ALLOCATE_MEMORY | ISC_REQ_STREAM;
            ss = InitializeSecurityContextA(
                &st->cred, &st->ctx, NULL, flags,
                0, 0, NULL, 0, NULL, &out_desc, &flags, NULL);

            if (out_buf.cbBuffer > 0 && out_buf.pvBuffer) {
                send(c->fd, (const char*)out_buf.pvBuffer, out_buf.cbBuffer, 0);
                FreeContextBuffer(out_buf.pvBuffer);
            }
        }

        DeleteSecurityContext(&st->ctx);
    }

    FreeCredentialsHandle(&st->cred);
    free(st->recv_buf);
    free(st->extra_buf);
    free(st);
    c->tls_state = NULL;
}

/* Custom TLS configuration stubs (Windows uses system certificate store) */
void __http_set_ca_bundle(const char* path) {
    (void)path;
    /* Windows uses the system certificate store — CA bundle not needed */
}

void __http_set_client_cert(const char* cert, const char* key) {
    (void)cert; (void)key;
    /*
     * NOTE: Client certificate support requires Windows-specific implementation:
     *   1. Open the Windows certificate store (CertOpenSystemStore)
     *   2. Find the certificate by subject or thumbprint (CertFindCertificateInStore)
     *   3. Associate it with the Schannel credential (set paCred in SCHANNEL_CRED)
     *   4. The 'cert' param should be a thumbprint/subject, 'key' is unused
     *      since Windows manages private keys internally.
     *
     * This must be implemented and tested on a Windows machine.
     * Tracked in: prep_for_v010.md (deferred items)
     */
}

#else /* !_WIN32 */

/* Non-Windows stub — should not be included */
#error "tls_win.h should only be included on Windows builds"

#endif /* _WIN32 */

#endif /* DESI_TLS_WIN_H */
