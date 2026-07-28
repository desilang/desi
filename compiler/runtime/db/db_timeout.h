/*
 * db_timeout.h — Shared timeout infrastructure for PG/MySQL drivers
 *
 * Provides:
 *   - Global configurable connect/read timeouts (defined in db_timeout.c)
 *   - Non-blocking TCP connect with poll() timeout
 *   - SO_RCVTIMEO wrapper for read timeouts
 *
 * Used by postgres.c, mysql.c, and dispatch.c.
 */

#ifndef DB_TIMEOUT_H
#define DB_TIMEOUT_H

#include "db_socket.h"
#include "../platform.h"   /* desi_now_ms() for the connect retry budget */
#include <stdint.h>
#include <stdio.h>

// ============================================================
// Global configurable timeouts (milliseconds)
// Defined in db_timeout.c — shared across all translation units.
// ============================================================

extern int g_db_connect_timeout_ms;
extern int g_db_read_timeout_ms;

// ============================================================
// Set timeouts — defined in db_timeout.c
// ============================================================

int32_t db_set_timeouts(int32_t connect_ms, int32_t read_ms);

// ============================================================
// Non-blocking TCP connect with timeout
//
// Steps:
//   1. Set socket to non-blocking
//   2. Call connect() — returns -1 with EINPROGRESS
//   3. Use poll() with timeout to wait for writability
//   4. Check SO_ERROR to verify connection succeeded
//   5. Restore socket to blocking mode
//
// Returns: 0 on success, -1 on timeout or error
// ============================================================

static inline int db_connect_with_timeout(int fd, struct sockaddr* addr,
                                           socklen_t addrlen, int timeout_ms) {
    // Set non-blocking
    if (db_set_nonblocking(fd, 1) < 0) return -1;

    int rc = connect(fd, addr, addrlen);
    if (rc == 0) {
        // Connected immediately (e.g., localhost)
        db_set_nonblocking(fd, 0);  // restore blocking
        return 0;
    }

    int connect_err = db_socket_errno();
    if (connect_err != DB_EINPROGRESS) {
        // Capture before restoring blocking mode: that call can overwrite the
        // last-error, and the caller needs the real reason connect() failed.
        db_set_nonblocking(fd, 0);
#ifdef _WIN32
        WSASetLastError(connect_err);
#else
        errno = connect_err;
#endif
        return -1;
    }

    // Wait for connection with poll() — no fd limit unlike select()/FD_SET
    db_pollfd pfd;
    pfd.fd = fd;
    pfd.events = POLLOUT;
    pfd.revents = 0;

    rc = db_poll(&pfd, 1, timeout_ms);
    if (rc <= 0) {
        // Timeout (rc == 0) or error (rc < 0)
        db_set_nonblocking(fd, 0);
        return -1;
    }

    // Check for connection error. This runs for POLLERR/POLLHUP too rather
    // than bailing out early: a refused connection reports POLLERR alongside
    // POLLOUT, and without reading SO_ERROR the caller has nothing to report
    // but a misleading "timed out" with no underlying error.
    int so_error = 0;
    socklen_t so_len = sizeof(so_error);
    if (getsockopt(fd, SOL_SOCKET, SO_ERROR, (char*)&so_error, &so_len) < 0) {
        db_set_nonblocking(fd, 0);
        return -1;
    }

    if (so_error == 0 && (pfd.revents & (POLLERR | POLLHUP))) {
        // Reported failed but SO_ERROR was already consumed — still an error.
        db_set_nonblocking(fd, 0);
        return -1;
    }

    if (so_error != 0) {
        // Connection refused, network unreachable, etc. Publish it last:
        // restoring blocking mode goes through the socket provider, which
        // clears the pending error on success and would erase this.
        db_set_nonblocking(fd, 0);
#ifdef _WIN32
        WSASetLastError(so_error);
#else
        errno = so_error;
#endif
        return -1;
    }

    // Restore blocking mode
    db_set_nonblocking(fd, 0);
    return 0;
}

// ============================================================
// Create a socket and connect it, honouring the connect timeout.
// Returns a connected fd, or -1.
//
// On Windows a busy machine can fail connect() outright with WSAEADDRINUSE:
// every port in the ephemeral range is held by a lingering TIME_WAIT 4-tuple,
// so no local port can be assigned at all. A fresh socket a moment later
// usually gets one, so retry until the caller's connect timeout is spent
// rather than reporting a failure the database had nothing to do with.
// This mirrors net_connect_one() in net.c. POSIX keeps single-attempt.
// ============================================================

static inline int db_open_connection(struct sockaddr* addr, socklen_t addrlen,
                                     int timeout_ms) {
    int fd = (int)socket(addr->sa_family, SOCK_STREAM, 0);
    if (fd < 0) return -1;
    if (db_connect_with_timeout(fd, addr, addrlen, timeout_ms) == 0) return fd;

#ifdef _WIN32
    int64_t deadline = desi_now_ms() + (timeout_ms > 0 ? timeout_ms : 0);
    while (WSAGetLastError() == WSAEADDRINUSE && desi_now_ms() < deadline) {
        db_close_socket(fd);
        Sleep(10);
        fd = (int)socket(addr->sa_family, SOCK_STREAM, 0);
        if (fd < 0) return -1;
        if (db_connect_with_timeout(fd, addr, addrlen, timeout_ms) == 0) return fd;
    }
#endif

    db_close_socket(fd);
    return -1;
}

// ============================================================
// Apply SO_RCVTIMEO to a connected socket
//
// This makes read() / recv() time out instead of blocking forever.
// Returns: 0 on success, -1 on error.
// ============================================================

static inline int db_set_read_timeout(int fd, int timeout_ms) {
#ifdef _WIN32
    // Winsock takes a DWORD of milliseconds here, not a struct timeval —
    // passing a timeval would be read as a garbage millisecond count.
    DWORD tv = (DWORD)timeout_ms;
#else
    struct timeval tv;
    tv.tv_sec  = timeout_ms / 1000;
    tv.tv_usec = (timeout_ms % 1000) * 1000;
#endif
    return setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, DB_SOCKOPT_CAST(&tv),
                      sizeof(tv));
}

#endif /* DB_TIMEOUT_H */
