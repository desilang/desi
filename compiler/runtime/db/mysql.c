/*
 * db_mysql.c — Pure C MySQL Wire Protocol Client
 *
 * MySQL client/server protocol over TCP.
 * All internal functions prefixed with my_ to avoid conflicts.
 * Supports mysql_native_password auth (SHA1-based).
 *
 * Reference: https://dev.mysql.com/doc/dev/mysql-server/latest/
 *            page_protocol_basic_packets.html
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdarg.h>
#include <unistd.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#include <errno.h>
#include "db_timeout.h"

// Optional TLS/SSL support (compile-time detection)
#ifdef __has_include
  #if __has_include(<openssl/ssl.h>)
    #define MY_HAS_SSL 1
    #include <openssl/ssl.h>
    #include <openssl/err.h>
  #endif
#endif
#ifndef MY_HAS_SSL
  #define MY_HAS_SSL 0
#endif

// ============================================================
// Byte helpers (MySQL = little-endian)
// ============================================================

static void my_write_u24(unsigned char* buf, uint32_t val) {
    buf[0] = val & 0xFF;
    buf[1] = (val >> 8) & 0xFF;
    buf[2] = (val >> 16) & 0xFF;
}

static uint32_t my_read_u24(const unsigned char* buf) {
    return buf[0] | (buf[1] << 8) | (buf[2] << 16);
}

static void my_write_u32(unsigned char* buf, uint32_t val) {
    buf[0] = val & 0xFF;
    buf[1] = (val >> 8) & 0xFF;
    buf[2] = (val >> 16) & 0xFF;
    buf[3] = (val >> 24) & 0xFF;
}

static uint32_t my_read_u32(const unsigned char* buf) {
    return buf[0] | (buf[1] << 8) | (buf[2] << 16) | (buf[3] << 24);
}

static uint16_t my_read_u16(const unsigned char* buf) {
    return buf[0] | (buf[1] << 8);
}

static void my_write_u16(unsigned char* buf, uint16_t val) {
    buf[0] = val & 0xFF;
    buf[1] = (val >> 8) & 0xFF;
}

// ============================================================
// Connection State — struct-based for connection pooling
// ============================================================

#define MY_MAX_COLS 64
#define MY_BUF_SIZE 65536

// MYConn holds all state for a single MySQL connection.
// For connection pooling, each pool slot gets its own MYConn*.
typedef struct {
    // Connection
    int fd;
    int connected;
    char error[512];
    uint8_t seq;  // packet sequence number

    // Stored connection params for reconnect
    char host[256];
    int  port;
    char dbname[128];
    char user[128];
    char password[256];

    // TLS/SSL
#if MY_HAS_SSL
    SSL_CTX* ssl_ctx;
    SSL* ssl;
#endif
    int use_ssl;  // 1 if SSL is active

    // Result set
    int ncols;
    char colnames[MY_MAX_COLS][128];
    int nrows;
    char** cells;
    int cells_cap;
} MYConn;

// The active connection — backward compatible with existing code.
// Connection pooling will swap this pointer.
static MYConn* g_my = NULL;
static int g_my_debug = 0;

// Ensure g_my is allocated (lazy init)
static void my_ensure_conn(void) {
    if (!g_my) {
        g_my = (MYConn*)calloc(1, sizeof(MYConn));
        g_my->fd = -1;
    }
}

// Pool accessors — used by db_pool.c to swap the active connection
void* __my_get_conn_ptr(void) { return (void*)g_my; }
void  __my_set_conn_ptr(void* conn) { g_my = (MYConn*)conn; }

static void my_log(const char* fmt, ...) {
    if (!g_my_debug) return;
    va_list ap;
    va_start(ap, fmt);
    fprintf(stderr, "[mysql] ");
    vfprintf(stderr, fmt, ap);
    fprintf(stderr, "\n");
    va_end(ap);
}

int32_t __my_set_debug(int32_t enabled) {
    g_my_debug = enabled;
    return 0;
}

// ============================================================
// Low-level I/O
// ============================================================

static int my_send_raw(const unsigned char* data, int len) {
    if (g_my->fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n;
#if MY_HAS_SSL
        if (g_my->use_ssl && g_my->ssl)
            n = SSL_write(g_my->ssl, data + total, len - total);
        else
#endif
            n = (int)write(g_my->fd, data + total, len - total);
        if (n <= 0) return -1;
        total += n;
    }
    return 0;
}

static int my_recv_raw(unsigned char* buf, int len) {
    if (g_my->fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n;
#if MY_HAS_SSL
        if (g_my->use_ssl && g_my->ssl)
            n = SSL_read(g_my->ssl, buf + total, len - total);
        else
#endif
            n = (int)read(g_my->fd, buf + total, len - total);
        if (n <= 0) return -1;
        total += n;
    }
    return 0;
}

// MySQL packet: 3-byte length + 1-byte sequence + payload
static int my_read_packet(unsigned char* payload, int* out_len) {
    unsigned char header[4];
    if (my_recv_raw(header, 4) < 0) return -1;
    uint32_t plen = my_read_u24(header);
    g_my->seq = header[3];
    if (plen > 0 && plen < MY_BUF_SIZE) {
        if (my_recv_raw(payload, plen) < 0) return -1;
    } else if (plen >= MY_BUF_SIZE) {
        unsigned char drain[4096];
        int rem = plen;
        while (rem > 0) {
            int chunk = rem > 4096 ? 4096 : rem;
            if (my_recv_raw(drain, chunk) < 0) return -1;
            rem -= chunk;
        }
        plen = 0;
    }
    *out_len = (int)plen;
    my_log("recv packet seq=%d len=%d type=0x%02x", g_my->seq, plen, plen > 0 ? payload[0] : 0);
    return 0;
}

static int my_send_packet(const unsigned char* payload, int len) {
    unsigned char header[4];
    my_write_u24(header, len);
    header[3] = ++g_my->seq;
    if (my_send_raw(header, 4) < 0) return -1;
    if (my_send_raw(payload, len) < 0) return -1;
    my_log("sent packet seq=%d len=%d", g_my->seq, len);
    return 0;
}

// ============================================================
// SHA1 (for mysql_native_password)
// ============================================================

static void my_sha1_block(uint32_t st[5], const unsigned char blk[64]) {
    uint32_t w[80];
    for (int i=0;i<16;i++)
        w[i]=((uint32_t)blk[i*4]<<24)|((uint32_t)blk[i*4+1]<<16)|((uint32_t)blk[i*4+2]<<8)|blk[i*4+3];
    for (int i=16;i<80;i++) {
        uint32_t t=w[i-3]^w[i-8]^w[i-14]^w[i-16];
        w[i]=(t<<1)|(t>>31);
    }
    uint32_t a=st[0],b=st[1],c=st[2],d=st[3],e=st[4];
    for (int i=0;i<80;i++){
        uint32_t f,k;
        if(i<20){f=(b&c)|((~b)&d);k=0x5A827999;}
        else if(i<40){f=b^c^d;k=0x6ED9EBA1;}
        else if(i<60){f=(b&c)|(b&d)|(c&d);k=0x8F1BBCDC;}
        else{f=b^c^d;k=0xCA62C1D6;}
        uint32_t t=((a<<5)|(a>>27))+f+e+k+w[i];
        e=d;d=c;c=(b<<30)|(b>>2);b=a;a=t;
    }
    st[0]+=a;st[1]+=b;st[2]+=c;st[3]+=d;st[4]+=e;
}

static void my_sha1(const unsigned char* data, size_t len, unsigned char digest[20]) {
    uint32_t st[5]={0x67452301,0xEFCDAB89,0x98BADCFE,0x10325476,0xC3D2E1F0};
    unsigned char buf[64];
    size_t i;
    for(i=0;i+64<=len;i+=64) my_sha1_block(st,(const unsigned char*)(data+i));
    size_t rem=len-i;
    memset(buf,0,64);
    memcpy(buf,data+i,rem);
    buf[rem]=0x80;
    if(rem>=56){my_sha1_block(st,buf);memset(buf,0,64);}
    uint64_t bits=len*8;
    buf[56]=(bits>>56)&0xFF; buf[57]=(bits>>48)&0xFF;
    buf[58]=(bits>>40)&0xFF; buf[59]=(bits>>32)&0xFF;
    buf[60]=(bits>>24)&0xFF; buf[61]=(bits>>16)&0xFF;
    buf[62]=(bits>>8)&0xFF;  buf[63]=bits&0xFF;
    my_sha1_block(st,buf);
    for(int j=0;j<5;j++){
        digest[j*4]=(st[j]>>24)&0xFF; digest[j*4+1]=(st[j]>>16)&0xFF;
        digest[j*4+2]=(st[j]>>8)&0xFF; digest[j*4+3]=st[j]&0xFF;
    }
}

// mysql_native_password: SHA1(password) XOR SHA1(scramble + SHA1(SHA1(password)))
static void my_native_auth(const char* password, const unsigned char* scramble, int scramble_len,
                           unsigned char result[20]) {
    unsigned char sha1_pass[20], sha1_sha1[20], combined[40+20], sha1_combined[20];

    // SHA1(password)
    my_sha1((const unsigned char*)password, strlen(password), sha1_pass);
    // SHA1(SHA1(password))
    my_sha1(sha1_pass, 20, sha1_sha1);
    // SHA1(scramble + SHA1(SHA1(password)))
    memcpy(combined, scramble, scramble_len);
    memcpy(combined + scramble_len, sha1_sha1, 20);
    my_sha1(combined, scramble_len + 20, sha1_combined);
    // XOR
    for (int i = 0; i < 20; i++)
        result[i] = sha1_pass[i] ^ sha1_combined[i];
}

// ============================================================
// Result set management
// ============================================================

static void my_free_results(void) {
    if (g_my->cells) {
        for (int i = 0; i < g_my->nrows * g_my->ncols; i++) {
            if (g_my->cells[i]) free(g_my->cells[i]);
        }
        free(g_my->cells);
        g_my->cells = NULL;
    }
    g_my->nrows = 0;
    g_my->ncols = 0;
    g_my->cells_cap = 0;
}

static void my_ensure_cells(int needed) {
    if (needed <= g_my->cells_cap) return;
    int newcap = needed * 2;
    g_my->cells = realloc(g_my->cells, newcap * sizeof(char*));
    for (int i = g_my->cells_cap; i < newcap; i++) g_my->cells[i] = NULL;
    g_my->cells_cap = newcap;
}

// ============================================================
// Read length-encoded integer (MySQL protocol)
// ============================================================

static uint64_t my_read_lenenc(const unsigned char* p, int* bytes_read) {
    if (p[0] < 0xFB) {
        *bytes_read = 1;
        return p[0];
    } else if (p[0] == 0xFB) {
        // NULL
        *bytes_read = 1;
        return 0xFFFFFFFFFFFFFFFFULL; // sentinel for NULL
    } else if (p[0] == 0xFC) {
        *bytes_read = 3;
        return my_read_u16(p + 1);
    } else if (p[0] == 0xFD) {
        *bytes_read = 4;
        return my_read_u24(p + 1);
    } else { // 0xFE
        *bytes_read = 9;
        return my_read_u32(p + 1) | ((uint64_t)my_read_u32(p + 5) << 32);
    }
}

// ============================================================
// Connect
// ============================================================

// Forward declarations
extern int32_t __db_set_dialect(int32_t dialect);
extern int32_t __orm_set_dialect(int32_t dialect);

// Send COM_INIT_DB to select database after auth completes.
// Uses the raw protocol command (0x02) rather than __my_query to avoid
// circular dependency on connected state.
static int my_use_database(const char* dbname) {
    if (!dbname || dbname[0] == '\0') return 0; // no db requested

    // COM_INIT_DB: command byte + database name
    int dlen = strlen(dbname);
    unsigned char pkt[MY_BUF_SIZE];
    pkt[0] = 0x02; // COM_INIT_DB
    memcpy(pkt + 1, dbname, dlen);

    g_my->seq = 0xFF; // reset so send_packet uses seq 0
    if (my_send_packet(pkt, 1 + dlen) < 0) {
        snprintf(g_my->error, sizeof(g_my->error), "Failed to send USE %s", dbname);
        return -1;
    }

    unsigned char resp[MY_BUF_SIZE];
    int rlen;
    if (my_read_packet(resp, &rlen) < 0) {
        snprintf(g_my->error, sizeof(g_my->error), "Failed to read USE response");
        return -1;
    }
    if (resp[0] == 0xFF) {
        uint16_t ec = my_read_u16(resp + 1);
        snprintf(g_my->error, sizeof(g_my->error), "USE %s failed (%d): %.*s",
            dbname, ec, rlen - 9, (char*)(resp + 9));
        return -1;
    }
    my_log("selected database: %s", dbname);
    return 0;
}

int32_t __my_connect(const char* host, int32_t port, const char* dbname,
                     const char* user, const char* password) {
    my_ensure_conn();
    // Set dialect to MySQL (1)
    __db_set_dialect(1);
    __orm_set_dialect(1);

    if (!host) host = "127.0.0.1";
    if (port <= 0) port = 3306;
    if (!dbname) dbname = "";
    if (!user) user = "root";
    if (!password) password = "";

    my_free_results();
    g_my->fd = -1;
    g_my->connected = 0;
    g_my->error[0] = '\0';
    g_my->seq = 0;

    // Store connection params for reconnect
    strncpy(g_my->host, host, sizeof(g_my->host)-1);
    g_my->port = port;
    strncpy(g_my->dbname, dbname, sizeof(g_my->dbname)-1);
    strncpy(g_my->user, user, sizeof(g_my->user)-1);
    strncpy(g_my->password, password, sizeof(g_my->password)-1);

    my_log("connecting to %s:%d db=%s user=%s", host, port, dbname, user);

    // TCP connect
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);

    if (inet_pton(AF_INET, host, &addr.sin_addr) <= 0) {
        struct hostent* he = gethostbyname(host);
        if (!he) {
            snprintf(g_my->error, sizeof(g_my->error), "Cannot resolve host: %s", host);
            return -1;
        }
        memcpy(&addr.sin_addr, he->h_addr_list[0], he->h_length);
    }

    g_my->fd = socket(AF_INET, SOCK_STREAM, 0);
    if (g_my->fd < 0) {
        snprintf(g_my->error, sizeof(g_my->error), "Socket failed: %s", strerror(errno));
        return -1;
    }

    if (db_connect_with_timeout(g_my->fd, (struct sockaddr*)&addr, sizeof(addr),
                                g_db_connect_timeout_ms) < 0) {
        snprintf(g_my->error, sizeof(g_my->error),
                 "Connect timed out after %dms: %s", g_db_connect_timeout_ms, strerror(errno));
        close(g_my->fd); g_my->fd = -1;
        return -1;
    }

    // Apply read timeout so recv() doesn't block forever
    db_set_read_timeout(g_my->fd, g_db_read_timeout_ms);

    my_log("TCP connected fd=%d", g_my->fd);

    // Read server greeting (Initial Handshake Packet)
    unsigned char greeting[MY_BUF_SIZE];
    int glen;
    if (my_read_packet(greeting, &glen) < 0) {
        snprintf(g_my->error, sizeof(g_my->error), "Failed to read greeting");
        close(g_my->fd); g_my->fd = -1;
        return -1;
    }

    // Check for error packet
    if (greeting[0] == 0xFF) {
        uint16_t errcode = my_read_u16(greeting + 1);
        snprintf(g_my->error, sizeof(g_my->error), "Server error %d: %.*s",
            errcode, glen - 9, (char*)(greeting + 9));
        close(g_my->fd); g_my->fd = -1;
        return -1;
    }

    // Parse handshake v10
    uint8_t protocol_version = greeting[0];
    my_log("protocol version: %d", protocol_version);

    // Server version (null-terminated)
    const char* server_version = (const char*)(greeting + 1);
    int sv_len = strlen(server_version);
    my_log("server: %s", server_version);

    unsigned char* p = greeting + 1 + sv_len + 1;
    // Connection ID (4 bytes)
    // uint32_t conn_id = my_read_u32(p);
    p += 4;

    // Auth plugin data part 1 (8 bytes)
    unsigned char scramble[21];
    memcpy(scramble, p, 8);
    p += 8;

    // Filler
    p += 1;

    // Capability flags lower 2 bytes
    uint16_t cap_low = my_read_u16(p);
    p += 2;

    // Character set
    // uint8_t charset = *p;
    p += 1;

    // Status flags
    p += 2;

    // Capability flags upper 2 bytes
    uint16_t cap_high = my_read_u16(p);
    p += 2;
    uint32_t server_caps = cap_low | ((uint32_t)cap_high << 16);

    // Length of auth plugin data
    uint8_t auth_data_len = *p;
    p += 1;

    // Reserved (10 bytes)
    p += 10;

    // Auth plugin data part 2 (at least 13 bytes if secure connection)
    if (server_caps & 0x8000) { // CLIENT_SECURE_CONNECTION
        int part2_len = auth_data_len > 8 ? auth_data_len - 8 : 13;
        if (part2_len > 12) part2_len = 12;
        memcpy(scramble + 8, p, part2_len);
        scramble[8 + part2_len] = '\0';
        p += part2_len + 1; // +1 for null terminator
    }

    // Auth plugin name
    const char* auth_plugin = (const char*)p;
    my_log("auth plugin: %s", auth_plugin);

    // ---- SSL/TLS Negotiation ----
    // If server supports CLIENT_SSL (0x00000800), try to upgrade.
    g_my->use_ssl = 0;
#if MY_HAS_SSL
    if (server_caps & 0x00000800) { // CLIENT_SSL
        // Send SSL Request packet: abbreviated handshake with CLIENT_SSL flag
        unsigned char ssl_req[MY_BUF_SIZE];
        int spos = 0;
        uint32_t ssl_caps =
            0x00000001 | // LONG_PASSWORD
            0x00000200 | // PROTOCOL_41
            0x00008000 | // SECURE_CONNECTION
            0x00000800 | // CLIENT_SSL
            0x00080000 | // PLUGIN_AUTH
            0x00040000;  // MULTI_STATEMENTS
        my_write_u32(ssl_req + spos, ssl_caps); spos += 4;
        my_write_u32(ssl_req + spos, 16777216); spos += 4; // max packet size
        ssl_req[spos++] = 45; // charset = utf8mb4
        memset(ssl_req + spos, 0, 23); spos += 23; // reserved

        if (my_send_packet(ssl_req, spos) < 0) {
            my_log("Failed to send SSL request, continuing plaintext");
            goto skip_ssl;
        }

        // Perform TLS handshake
        SSL_library_init();
        SSL_load_error_strings();
        g_my->ssl_ctx = SSL_CTX_new(TLS_client_method());
        if (g_my->ssl_ctx) {
            g_my->ssl = SSL_new(g_my->ssl_ctx);
            SSL_set_fd(g_my->ssl, g_my->fd);
            if (SSL_connect(g_my->ssl) == 1) {
                g_my->use_ssl = 1;
                my_log("SSL/TLS connection established (%s)", SSL_get_version(g_my->ssl));
            } else {
                my_log("SSL_connect failed, aborting");
                SSL_free(g_my->ssl); g_my->ssl = NULL;
                SSL_CTX_free(g_my->ssl_ctx); g_my->ssl_ctx = NULL;
                snprintf(g_my->error, sizeof(g_my->error), "MySQL SSL handshake failed");
                close(g_my->fd); g_my->fd = -1;
                return -1;
            }
        }
    }
skip_ssl: ;
#endif


    // Build Handshake Response packet
    unsigned char response[MY_BUF_SIZE];
    int rpos = 0;

    // Client capabilities
    uint32_t client_caps =
        0x00000001 | // LONG_PASSWORD
        0x00000200 | // PROTOCOL_41
        0x00008000 | // SECURE_CONNECTION
        0x00080000 | // PLUGIN_AUTH
        0x00200000 | // CONNECT_WITH_DB (if dbname given)
        0x00000008 | // CONNECT_WITH_DB
        0x00040000;  // MULTI_STATEMENTS

    if (dbname[0] == '\0') {
        client_caps &= ~0x00000008;
        client_caps &= ~0x00200000;
    }
    if (g_my->use_ssl) {
        client_caps |= 0x00000800; // CLIENT_SSL
    }

    my_write_u32(response + rpos, client_caps); rpos += 4;
    // Max packet size
    my_write_u32(response + rpos, 16777216); rpos += 4;
    // Character set (utf8mb4 = 45)
    response[rpos++] = 45;
    // Reserved (23 zero bytes)
    memset(response + rpos, 0, 23); rpos += 23;
    // Username (null-terminated)
    int ulen = strlen(user);
    memcpy(response + rpos, user, ulen + 1); rpos += ulen + 1;

    // Auth response
    if (password[0] != '\0' &&
        (strcmp(auth_plugin, "mysql_native_password") == 0 ||
         strcmp(auth_plugin, "caching_sha2_password") == 0)) {
        // mysql_native_password: SHA1 auth
        unsigned char auth_resp[20];
        my_native_auth(password, scramble, 20, auth_resp);
        response[rpos++] = 20; // length-encoded: 20 bytes
        memcpy(response + rpos, auth_resp, 20); rpos += 20;
    } else {
        response[rpos++] = 0; // no auth data
    }

    // Database (if provided)
    if (dbname[0] != '\0') {
        int dlen = strlen(dbname);
        memcpy(response + rpos, dbname, dlen + 1); rpos += dlen + 1;
    }

    // Auth plugin name
    int aplen = strlen(auth_plugin);
    memcpy(response + rpos, auth_plugin, aplen + 1); rpos += aplen + 1;

    // Send handshake response
    if (my_send_packet(response, rpos) < 0) {
        snprintf(g_my->error, sizeof(g_my->error), "Failed to send auth response");
        close(g_my->fd); g_my->fd = -1;
        return -1;
    }

    // Read auth result
    unsigned char auth_result[MY_BUF_SIZE];
    int arlen;
    if (my_read_packet(auth_result, &arlen) < 0) {
        snprintf(g_my->error, sizeof(g_my->error), "Failed to read auth result");
        close(g_my->fd); g_my->fd = -1;
        return -1;
    }

    if (auth_result[0] == 0x00) {
        // OK packet — authenticated
        g_my->connected = 1;
        my_log("authenticated OK");
        goto auth_ok;
    } else if (auth_result[0] == 0x01) {
        // Auth switch or extra data (caching_sha2_password phase 2)
        // For caching_sha2_password: 0x01 0x03 = fast auth ok, 0x01 0x04 = need full auth
        if (arlen >= 2 && auth_result[1] == 0x03) {
            // Fast auth success — read the OK packet
            unsigned char ok[MY_BUF_SIZE];
            int oklen;
            if (my_read_packet(ok, &oklen) < 0 || ok[0] != 0x00) {
                snprintf(g_my->error, sizeof(g_my->error), "Auth confirm failed");
                close(g_my->fd); g_my->fd = -1;
                return -1;
            }
            g_my->connected = 1;
            my_log("caching_sha2 fast auth OK");
            goto auth_ok;
        } else if (arlen >= 2 && auth_result[1] == 0x04) {
            // Full auth needed — send plaintext password over TLS
            // MySQL 8.4+ requires this (mysql_native_password removed)
            if (g_my->use_ssl) {
                // Send plaintext password (null-terminated) over encrypted channel
                int pwlen = strlen(password);
                unsigned char* pw_pkt = malloc(pwlen + 1);
                memcpy(pw_pkt, password, pwlen);
                pw_pkt[pwlen] = '\0';
                if (my_send_packet(pw_pkt, pwlen + 1) < 0) {
                    free(pw_pkt);
                    snprintf(g_my->error, sizeof(g_my->error), "Failed to send full auth password");
                    close(g_my->fd); g_my->fd = -1;
                    return -1;
                }
                free(pw_pkt);

                // Read OK/ERR
                unsigned char ok2[MY_BUF_SIZE];
                int ok2len;
                if (my_read_packet(ok2, &ok2len) < 0 || ok2[0] != 0x00) {
                    if (ok2[0] == 0xFF) {
                        uint16_t ec = my_read_u16(ok2 + 1);
                        snprintf(g_my->error, sizeof(g_my->error), "Auth error %d: %.*s", ec, ok2len - 9, (char*)(ok2 + 9));
                    } else {
                        snprintf(g_my->error, sizeof(g_my->error), "caching_sha2 full auth failed");
                    }
                    close(g_my->fd); g_my->fd = -1;
                    return -1;
                }
                g_my->connected = 1;
                my_log("caching_sha2 full auth over TLS OK");
                goto auth_ok;
            } else {
                snprintf(g_my->error, sizeof(g_my->error),
                    "caching_sha2_password full auth requires TLS (no SSL available). "
                    "Rebuild Desi with OpenSSL support or use mysql_native_password.");
                close(g_my->fd); g_my->fd = -1;
                return -1;
            }
        }
        // Auth method switch
        if (auth_result[0] == 0xFE) {
            snprintf(g_my->error, sizeof(g_my->error), "Auth method switch not yet supported");
            close(g_my->fd); g_my->fd = -1;
            return -1;
        }
        snprintf(g_my->error, sizeof(g_my->error), "Unexpected auth response: 0x%02x", auth_result[0]);
        close(g_my->fd); g_my->fd = -1;
        return -1;
    } else if (auth_result[0] == 0xFE) {
        // Auth switch request  
        // Read plugin name and new scramble
        const char* new_plugin = (const char*)(auth_result + 1);
        int np_len = strlen(new_plugin);
        unsigned char* new_scramble = auth_result + 1 + np_len + 1;
        int ns_len = arlen - 1 - np_len - 1;
        if (ns_len < 0) ns_len = 0;
        if (ns_len > 20) ns_len = 20;
        
        my_log("auth switch to: %s", new_plugin);
        
        if (strcmp(new_plugin, "mysql_native_password") == 0) {
            unsigned char auth_resp[20];
            my_native_auth(password, new_scramble, ns_len, auth_resp);
            if (my_send_packet(auth_resp, 20) < 0) {
                snprintf(g_my->error, sizeof(g_my->error), "Failed to send switched auth");
                close(g_my->fd); g_my->fd = -1;
                return -1;
            }
            // Read OK/ERR
            unsigned char ok[MY_BUF_SIZE];
            int oklen;
            if (my_read_packet(ok, &oklen) < 0) {
                snprintf(g_my->error, sizeof(g_my->error), "Auth switch response failed");
                close(g_my->fd); g_my->fd = -1;
                return -1;
            }
            if (ok[0] == 0x00) {
                g_my->connected = 1;
                my_log("auth switch OK");
                goto auth_ok;
            }
        }
        snprintf(g_my->error, sizeof(g_my->error), "Auth switch to %s failed", new_plugin);
        close(g_my->fd); g_my->fd = -1;
        return -1;
    } else if (auth_result[0] == 0xFF) {
        // Error packet
        uint16_t errcode = my_read_u16(auth_result + 1);
        // Skip SQL state marker (1 byte '#') + state (5 bytes)
        char* msg = (char*)(auth_result + 9);
        int msg_len = arlen - 9;
        if (msg_len < 0) msg_len = 0;
        snprintf(g_my->error, sizeof(g_my->error), "Auth error %d: %.*s", errcode, msg_len, msg);
        close(g_my->fd); g_my->fd = -1;
        return -1;
    }

    snprintf(g_my->error, sizeof(g_my->error), "Unknown auth response: 0x%02x", auth_result[0]);
    close(g_my->fd); g_my->fd = -1;
    return -1;

auth_ok:
    // Explicitly select database — CONNECT_WITH_DB in the handshake is
    // unreliable across MySQL/MariaDB versions and auth-switch paths.
    if (dbname && dbname[0] != '\0') {
        if (my_use_database(dbname) < 0) {
            my_log("warning: USE %s failed: %s", dbname, g_my->error);
            // Non-fatal: connection is still valid, user can USE manually
        }
    }
    return 0;
}

int32_t __my_close(void) {
    if (!g_my) return 0;
    if (g_my->fd >= 0) {
        // COM_QUIT
        unsigned char quit[1] = {0x01};
        g_my->seq = 0xFF; // reset seq so send_packet uses 0
        my_send_packet(quit, 1);
#if MY_HAS_SSL
        if (g_my->ssl) { SSL_shutdown(g_my->ssl); SSL_free(g_my->ssl); g_my->ssl = NULL; }
        if (g_my->ssl_ctx) { SSL_CTX_free(g_my->ssl_ctx); g_my->ssl_ctx = NULL; }
#endif
        g_my->use_ssl = 0;
        close(g_my->fd);
        g_my->fd = -1;
    }
    g_my->connected = 0;
    my_log("connection closed");
    return 0;
}

int32_t __my_is_connected(void) { return g_my ? g_my->connected : 0; }

// Reconnect using stored connection params (called by pool/dispatch on ping failure)
int32_t __my_reconnect(void) {
    if (!g_my) return -1;
    // Close existing dead connection gracefully
    if (g_my->fd >= 0) {
        close(g_my->fd);
        g_my->fd = -1;
    }
    g_my->connected = 0;
    my_log("reconnecting to %s:%d db=%s user=%s",
           g_my->host, g_my->port, g_my->dbname, g_my->user);
    return __my_connect(g_my->host, g_my->port, g_my->dbname,
                        g_my->user, g_my->password);
}

char* __my_last_error(void) { return g_my ? strdup(g_my->error) : strdup(""); }

// ============================================================
// Query
// ============================================================

int32_t __my_query(const char* sql) {
    if (!g_my || !g_my->connected || g_my->fd < 0) {
        if (g_my) snprintf(g_my->error, sizeof(g_my->error), "Not connected");
        return -1;
    }

    my_free_results();
    g_my->error[0] = '\0';

    my_log("query: %s", sql);

    // COM_QUERY: 0x03 + sql
    int sql_len = strlen(sql);
    unsigned char* msg = malloc(1 + sql_len);
    msg[0] = 0x03; // COM_QUERY
    memcpy(msg + 1, sql, sql_len);
    g_my->seq = 0xFF; // reset so send_packet increments to 0
    if (my_send_packet(msg, 1 + sql_len) < 0) {
        free(msg);
        snprintf(g_my->error, sizeof(g_my->error), "Failed to send query");
        return -1;
    }
    free(msg);

    // Read response
    unsigned char pkt[MY_BUF_SIZE];
    int plen;
    if (my_read_packet(pkt, &plen) < 0) {
        snprintf(g_my->error, sizeof(g_my->error), "Failed to read query response");
        return -1;
    }

    if (pkt[0] == 0x00) {
        // OK packet (for INSERT/UPDATE/DELETE)
        int br;
        pkt[0] = 0; // skip header byte
        uint64_t affected = my_read_lenenc(pkt + 1, &br);
        my_log("OK affected_rows=%llu", (unsigned long long)affected);
        return (int32_t)affected;
    }

    if (pkt[0] == 0xFF) {
        // Error packet
        uint16_t errcode = my_read_u16(pkt + 1);
        char* errmsg = (char*)(pkt + 9);
        int elen = plen - 9;
        if (elen < 0) elen = 0;
        snprintf(g_my->error, sizeof(g_my->error), "MySQL error %d: %.*s", errcode, elen, errmsg);
        my_log("error: %s", g_my->error);
        return -1;
    }

    // Result Set: first packet is column count
    int br;
    uint64_t ncols = my_read_lenenc(pkt, &br);
    g_my->ncols = (int)ncols;
    my_log("result set: %d columns", g_my->ncols);

    // Read column definitions
    for (int i = 0; i < g_my->ncols && i < MY_MAX_COLS; i++) {
        if (my_read_packet(pkt, &plen) < 0) return -1;
        unsigned char* cp = pkt;
        // Skip: catalog, schema, table, org_table
        for (int skip = 0; skip < 4; skip++) {
            uint64_t slen = my_read_lenenc(cp, &br);
            cp += br + (int)slen;
        }
        // Column name (length-encoded string)
        uint64_t name_len = my_read_lenenc(cp, &br);
        cp += br;
        int nlen = (int)name_len;
        if (nlen > 127) nlen = 127;
        memcpy(g_my->colnames[i], cp, nlen);
        g_my->colnames[i][nlen] = '\0';
        my_log("  col %d: %s", i, g_my->colnames[i]);
    }

    // Check for deprecation marker (EOF for protocol < 4.1) or skip
    if (my_read_packet(pkt, &plen) < 0) return -1;
    // If it's an EOF packet (0xFE), that marks end of column defs
    // If it's already a row, we process it

    // Read rows until EOF
    int processing_rows = 1;
    // If the packet we just read is already a row (not EOF), process it first
    if (pkt[0] != 0xFE || plen > 5) {
        // This is a row packet, process it
        goto process_row;
    }

    while (processing_rows) {
        if (my_read_packet(pkt, &plen) < 0) return -1;

    process_row:
        if (pkt[0] == 0xFE && plen <= 5) {
            // EOF packet — end of rows
            my_log("EOF rows=%d", g_my->nrows);
            break;
        }
        if (pkt[0] == 0xFF) {
            // Error
            snprintf(g_my->error, sizeof(g_my->error), "Error during row fetch");
            return -1;
        }

        // Row data: sequence of length-encoded strings
        int row_base = g_my->nrows * g_my->ncols;
        my_ensure_cells(row_base + g_my->ncols);
        unsigned char* rp = pkt;
        for (int i = 0; i < g_my->ncols; i++) {
            if (*rp == 0xFB) {
                // NULL
                g_my->cells[row_base + i] = strdup("");
                rp++;
            } else {
                uint64_t vlen = my_read_lenenc(rp, &br);
                rp += br;
                int vl = (int)vlen;
                g_my->cells[row_base + i] = malloc(vl + 1);
                memcpy(g_my->cells[row_base + i], rp, vl);
                g_my->cells[row_base + i][vl] = '\0';
                rp += vl;
            }
        }
        g_my->nrows++;
    }

    return g_my->nrows;
}

int32_t __my_execute(const char* sql) { return __my_query(sql); }

// ============================================================
// Parameterized Query — Client-side escaping for MySQL
//
// MySQL's COM_STMT_PREPARE (binary protocol) is complex to implement
// in a raw wire protocol client. Instead, we use client-side escaping
// (equivalent to mysql_real_escape_string) and send via COM_QUERY.
//
// This provides the SAME SQL injection protection:
//   - Single quotes are doubled: ' → ''
//   - Backslashes are escaped: \ → \\
//   - The escaped value is wrapped in single quotes
//
// The escaped SQL is then sent via the normal COM_QUERY path.
// ============================================================

// Escape a string value for safe MySQL interpolation
// Returns heap-allocated escaped string (caller must free)
static char* my_escape_value(const char* val) {
    if (!val) return strdup("NULL");

    int vlen = strlen(val);
    // Worst case: every char doubles + 2 quotes + null
    char* out = malloc(vlen * 2 + 3);
    if (!out) return strdup("''");

    int pos = 0;
    out[pos++] = '\'';
    for (int i = 0; i < vlen; i++) {
        switch (val[i]) {
            case '\'':
                out[pos++] = '\''; out[pos++] = '\'';  // '' escaping
                break;
            case '\\':
                out[pos++] = '\\'; out[pos++] = '\\';  // \\ escaping
                break;
            case '\0':
                out[pos++] = '\\'; out[pos++] = '0';
                break;
            case '\n':
                out[pos++] = '\\'; out[pos++] = 'n';
                break;
            case '\r':
                out[pos++] = '\\'; out[pos++] = 'r';
                break;
            case '\x1a':  // Ctrl-Z (EOF on Windows)
                out[pos++] = '\\'; out[pos++] = 'Z';
                break;
            default:
                out[pos++] = val[i];
                break;
        }
    }
    out[pos++] = '\'';
    out[pos] = '\0';
    return out;
}

// Replace ? placeholders in SQL with escaped values
// Returns heap-allocated SQL string (caller must free)
static char* my_interpolate_params(const char* sql, const char** params, int nparams) {
    // Estimate output size
    int sql_len = strlen(sql);
    int total = sql_len;
    for (int i = 0; i < nparams; i++) {
        total += params[i] ? strlen(params[i]) * 2 + 3 : 4;  // escaped value or NULL
    }

    char* out = malloc(total + 1);
    if (!out) return strdup(sql);

    int opos = 0;
    int pidx = 0;
    int in_quote = 0;  // inside single-quoted string literal

    for (int i = 0; i < sql_len; i++) {
        if (sql[i] == '\\' && in_quote && i + 1 < sql_len) {
            // Backslash escape inside quotes — copy both chars verbatim
            out[opos++] = sql[i];
            out[opos++] = sql[++i];
            continue;
        }
        if (sql[i] == '\'') {
            in_quote = !in_quote;
            out[opos++] = sql[i];
            continue;
        }
        if (sql[i] == '?' && !in_quote && pidx < nparams) {
            // Replace ? with escaped value (only outside quotes)
            char* escaped = my_escape_value(params[pidx]);
            int elen = strlen(escaped);
            memcpy(out + opos, escaped, elen);
            opos += elen;
            free(escaped);
            pidx++;
        } else {
            out[opos++] = sql[i];
        }
    }
    out[opos] = '\0';
    return out;
}

int32_t __my_query_params(const char* sql, const char** params, int nparams) {
    // Build escaped SQL and send via normal COM_QUERY
    char* safe_sql = my_interpolate_params(sql, params, nparams);
    my_log("query_params: %s", safe_sql);
    int32_t result = __my_query(safe_sql);
    free(safe_sql);
    return result;
}

// ============================================================
// Result Access
// ============================================================

int32_t __my_row_count(void) { return g_my ? g_my->nrows : 0; }
int32_t __my_col_count(void) { return g_my ? g_my->ncols : 0; }

char* __my_col_name(int32_t idx) {
    if (!g_my || idx < 0 || idx >= g_my->ncols) return strdup("");
    return strdup(g_my->colnames[idx]);
}

char* __my_get_value(int32_t row, int32_t col) {
    if (!g_my || row < 0 || row >= g_my->nrows || col < 0 || col >= g_my->ncols) return strdup("");
    if (!g_my->cells) return strdup("");
    char* v = g_my->cells[row * g_my->ncols + col];
    return strdup(v ? v : "");
}

char* __my_get_field(int32_t row, const char* name) {
    if (!g_my || !name) return strdup("");
    for (int i = 0; i < g_my->ncols; i++) {
        if (strcmp(g_my->colnames[i], name) == 0)
            return __my_get_value(row, i);
    }
    return strdup("");
}

// ============================================================
// Debug Helpers
// ============================================================

int32_t __my_dump_results(void) {
    if (!g_my) { fprintf(stderr, "=== MySQL: no connection ===\n"); return 0; }
    fprintf(stderr, "=== MySQL Result: %d rows x %d cols ===\n", g_my->nrows, g_my->ncols);
    for (int c = 0; c < g_my->ncols; c++) {
        if (c > 0) fprintf(stderr, " | ");
        fprintf(stderr, "%-15s", g_my->colnames[c]);
    }
    fprintf(stderr, "\n");
    for (int c = 0; c < g_my->ncols; c++) {
        if (c > 0) fprintf(stderr, "-+-");
        fprintf(stderr, "---------------");
    }
    fprintf(stderr, "\n");
    for (int r = 0; r < g_my->nrows; r++) {
        for (int c = 0; c < g_my->ncols; c++) {
            if (c > 0) fprintf(stderr, " | ");
            char* v = g_my->cells ? g_my->cells[r * g_my->ncols + c] : NULL;
            fprintf(stderr, "%-15s", v ? v : "(null)");
        }
        fprintf(stderr, "\n");
    }
    fprintf(stderr, "===\n");
    return 0;
}

char* __my_connection_info(void) {
    if (!g_my) return strdup("not initialized");
    char buf[512];
    snprintf(buf, sizeof(buf), "fd=%d connected=%d error=%s",
        g_my->fd, g_my->connected, g_my->error);
    return strdup(buf);
}
