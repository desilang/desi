/*
 * db_redis.c — Pure C Redis RESP protocol client for Desi stdlib
 *
 * Implements the Redis RESP (REdis Serialization Protocol) over TCP.
 * No external dependencies — uses basic sockets.
 *
 * Supports:
 *   - String: SET, GET, DEL, SETEX, TTL, EXISTS, INCR, DECR
 *   - Hash: HSET, HGET, HDEL, HGETALL
 *   - List: LPUSH, RPUSH, LPOP, RPOP, LLEN, LRANGE
 *   - Key: KEYS, EXPIRE, TYPE
 *   - Utility: PING, FLUSHDB
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#include <errno.h>

// Forward declare list types
#include "../list.h"

// ============================================================
// Connection
// ============================================================

static int g_redis_fd = -1;

// Send raw bytes to Redis
static int redis_send(const char* data, int len) {
    if (g_redis_fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n = write(g_redis_fd, data + total, len - total);
        if (n <= 0) return -1;
        total += n;
    }
    return 0;
}

// Read a line from Redis (up to \r\n)
static int redis_readline(char* buf, int max) {
    int i = 0;
    while (i < max - 1) {
        char c;
        int n = read(g_redis_fd, &c, 1);
        if (n <= 0) break;
        buf[i++] = c;
        if (i >= 2 && buf[i-2] == '\r' && buf[i-1] == '\n') {
            buf[i-2] = '\0';
            return i - 2;
        }
    }
    buf[i] = '\0';
    return i;
}

// Read exact number of bytes
static int redis_readn(char* buf, int n) {
    int total = 0;
    while (total < n) {
        int r = read(g_redis_fd, buf + total, n - total);
        if (r <= 0) return -1;
        total += r;
    }
    return 0;
}

// ============================================================
// RESP Protocol
// ============================================================

// Send RESP command: *<argc>\r\n$<len>\r\n<arg>\r\n...
static int send_command(int argc, const char** argv) {
    char buf[64];
    snprintf(buf, sizeof(buf), "*%d\r\n", argc);
    redis_send(buf, strlen(buf));
    for (int i = 0; i < argc; i++) {
        int len = strlen(argv[i]);
        snprintf(buf, sizeof(buf), "$%d\r\n", len);
        redis_send(buf, strlen(buf));
        redis_send(argv[i], len);
        redis_send("\r\n", 2);
    }
    return 0;
}

// Read RESP reply — returns string (caller frees)
static char* read_reply(void) {
    char line[4096];
    redis_readline(line, sizeof(line));
    
    switch (line[0]) {
        case '+': // Simple string
            return strdup(line + 1);
        case '-': // Error
            return strdup(line + 1);
        case ':': // Integer
            return strdup(line + 1);
        case '$': { // Bulk string
            int len = atoi(line + 1);
            if (len < 0) return strdup("");
            char* data = malloc(len + 1);
            redis_readn(data, len);
            data[len] = '\0';
            // Read trailing \r\n
            char crlf[2];
            redis_readn(crlf, 2);
            return data;
        }
        case '*': { // Array (return first element for simple ops)
            int count = atoi(line + 1);
            if (count <= 0) return strdup("");
            // Read just the first element for simple use
            char* first = read_reply();
            for (int i = 1; i < count; i++) {
                char* skip = read_reply();
                free(skip);
            }
            return first;
        }
        default:
            return strdup(line);
    }
}

// Read array reply as list
static DesiList* read_array_reply(void) {
    DesiList* result = list_new(1, NULL);
    char line[4096];
    redis_readline(line, sizeof(line));
    
    if (line[0] != '*') {
        return result;
    }
    
    int count = atoi(line + 1);
    for (int i = 0; i < count; i++) {
        char* item = read_reply();
        list_append(result, item, 1);
    }
    return result;
}

// ============================================================
// Public API
// ============================================================

// Connect to Redis server
int32_t __redis_connect(const char* host, int32_t port) {
    if (!host) host = "127.0.0.1";
    if (port <= 0) port = 6379;
    
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    
    // Try numeric first
    if (inet_pton(AF_INET, host, &addr.sin_addr) <= 0) {
        struct hostent* he = gethostbyname(host);
        if (!he) return -1;
        memcpy(&addr.sin_addr, he->h_addr_list[0], he->h_length);
    }
    
    g_redis_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (g_redis_fd < 0) return -1;
    
    if (connect(g_redis_fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        close(g_redis_fd);
        g_redis_fd = -1;
        return -1;
    }
    return 0;
}

// Close Redis connection
int32_t __redis_close(void) {
    if (g_redis_fd >= 0) {
        close(g_redis_fd);
        g_redis_fd = -1;
    }
    return 0;
}

// PING
char* __redis_ping(void) {
    const char* argv[] = {"PING"};
    send_command(1, argv);
    return read_reply();
}

// ============================================================
// String Commands
// ============================================================

int32_t __redis_set(const char* key, const char* value) {
    const char* argv[] = {"SET", key, value};
    send_command(3, argv);
    char* reply = read_reply();
    int ok = (reply && strcmp(reply, "OK") == 0);
    free(reply);
    return ok ? 0 : -1;
}

char* __redis_get(const char* key) {
    const char* argv[] = {"GET", key};
    send_command(2, argv);
    return read_reply();
}

int32_t __redis_del(const char* key) {
    const char* argv[] = {"DEL", key};
    send_command(2, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

int32_t __redis_exists(const char* key) {
    const char* argv[] = {"EXISTS", key};
    send_command(2, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

// SET with expiry in seconds
int32_t __redis_setex(const char* key, int32_t seconds, const char* value) {
    char sec_str[16];
    snprintf(sec_str, sizeof(sec_str), "%d", seconds);
    const char* argv[] = {"SETEX", key, sec_str, value};
    send_command(4, argv);
    char* reply = read_reply();
    int ok = (reply && strcmp(reply, "OK") == 0);
    free(reply);
    return ok ? 0 : -1;
}

int32_t __redis_expire(const char* key, int32_t seconds) {
    char sec_str[16];
    snprintf(sec_str, sizeof(sec_str), "%d", seconds);
    const char* argv[] = {"EXPIRE", key, sec_str};
    send_command(3, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

int32_t __redis_ttl(const char* key) {
    const char* argv[] = {"TTL", key};
    send_command(2, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

int32_t __redis_incr(const char* key) {
    const char* argv[] = {"INCR", key};
    send_command(2, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

int32_t __redis_decr(const char* key) {
    const char* argv[] = {"DECR", key};
    send_command(2, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

// ============================================================
// Hash Commands
// ============================================================

int32_t __redis_hset(const char* key, const char* field, const char* value) {
    const char* argv[] = {"HSET", key, field, value};
    send_command(4, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

char* __redis_hget(const char* key, const char* field) {
    const char* argv[] = {"HGET", key, field};
    send_command(3, argv);
    return read_reply();
}

int32_t __redis_hdel(const char* key, const char* field) {
    const char* argv[] = {"HDEL", key, field};
    send_command(3, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

// ============================================================
// List Commands
// ============================================================

int32_t __redis_lpush(const char* key, const char* value) {
    const char* argv[] = {"LPUSH", key, value};
    send_command(3, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

int32_t __redis_rpush(const char* key, const char* value) {
    const char* argv[] = {"RPUSH", key, value};
    send_command(3, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

char* __redis_lpop(const char* key) {
    const char* argv[] = {"LPOP", key};
    send_command(2, argv);
    return read_reply();
}

char* __redis_rpop(const char* key) {
    const char* argv[] = {"RPOP", key};
    send_command(2, argv);
    return read_reply();
}

int32_t __redis_llen(const char* key) {
    const char* argv[] = {"LLEN", key};
    send_command(2, argv);
    char* reply = read_reply();
    int n = atoi(reply);
    free(reply);
    return n;
}

// ============================================================
// Utility
// ============================================================

int32_t __redis_flushdb(void) {
    const char* argv[] = {"FLUSHDB"};
    send_command(1, argv);
    char* reply = read_reply();
    int ok = (reply && strcmp(reply, "OK") == 0);
    free(reply);
    return ok ? 0 : -1;
}

// Check if connected
int32_t __redis_is_connected(void) {
    return g_redis_fd >= 0 ? 1 : 0;
}
