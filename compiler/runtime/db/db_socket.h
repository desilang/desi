/*
 * db_socket.h — Socket layer for the database drivers
 *
 * The PostgreSQL, MySQL and Redis drivers speak their wire protocols over
 * plain TCP, so they only need a small, shared slice of the sockets API.
 * This header supplies it per platform: POSIX headers as before, WinSock2 on
 * Windows, plus shims for the handful of calls that differ.
 *
 * Same approach net.c and http_server.c already use. Kept in one place so the
 * drivers include this instead of repeating the platform guard four times.
 */

#ifndef DB_SOCKET_H
#define DB_SOCKET_H

#include <stdio.h>
#include <string.h>

#ifdef _WIN32
  #ifndef WIN32_LEAN_AND_MEAN
    #define WIN32_LEAN_AND_MEAN
  #endif
  #include <winsock2.h>
  #include <ws2tcpip.h>
  #pragma comment(lib, "ws2_32.lib")

  /* WinSock spells these differently. */
  typedef int socklen_t;
  #define db_close_socket(fd)   closesocket((SOCKET)(fd))
  #define db_socket_errno()     WSAGetLastError()
  #define DB_EINPROGRESS        WSAEWOULDBLOCK   /* connect() in progress */
  #define DB_EWOULDBLOCK        WSAEWOULDBLOCK

  /* A Windows SOCKET is not a file descriptor, so read/write don't apply to
     it — recv/send are the equivalents. */
  #define db_sock_read(fd, buf, len) \
      recv((SOCKET)(fd), (char*)(buf), (int)(len), 0)
  #define db_sock_write(fd, buf, len) \
      send((SOCKET)(fd), (const char*)(buf), (int)(len), 0)

  /* poll() is WSAPoll, with the same struct and semantics for our use. */
  #define db_poll(fds, n, timeout) WSAPoll((fds), (n), (timeout))
  typedef WSAPOLLFD db_pollfd;

  /* setsockopt takes char* rather than void* here. */
  #define DB_SOCKOPT_CAST(p)    ((const char*)(p))

  /* Winsock has no fcntl; non-blocking is a mode flag. */
  static inline int db_set_nonblocking(int fd, int on) {
      u_long mode = on ? 1u : 0u;
      return ioctlsocket((SOCKET)fd, FIONBIO, &mode) == 0 ? 0 : -1;
  }

  /* Winsock must be started before any socket call. Idempotent; the drivers
     call this from their connect paths. */
  static inline void db_socket_init(void) {
      static int started = 0;
      if (!started) {
          WSADATA wsa;
          WSAStartup(MAKEWORD(2, 2), &wsa);
          started = 1;
      }
  }

  /* Socket errors don't land in errno here, so strerror() would report the
     wrong thing — usually "No error" over a real failure. */
  static inline const char* db_socket_error_str(void) {
      static char buf[128];
      int err = WSAGetLastError();
      if (!FormatMessageA(FORMAT_MESSAGE_FROM_SYSTEM |
                              FORMAT_MESSAGE_IGNORE_INSERTS,
                          NULL, (DWORD)err, 0, buf, sizeof(buf), NULL)) {
          snprintf(buf, sizeof(buf), "winsock error %d", err);
      } else {
          /* FormatMessage appends CRLF. */
          size_t n = strlen(buf);
          while (n > 0 && (buf[n - 1] == '\n' || buf[n - 1] == '\r')) buf[--n] = '\0';
      }
      return buf;
  }
#else
  #include <sys/socket.h>
  #include <sys/time.h>
  #include <netinet/in.h>
  #include <arpa/inet.h>
  #include <netdb.h>
  #include <unistd.h>
  #include <fcntl.h>
  #include <poll.h>
  #include <errno.h>

  #define db_close_socket(fd)   close(fd)
  #define db_socket_errno()     errno
  #define DB_EINPROGRESS        EINPROGRESS
  #define DB_EWOULDBLOCK        EWOULDBLOCK

  #define db_sock_read(fd, buf, len)  read((fd), (buf), (len))
  #define db_sock_write(fd, buf, len) write((fd), (buf), (len))

  static inline const char* db_socket_error_str(void) { return strerror(errno); }

  #define db_poll(fds, n, timeout) poll((fds), (n), (timeout))
  typedef struct pollfd db_pollfd;

  #define DB_SOCKOPT_CAST(p)    (p)

  static inline int db_set_nonblocking(int fd, int on) {
      int flags = fcntl(fd, F_GETFL, 0);
      if (flags < 0) return -1;
      if (on) flags |= O_NONBLOCK;
      else    flags &= ~O_NONBLOCK;
      return fcntl(fd, F_SETFL, flags) < 0 ? -1 : 0;
  }

  static inline void db_socket_init(void) { /* nothing to start on POSIX */ }
#endif

#endif /* DB_SOCKET_H */
