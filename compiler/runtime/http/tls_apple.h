/*
 * tls_apple.h — macOS TLS via OpenSSL/LibreSSL
 *
 * SecureTransport is deprecated by Apple and broken on modern macOS
 * (returns errSecParam -50 during handshake). We use OpenSSL instead,
 * which is available via Homebrew (brew install openssl) or as
 * LibreSSL on some macOS versions.
 *
 * Compile with: -I$(brew --prefix openssl)/include
 * Link with:    -L$(brew --prefix openssl)/lib -lssl -lcrypto
 *
 * If OpenSSL is not installed, HTTP still works; only HTTPS fails
 * with a clear error message.
 */
#ifndef DESI_TLS_APPLE_H
#define DESI_TLS_APPLE_H

/*
 * On macOS, we try to dynamically detect and use OpenSSL.
 * If the headers aren't available at compile time, we compile
 * in stub mode (HTTP only, HTTPS returns error).
 */

#if __has_include(<openssl/ssl.h>)
  #define DESI_HAS_OPENSSL 1
  #include <openssl/ssl.h>
  #include <openssl/err.h>

  /* Reuse the OpenSSL implementation directly */
  #include "tls_openssl.h"

#else
  /* No OpenSSL available — HTTP only */
  #define DESI_HAS_OPENSSL 0

  static int tls_handshake(Connection *c, const char *host) {
      (void)c; (void)host;
      return -1;  /* Caller will report "TLS handshake failed" */
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

#endif /* __has_include */

#endif /* DESI_TLS_APPLE_H */
