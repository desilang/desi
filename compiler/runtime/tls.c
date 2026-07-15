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
            /* Distinguish self-signed cert warnings from real errors */
            if (strstr(errbuf, "certificate unknown") != NULL) {
                /* Expected with self-signed certs — browser rejects then retries */
                fprintf(stderr, "[tls] Client rejected certificate (fd=%d) — expected with self-signed certs\n", fd);
            } else {
                fprintf(stderr, "[tls] Handshake error (fd=%d): %s\n", fd, errbuf);
            }
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

#elif defined(_WIN32)

/*
 * Server-side TLS via Windows Schannel (SSPI)
 *
 * Links with secur32.lib + crypt32.lib (system).
 * Loads PEM certificate files by converting to DER and importing into
 * a temporary certificate store.
 */

#define SECURITY_WIN32
/* winsock2.h must precede windows.h (or windows.h pulls the legacy
 * winsock.h and the two redefine sockaddr/fd_set) */
#ifndef WIN32_LEAN_AND_MEAN
  #define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <windows.h>
#include <security.h>
#include <schannel.h>
#include <wincrypt.h>

#pragma comment(lib, "secur32.lib")
#pragma comment(lib, "crypt32.lib")
#pragma comment(lib, "advapi32.lib")  /* CryptAcquireContext / CryptImportKey */
#pragma comment(lib, "ws2_32.lib")

/* Schannel server TLS state (wraps SSPI handles) */
typedef struct {
    CredHandle   cred;
    CtxtHandle   ctx;
    int          ctx_valid;
    SecPkgContext_StreamSizes sizes;
    int          sizes_valid;
    SOCKET       fd;
    unsigned char *recv_buf;
    int          recv_len;
    int          recv_cap;
    unsigned char *extra_buf;
    int          extra_len;
} SchannelServerSSL;

/* Helper: Read PEM file and decode Base64 content to DER binary */
static unsigned char* pem_to_der(const char* pem_path, DWORD* out_len) {
    FILE* f = fopen(pem_path, "rb");
    if (!f) return NULL;
    fseek(f, 0, SEEK_END);
    long flen = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* pem = (char*)malloc(flen + 1);
    if (!pem) { fclose(f); return NULL; }
    fread(pem, 1, flen, f);
    fclose(f);
    pem[flen] = '\0';

    /* Find Base64 content between PEM headers */
    char* start = strstr(pem, "-----BEGIN ");
    if (start) {
        start = strchr(start, '\n');
        if (start) start++;
    }
    char* end = strstr(pem, "-----END ");

    if (!start || !end || end <= start) { free(pem); return NULL; }

    /* Strip PEM armor and decode */
    int b64_len = (int)(end - start);
    DWORD der_len = 0;
    CryptStringToBinaryA(start, b64_len, CRYPT_STRING_BASE64, NULL, &der_len, NULL, NULL);
    unsigned char* der = (unsigned char*)malloc(der_len);
    if (!der) { free(pem); return NULL; }
    if (!CryptStringToBinaryA(start, b64_len, CRYPT_STRING_BASE64, der, &der_len, NULL, NULL)) {
        free(der);
        free(pem);
        return NULL;
    }
    free(pem);
    *out_len = der_len;
    return der;
}

DESI_SSL_CTX* desi_tls_ctx_new(const char* cert_path, const char* key_path) {
    /* Load certificate from PEM */
    DWORD cert_der_len = 0;
    unsigned char* cert_der = pem_to_der(cert_path, &cert_der_len);
    if (!cert_der) {
        fprintf(stderr, "[tls/schannel] Failed to load certificate: %s\n", cert_path);
        return NULL;
    }

    /* Create certificate context from DER data */
    PCCERT_CONTEXT cert_ctx = CertCreateCertificateContext(
        X509_ASN_ENCODING | PKCS_7_ASN_ENCODING,
        cert_der, cert_der_len);
    free(cert_der);

    if (!cert_ctx) {
        fprintf(stderr, "[tls/schannel] Failed to create certificate context\n");
        return NULL;
    }

    /* Load private key from PEM and associate with certificate */
    DWORD key_der_len = 0;
    unsigned char* key_der = pem_to_der(key_path, &key_der_len);
    if (key_der) {
        /* Decode PKCS#8 or RSA private key */
        DWORD key_blob_len = 0;
        BYTE* key_blob = NULL;
        if (CryptDecodeObjectEx(X509_ASN_ENCODING | PKCS_7_ASN_ENCODING,
                PKCS_RSA_PRIVATE_KEY, key_der, key_der_len,
                CRYPT_DECODE_ALLOC_FLAG, NULL, &key_blob, &key_blob_len)) {

            HCRYPTPROV hProv = 0;
            HCRYPTKEY hKey = 0;
            if (CryptAcquireContextA(&hProv, NULL, MS_ENHANCED_PROV_A,
                    PROV_RSA_FULL, CRYPT_NEWKEYSET | CRYPT_VERIFYCONTEXT)) {
                if (CryptImportKey(hProv, key_blob, key_blob_len, 0, 0, &hKey)) {
                    /* Associate key with certificate */
                    CRYPT_KEY_PROV_INFO kpi = {0};
                    kpi.dwProvType = PROV_RSA_FULL;
                    kpi.dwKeySpec = AT_KEYEXCHANGE;
                    kpi.pwszContainerName = L"DesiTLS";
                    kpi.pwszProvName = NULL;
                    CertSetCertificateContextProperty(cert_ctx,
                        CERT_KEY_PROV_INFO_PROP_ID, 0, &kpi);
                    CryptDestroyKey(hKey);
                }
                CryptReleaseContext(hProv, 0);
            }
            LocalFree(key_blob);
        }
        free(key_der);
    }

    /* Acquire server credentials with this certificate */
    SCHANNEL_CRED sc = {0};
    sc.dwVersion = SCHANNEL_CRED_VERSION;
    sc.cCreds = 1;
    sc.paCred = &cert_ctx;
    sc.grbitEnabledProtocols = SP_PROT_TLS1_2_SERVER | SP_PROT_TLS1_3_SERVER;

    CredHandle* cred = (CredHandle*)calloc(1, sizeof(CredHandle));
    if (!cred) {
        CertFreeCertificateContext(cert_ctx);
        return NULL;
    }

    SECURITY_STATUS ss = AcquireCredentialsHandleA(
        NULL, UNISP_NAME_A, SECPKG_CRED_INBOUND,
        NULL, &sc, NULL, NULL, cred, NULL);

    CertFreeCertificateContext(cert_ctx);

    if (ss != SEC_E_OK) {
        fprintf(stderr, "[tls/schannel] AcquireCredentialsHandle failed: 0x%lx\n", (unsigned long)ss);
        free(cred);
        return NULL;
    }

    return (DESI_SSL_CTX*)cred;
}

DESI_SSL* desi_tls_accept(DESI_SSL_CTX* ctx, int fd) {
    if (!ctx || fd < 0) return NULL;
    CredHandle* cred = (CredHandle*)ctx;

    SchannelServerSSL* st = (SchannelServerSSL*)calloc(1, sizeof(SchannelServerSSL));
    if (!st) return NULL;
    st->fd = (SOCKET)fd;
    st->recv_cap = 65536;
    st->recv_buf = (unsigned char*)malloc(st->recv_cap);
    if (!st->recv_buf) { free(st); return NULL; }
    memcpy(&st->cred, cred, sizeof(CredHandle));

    /* Server handshake loop */
    DWORD ctx_flags = ASC_REQ_ALLOCATE_MEMORY |
                      ASC_REQ_CONFIDENTIALITY |
                      ASC_REQ_REPLAY_DETECT |
                      ASC_REQ_SEQUENCE_DETECT |
                      ASC_REQ_STREAM;
    DWORD out_flags = 0;
    SECURITY_STATUS ss = SEC_I_CONTINUE_NEEDED;

    unsigned char hs_buf[65536];
    int hs_len = 0;
    int first_call = 1;

    while (ss == SEC_I_CONTINUE_NEEDED || ss == SEC_E_INCOMPLETE_MESSAGE) {
        /* Read data from client */
        int n = recv(fd, (char*)hs_buf + hs_len, sizeof(hs_buf) - hs_len, 0);
        if (n <= 0) goto handshake_fail;
        hs_len += n;

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

        SecBuffer out_buf;
        out_buf.cbBuffer = 0;
        out_buf.BufferType = SECBUFFER_TOKEN;
        out_buf.pvBuffer = NULL;

        SecBufferDesc out_desc;
        out_desc.ulVersion = SECBUFFER_VERSION;
        out_desc.cBuffers = 1;
        out_desc.pBuffers = &out_buf;

        ss = AcceptSecurityContext(
            &st->cred,
            first_call ? NULL : &st->ctx,
            &in_desc, ctx_flags, 0,
            first_call ? &st->ctx : NULL,
            &out_desc, &out_flags, NULL);

        if (first_call) {
            st->ctx_valid = 1;
            first_call = 0;
        }

        /* Send outgoing token */
        if (out_buf.cbBuffer > 0 && out_buf.pvBuffer) {
            send(fd, (const char*)out_buf.pvBuffer, out_buf.cbBuffer, 0);
            FreeContextBuffer(out_buf.pvBuffer);
        }

        /* Handle extra data */
        if (in_bufs[1].BufferType == SECBUFFER_EXTRA && in_bufs[1].cbBuffer > 0) {
            memmove(hs_buf, hs_buf + (hs_len - in_bufs[1].cbBuffer), in_bufs[1].cbBuffer);
            hs_len = in_bufs[1].cbBuffer;
        } else if (ss != SEC_E_INCOMPLETE_MESSAGE) {
            hs_len = 0;
        }
    }

    if (ss != SEC_E_OK) goto handshake_fail;

    /* Get stream sizes */
    ss = QueryContextAttributes(&st->ctx, SECPKG_ATTR_STREAM_SIZES, &st->sizes);
    if (ss == SEC_E_OK) st->sizes_valid = 1;

    /* Save extra data from handshake */
    if (hs_len > 0) {
        memcpy(st->recv_buf, hs_buf, hs_len);
        st->recv_len = hs_len;
    }

    return (DESI_SSL*)st;

handshake_fail:
    if (st->ctx_valid) DeleteSecurityContext(&st->ctx);
    free(st->recv_buf);
    free(st);
    return NULL;
}

ssize_t desi_tls_send(DESI_SSL* ssl, const void* buf, size_t len) {
    if (!ssl || !buf) return -1;
    SchannelServerSSL* st = (SchannelServerSSL*)ssl;
    if (!st->sizes_valid) return -1;

    DWORD max_msg = st->sizes.cbMaximumMessage;
    if (len > max_msg) len = max_msg;

    DWORD total = st->sizes.cbHeader + (DWORD)len + st->sizes.cbTrailer;
    unsigned char* msg = (unsigned char*)malloc(total);
    if (!msg) return -1;

    memcpy(msg + st->sizes.cbHeader, buf, len);

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
    if (ss != SEC_E_OK) { free(msg); return -1; }

    DWORD send_len = enc_bufs[0].cbBuffer + enc_bufs[1].cbBuffer + enc_bufs[2].cbBuffer;
    int sent = 0;
    while (sent < (int)send_len) {
        int n = send(st->fd, (const char*)msg + sent, send_len - sent, 0);
        if (n <= 0) { free(msg); return -1; }
        sent += n;
    }

    free(msg);
    return (ssize_t)len;
}

ssize_t desi_tls_recv(DESI_SSL* ssl, void* buf, size_t len) {
    if (!ssl || !buf) return -1;
    SchannelServerSSL* st = (SchannelServerSSL*)ssl;

    /* Return extra decrypted data first */
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

    while (1) {
        if (st->recv_len > 0) {
            SecBuffer dec_bufs[4];
            dec_bufs[0].cbBuffer = st->recv_len;
            dec_bufs[0].BufferType = SECBUFFER_DATA;
            dec_bufs[0].pvBuffer = st->recv_buf;
            for (int i = 1; i < 4; i++) {
                dec_bufs[i].cbBuffer = 0;
                dec_bufs[i].BufferType = SECBUFFER_EMPTY;
                dec_bufs[i].pvBuffer = NULL;
            }

            SecBufferDesc dec_desc;
            dec_desc.ulVersion = SECBUFFER_VERSION;
            dec_desc.cBuffers = 4;
            dec_desc.pBuffers = dec_bufs;

            SECURITY_STATUS ss = DecryptMessage(&st->ctx, &dec_desc, 0, NULL);

            if (ss == SEC_E_OK) {
                SecBuffer* data_buf = NULL;
                SecBuffer* extra = NULL;
                for (int i = 0; i < 4; i++) {
                    if (dec_bufs[i].BufferType == SECBUFFER_DATA) data_buf = &dec_bufs[i];
                    if (dec_bufs[i].BufferType == SECBUFFER_EXTRA) extra = &dec_bufs[i];
                }
                if (data_buf && data_buf->cbBuffer > 0) {
                    int copy = (int)data_buf->cbBuffer < (int)len ? (int)data_buf->cbBuffer : (int)len;
                    memcpy(buf, data_buf->pvBuffer, copy);
                    if (copy < (int)data_buf->cbBuffer) {
                        int leftover = (int)data_buf->cbBuffer - copy;
                        st->extra_buf = (unsigned char*)malloc(leftover);
                        if (st->extra_buf) {
                            memcpy(st->extra_buf, (unsigned char*)data_buf->pvBuffer + copy, leftover);
                            st->extra_len = leftover;
                        }
                    }
                    if (extra && extra->cbBuffer > 0) {
                        memmove(st->recv_buf, extra->pvBuffer, extra->cbBuffer);
                        st->recv_len = extra->cbBuffer;
                    } else {
                        st->recv_len = 0;
                    }
                    return (ssize_t)copy;
                }
            } else if (ss == SEC_E_INCOMPLETE_MESSAGE) {
                /* need more data */
            } else if (ss == SEC_I_CONTEXT_EXPIRED) {
                return 0;
            } else {
                return -1;
            }
        }

        if (st->recv_len >= st->recv_cap) {
            st->recv_cap *= 2;
            unsigned char* nb = (unsigned char*)realloc(st->recv_buf, st->recv_cap);
            if (!nb) return -1;
            st->recv_buf = nb;
        }

        int n = recv(st->fd, (char*)st->recv_buf + st->recv_len,
                     st->recv_cap - st->recv_len, 0);
        if (n <= 0) return n;
        st->recv_len += n;
    }
}

void desi_tls_close(DESI_SSL* ssl) {
    if (!ssl) return;
    SchannelServerSSL* st = (SchannelServerSSL*)ssl;
    if (st->ctx_valid) DeleteSecurityContext(&st->ctx);
    free(st->recv_buf);
    free(st->extra_buf);
    free(st);
}

void desi_tls_ctx_free(DESI_SSL_CTX* ctx) {
    if (!ctx) return;
    CredHandle* cred = (CredHandle*)ctx;
    FreeCredentialsHandle(cred);
    free(cred);
}

#else /* !DESI_HAS_OPENSSL && !_WIN32 — stub implementations */

DESI_SSL_CTX* desi_tls_ctx_new(const char* cert_path, const char* key_path) {
    (void)cert_path; (void)key_path;
    fprintf(stderr, "[tls] No TLS provider available — TLS disabled\n");
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
