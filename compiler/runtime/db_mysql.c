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
// Connection State
// ============================================================

#define MY_MAX_COLS 64
#define MY_MAX_ROWS 4096
#define MY_BUF_SIZE 65536

static int g_my_fd = -1;
static int g_my_connected = 0;
static int g_my_debug = 0;
static char g_my_error[512] = "";
static uint8_t g_my_seq = 0; // packet sequence number

// Result set
static int g_my_ncols = 0;
static char g_my_colnames[MY_MAX_COLS][128];
static int g_my_nrows = 0;
static char** g_my_cells = NULL;
static int g_my_cells_cap = 0;

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
    if (g_my_fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n = (int)write(g_my_fd, data + total, len - total);
        if (n <= 0) return -1;
        total += n;
    }
    return 0;
}

static int my_recv_raw(unsigned char* buf, int len) {
    if (g_my_fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n = (int)read(g_my_fd, buf + total, len - total);
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
    g_my_seq = header[3];
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
    my_log("recv packet seq=%d len=%d type=0x%02x", g_my_seq, plen, plen > 0 ? payload[0] : 0);
    return 0;
}

static int my_send_packet(const unsigned char* payload, int len) {
    unsigned char header[4];
    my_write_u24(header, len);
    header[3] = ++g_my_seq;
    if (my_send_raw(header, 4) < 0) return -1;
    if (my_send_raw(payload, len) < 0) return -1;
    my_log("sent packet seq=%d len=%d", g_my_seq, len);
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
    if (g_my_cells) {
        for (int i = 0; i < g_my_nrows * g_my_ncols; i++) {
            if (g_my_cells[i]) free(g_my_cells[i]);
        }
        free(g_my_cells);
        g_my_cells = NULL;
    }
    g_my_nrows = 0;
    g_my_ncols = 0;
    g_my_cells_cap = 0;
}

static void my_ensure_cells(int needed) {
    if (needed <= g_my_cells_cap) return;
    int newcap = needed * 2;
    g_my_cells = realloc(g_my_cells, newcap * sizeof(char*));
    for (int i = g_my_cells_cap; i < newcap; i++) g_my_cells[i] = NULL;
    g_my_cells_cap = newcap;
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

int32_t __my_connect(const char* host, int32_t port, const char* dbname,
                     const char* user, const char* password) {
    // Set dialect to MySQL (1)
    __db_set_dialect(1);
    __orm_set_dialect(1);

    if (!host) host = "127.0.0.1";
    if (port <= 0) port = 3306;
    if (!dbname) dbname = "";
    if (!user) user = "root";
    if (!password) password = "";

    my_free_results();
    g_my_fd = -1;
    g_my_connected = 0;
    g_my_error[0] = '\0';
    g_my_seq = 0;

    my_log("connecting to %s:%d db=%s user=%s", host, port, dbname, user);

    // TCP connect
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);

    if (inet_pton(AF_INET, host, &addr.sin_addr) <= 0) {
        struct hostent* he = gethostbyname(host);
        if (!he) {
            snprintf(g_my_error, sizeof(g_my_error), "Cannot resolve host: %s", host);
            return -1;
        }
        memcpy(&addr.sin_addr, he->h_addr_list[0], he->h_length);
    }

    g_my_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (g_my_fd < 0) {
        snprintf(g_my_error, sizeof(g_my_error), "Socket failed: %s", strerror(errno));
        return -1;
    }

    if (connect(g_my_fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        snprintf(g_my_error, sizeof(g_my_error), "Connect failed: %s", strerror(errno));
        close(g_my_fd); g_my_fd = -1;
        return -1;
    }

    my_log("TCP connected fd=%d", g_my_fd);

    // Read server greeting (Initial Handshake Packet)
    unsigned char greeting[MY_BUF_SIZE];
    int glen;
    if (my_read_packet(greeting, &glen) < 0) {
        snprintf(g_my_error, sizeof(g_my_error), "Failed to read greeting");
        close(g_my_fd); g_my_fd = -1;
        return -1;
    }

    // Check for error packet
    if (greeting[0] == 0xFF) {
        uint16_t errcode = my_read_u16(greeting + 1);
        snprintf(g_my_error, sizeof(g_my_error), "Server error %d: %.*s",
            errcode, glen - 9, (char*)(greeting + 9));
        close(g_my_fd); g_my_fd = -1;
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
        snprintf(g_my_error, sizeof(g_my_error), "Failed to send auth response");
        close(g_my_fd); g_my_fd = -1;
        return -1;
    }

    // Read auth result
    unsigned char auth_result[MY_BUF_SIZE];
    int arlen;
    if (my_read_packet(auth_result, &arlen) < 0) {
        snprintf(g_my_error, sizeof(g_my_error), "Failed to read auth result");
        close(g_my_fd); g_my_fd = -1;
        return -1;
    }

    if (auth_result[0] == 0x00) {
        // OK packet — authenticated
        g_my_connected = 1;
        my_log("authenticated OK");
        return 0;
    } else if (auth_result[0] == 0x01) {
        // Auth switch or extra data (caching_sha2_password phase 2)
        // For caching_sha2_password: 0x01 0x03 = fast auth ok, 0x01 0x04 = need full auth
        if (arlen >= 2 && auth_result[1] == 0x03) {
            // Fast auth success — read the OK packet
            unsigned char ok[MY_BUF_SIZE];
            int oklen;
            if (my_read_packet(ok, &oklen) < 0 || ok[0] != 0x00) {
                snprintf(g_my_error, sizeof(g_my_error), "Auth confirm failed");
                close(g_my_fd); g_my_fd = -1;
                return -1;
            }
            g_my_connected = 1;
            my_log("caching_sha2 fast auth OK");
            return 0;
        } else if (arlen >= 2 && auth_result[1] == 0x04) {
            // Full auth needed — send plaintext password over non-TLS
            // This requires a secure connection (TLS); skip for now
            snprintf(g_my_error, sizeof(g_my_error),
                "caching_sha2_password full auth requires TLS. "
                "Set 'default_authentication_plugin=mysql_native_password' in my.cnf "
                "or create user with mysql_native_password");
            close(g_my_fd); g_my_fd = -1;
            return -1;
        }
        // Auth method switch
        if (auth_result[0] == 0xFE) {
            snprintf(g_my_error, sizeof(g_my_error), "Auth method switch not yet supported");
            close(g_my_fd); g_my_fd = -1;
            return -1;
        }
        snprintf(g_my_error, sizeof(g_my_error), "Unexpected auth response: 0x%02x", auth_result[0]);
        close(g_my_fd); g_my_fd = -1;
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
                snprintf(g_my_error, sizeof(g_my_error), "Failed to send switched auth");
                close(g_my_fd); g_my_fd = -1;
                return -1;
            }
            // Read OK/ERR
            unsigned char ok[MY_BUF_SIZE];
            int oklen;
            if (my_read_packet(ok, &oklen) < 0) {
                snprintf(g_my_error, sizeof(g_my_error), "Auth switch response failed");
                close(g_my_fd); g_my_fd = -1;
                return -1;
            }
            if (ok[0] == 0x00) {
                g_my_connected = 1;
                my_log("auth switch OK");
                return 0;
            }
        }
        snprintf(g_my_error, sizeof(g_my_error), "Auth switch to %s failed", new_plugin);
        close(g_my_fd); g_my_fd = -1;
        return -1;
    } else if (auth_result[0] == 0xFF) {
        // Error packet
        uint16_t errcode = my_read_u16(auth_result + 1);
        // Skip SQL state marker (1 byte '#') + state (5 bytes)
        char* msg = (char*)(auth_result + 9);
        int msg_len = arlen - 9;
        if (msg_len < 0) msg_len = 0;
        snprintf(g_my_error, sizeof(g_my_error), "Auth error %d: %.*s", errcode, msg_len, msg);
        close(g_my_fd); g_my_fd = -1;
        return -1;
    }

    snprintf(g_my_error, sizeof(g_my_error), "Unknown auth response: 0x%02x", auth_result[0]);
    close(g_my_fd); g_my_fd = -1;
    return -1;
}

int32_t __my_close(void) {
    if (g_my_fd >= 0) {
        // COM_QUIT
        unsigned char quit[1] = {0x01};
        g_my_seq = 0xFF; // reset seq so send_packet uses 0
        my_send_packet(quit, 1);
        close(g_my_fd);
        g_my_fd = -1;
    }
    g_my_connected = 0;
    my_log("connection closed");
    return 0;
}

int32_t __my_is_connected(void) { return g_my_connected; }

char* __my_last_error(void) { return strdup(g_my_error); }

// ============================================================
// Query
// ============================================================

int32_t __my_query(const char* sql) {
    if (!g_my_connected || g_my_fd < 0) {
        snprintf(g_my_error, sizeof(g_my_error), "Not connected");
        return -1;
    }

    my_free_results();
    g_my_error[0] = '\0';

    my_log("query: %s", sql);

    // COM_QUERY: 0x03 + sql
    int sql_len = strlen(sql);
    unsigned char* msg = malloc(1 + sql_len);
    msg[0] = 0x03; // COM_QUERY
    memcpy(msg + 1, sql, sql_len);
    g_my_seq = 0xFF; // reset so send_packet increments to 0
    if (my_send_packet(msg, 1 + sql_len) < 0) {
        free(msg);
        snprintf(g_my_error, sizeof(g_my_error), "Failed to send query");
        return -1;
    }
    free(msg);

    // Read response
    unsigned char pkt[MY_BUF_SIZE];
    int plen;
    if (my_read_packet(pkt, &plen) < 0) {
        snprintf(g_my_error, sizeof(g_my_error), "Failed to read query response");
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
        snprintf(g_my_error, sizeof(g_my_error), "MySQL error %d: %.*s", errcode, elen, errmsg);
        my_log("error: %s", g_my_error);
        return -1;
    }

    // Result Set: first packet is column count
    int br;
    uint64_t ncols = my_read_lenenc(pkt, &br);
    g_my_ncols = (int)ncols;
    my_log("result set: %d columns", g_my_ncols);

    // Read column definitions
    for (int i = 0; i < g_my_ncols && i < MY_MAX_COLS; i++) {
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
        memcpy(g_my_colnames[i], cp, nlen);
        g_my_colnames[i][nlen] = '\0';
        my_log("  col %d: %s", i, g_my_colnames[i]);
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
            my_log("EOF rows=%d", g_my_nrows);
            break;
        }
        if (pkt[0] == 0xFF) {
            // Error
            snprintf(g_my_error, sizeof(g_my_error), "Error during row fetch");
            return -1;
        }

        // Row data: sequence of length-encoded strings
        int row_base = g_my_nrows * g_my_ncols;
        my_ensure_cells(row_base + g_my_ncols);
        unsigned char* rp = pkt;
        for (int i = 0; i < g_my_ncols; i++) {
            if (*rp == 0xFB) {
                // NULL
                g_my_cells[row_base + i] = strdup("");
                rp++;
            } else {
                uint64_t vlen = my_read_lenenc(rp, &br);
                rp += br;
                int vl = (int)vlen;
                g_my_cells[row_base + i] = malloc(vl + 1);
                memcpy(g_my_cells[row_base + i], rp, vl);
                g_my_cells[row_base + i][vl] = '\0';
                rp += vl;
            }
        }
        g_my_nrows++;
    }

    return g_my_nrows;
}

int32_t __my_execute(const char* sql) { return __my_query(sql); }

// ============================================================
// Result Access
// ============================================================

int32_t __my_row_count(void) { return g_my_nrows; }
int32_t __my_col_count(void) { return g_my_ncols; }

char* __my_col_name(int32_t idx) {
    if (idx < 0 || idx >= g_my_ncols) return strdup("");
    return strdup(g_my_colnames[idx]);
}

char* __my_get_value(int32_t row, int32_t col) {
    if (row < 0 || row >= g_my_nrows || col < 0 || col >= g_my_ncols) return strdup("");
    if (!g_my_cells) return strdup("");
    char* v = g_my_cells[row * g_my_ncols + col];
    return strdup(v ? v : "");
}

char* __my_get_field(int32_t row, const char* name) {
    if (!name) return strdup("");
    for (int i = 0; i < g_my_ncols; i++) {
        if (strcmp(g_my_colnames[i], name) == 0)
            return __my_get_value(row, i);
    }
    return strdup("");
}

// ============================================================
// Debug Helpers
// ============================================================

int32_t __my_dump_results(void) {
    fprintf(stderr, "=== MySQL Result: %d rows x %d cols ===\n", g_my_nrows, g_my_ncols);
    for (int c = 0; c < g_my_ncols; c++) {
        if (c > 0) fprintf(stderr, " | ");
        fprintf(stderr, "%-15s", g_my_colnames[c]);
    }
    fprintf(stderr, "\n");
    for (int c = 0; c < g_my_ncols; c++) {
        if (c > 0) fprintf(stderr, "-+-");
        fprintf(stderr, "---------------");
    }
    fprintf(stderr, "\n");
    for (int r = 0; r < g_my_nrows; r++) {
        for (int c = 0; c < g_my_ncols; c++) {
            if (c > 0) fprintf(stderr, " | ");
            char* v = g_my_cells ? g_my_cells[r * g_my_ncols + c] : NULL;
            fprintf(stderr, "%-15s", v ? v : "(null)");
        }
        fprintf(stderr, "\n");
    }
    fprintf(stderr, "===\n");
    return 0;
}

char* __my_connection_info(void) {
    char buf[512];
    snprintf(buf, sizeof(buf), "fd=%d connected=%d error=%s",
        g_my_fd, g_my_connected, g_my_error);
    return strdup(buf);
}
