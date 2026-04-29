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

#include <sys/socket.h>
#include <sys/time.h>
#include <fcntl.h>
#include <errno.h>
#include <unistd.h>
#include <stdio.h>
#include <poll.h>

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
    // Get current flags
    int flags = fcntl(fd, F_GETFL, 0);
    if (flags < 0) return -1;

    // Set non-blocking
    if (fcntl(fd, F_SETFL, flags | O_NONBLOCK) < 0) return -1;

    int rc = connect(fd, addr, addrlen);
    if (rc == 0) {
        // Connected immediately (e.g., localhost)
        fcntl(fd, F_SETFL, flags);  // restore blocking
        return 0;
    }

    if (errno != EINPROGRESS) {
        fcntl(fd, F_SETFL, flags);  // restore blocking
        return -1;
    }

    // Wait for connection with poll() — no fd limit unlike select()/FD_SET
    struct pollfd pfd;
    pfd.fd = fd;
    pfd.events = POLLOUT;
    pfd.revents = 0;

    rc = poll(&pfd, 1, timeout_ms);
    if (rc <= 0) {
        // Timeout (rc == 0) or error (rc < 0)
        fcntl(fd, F_SETFL, flags);
        return -1;
    }

    if (pfd.revents & (POLLERR | POLLHUP)) {
        fcntl(fd, F_SETFL, flags);
        return -1;
    }

    // Check for connection error
    int so_error = 0;
    socklen_t so_len = sizeof(so_error);
    if (getsockopt(fd, SOL_SOCKET, SO_ERROR, &so_error, &so_len) < 0) {
        fcntl(fd, F_SETFL, flags);
        return -1;
    }

    if (so_error != 0) {
        // Connection refused, network unreachable, etc.
        errno = so_error;
        fcntl(fd, F_SETFL, flags);
        return -1;
    }

    // Restore blocking mode
    fcntl(fd, F_SETFL, flags);
    return 0;
}

// ============================================================
// Apply SO_RCVTIMEO to a connected socket
//
// This makes read() / recv() time out instead of blocking forever.
// Returns: 0 on success, -1 on error.
// ============================================================

static inline int db_set_read_timeout(int fd, int timeout_ms) {
    struct timeval tv;
    tv.tv_sec  = timeout_ms / 1000;
    tv.tv_usec = (timeout_ms % 1000) * 1000;
    return setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
}

#endif /* DB_TIMEOUT_H */
