/*
 * db_postgres.c — Pure C PostgreSQL Wire Protocol v3 Client
 *
 * All internal functions prefixed with pg_ to avoid linker conflicts.
 * Debug mode: set g_pg_debug=1 for verbose protocol logging.
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
// Byte order helpers (prefixed to avoid conflicts)
// ============================================================

static void pg_write_i32(char* buf, int32_t val) {
    buf[0] = (val >> 24) & 0xFF;
    buf[1] = (val >> 16) & 0xFF;
    buf[2] = (val >> 8) & 0xFF;
    buf[3] = val & 0xFF;
}

static int32_t pg_read_i32(const char* buf) {
    return ((unsigned char)buf[0] << 24) |
           ((unsigned char)buf[1] << 16) |
           ((unsigned char)buf[2] << 8) |
           (unsigned char)buf[3];
}

static int16_t pg_read_i16(const char* buf) {
    return ((unsigned char)buf[0] << 8) | (unsigned char)buf[1];
}

// ============================================================
// Connection State — simple globals (no large struct)
// ============================================================

#define PG_MAX_COLS 64
#define PG_MAX_ROWS 4096
#define PG_BUF_SIZE 65536

// Connection
static int g_pg_fd = -1;
static int g_pg_connected = 0;
static int g_pg_debug = 0;
static char g_pg_error[512] = "";
static char g_pg_user[128] = "";
static char g_pg_password[256] = "";

// Result set
static int g_pg_ncols = 0;
static char g_pg_colnames[PG_MAX_COLS][128];
static int g_pg_nrows = 0;
static char** g_pg_cells = NULL; // flat array: [row * ncols + col]
static int g_pg_cells_cap = 0;

// Debug log
static void pg_log(const char* fmt, ...) {
    if (!g_pg_debug) return;
    va_list ap;
    va_start(ap, fmt);
    fprintf(stderr, "[pg] ");
    vfprintf(stderr, fmt, ap);
    fprintf(stderr, "\n");
    va_end(ap);
}

// ============================================================
// Enable/disable debug mode
// ============================================================

int32_t __pg_set_debug(int32_t enabled) {
    g_pg_debug = enabled;
    return 0;
}

// ============================================================
// Low-level I/O
// ============================================================

static int pg_send_raw(const char* data, int len) {
    if (g_pg_fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n = (int)write(g_pg_fd, data + total, len - total);
        if (n <= 0) return -1;
        total += n;
    }
    return 0;
}

static int pg_recv_raw(char* buf, int len) {
    if (g_pg_fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n = (int)read(g_pg_fd, buf + total, len - total);
        if (n <= 0) return -1;
        total += n;
    }
    return 0;
}

// Read a PG message: type(1) + len(4) + payload
static int pg_read_msg(char* out_type, char* payload, int* out_plen) {
    if (pg_recv_raw(out_type, 1) < 0) return -1;
    char lb[4];
    if (pg_recv_raw(lb, 4) < 0) return -1;
    int32_t total_len = pg_read_i32(lb);
    int plen = total_len - 4;
    if (plen < 0) plen = 0;
    if (plen > 0 && plen < PG_BUF_SIZE) {
        if (pg_recv_raw(payload, plen) < 0) return -1;
    } else if (plen >= PG_BUF_SIZE) {
        // Too large — drain it
        char drain[4096];
        int rem = plen;
        while (rem > 0) {
            int chunk = rem > 4096 ? 4096 : rem;
            if (pg_recv_raw(drain, chunk) < 0) return -1;
            rem -= chunk;
        }
    }
    *out_plen = plen < PG_BUF_SIZE ? plen : 0;
    pg_log("recv msg type='%c' len=%d payload=%d", *out_type, total_len, plen);
    return 0;
}

// ============================================================
// MD5 Authentication (inline, prefixed)
// ============================================================

#define PG_ROTL32(x,n) (((x)<<(n))|((x)>>(32-(n))))

static const uint32_t pg_md5_T[64] = {
    0xd76aa478,0xe8c7b756,0x242070db,0xc1bdceee,0xf57c0faf,0x4787c62a,0xa8304613,0xfd469501,
    0x698098d8,0x8b44f7af,0xffff5bb1,0x895cd7be,0x6b901122,0xfd987193,0xa679438e,0x49b40821,
    0xf61e2562,0xc040b340,0x265e5a51,0xe9b6c7aa,0xd62f105d,0x02441453,0xd8a1e681,0xe7d3fbc8,
    0x21e1cde6,0xc33707d6,0xf4d50d87,0x455a14ed,0xa9e3e905,0xfcefa3f8,0x676f02d9,0x8d2a4c8a,
    0xfffa3942,0x8771f681,0x6d9d6122,0xfde5380c,0xa4beea44,0x4bdecfa9,0xf6bb4b60,0xbebfbc70,
    0x289b7ec6,0xeaa127fa,0xd4ef3085,0x04881d05,0xd9d4d039,0xe6db99e5,0x1fa27cf8,0xc4ac5665,
    0xf4292244,0x432aff97,0xab9423a7,0xfc93a039,0x655b59c3,0x8f0ccc92,0xffeff47d,0x85845dd1,
    0x6fa87e4f,0xfe2ce6e0,0xa3014314,0x4e0811a1,0xf7537e82,0xbd3af235,0x2ad7d2bb,0xeb86d391
};
static const int pg_md5_s[64] = {
    7,12,17,22,7,12,17,22,7,12,17,22,7,12,17,22,
    5,9,14,20,5,9,14,20,5,9,14,20,5,9,14,20,
    4,11,16,23,4,11,16,23,4,11,16,23,4,11,16,23,
    6,10,15,21,6,10,15,21,6,10,15,21,6,10,15,21
};

static void pg_md5_block(uint32_t st[4], const unsigned char blk[64]) {
    uint32_t a=st[0],b=st[1],c=st[2],d=st[3], M[16];
    for(int i=0;i<16;i++)
        M[i]=(uint32_t)blk[i*4]|((uint32_t)blk[i*4+1]<<8)|((uint32_t)blk[i*4+2]<<16)|((uint32_t)blk[i*4+3]<<24);
    for(int i=0;i<64;i++){
        uint32_t f,g;
        if(i<16){f=(b&c)|((~b)&d);g=i;}
        else if(i<32){f=(d&b)|((~d)&c);g=(5*i+1)%16;}
        else if(i<48){f=b^c^d;g=(3*i+5)%16;}
        else{f=c^(b|(~d));g=(7*i)%16;}
        uint32_t t=d;d=c;c=b;b=b+PG_ROTL32(a+f+pg_md5_T[i]+M[g],pg_md5_s[i]);a=t;
    }
    st[0]+=a;st[1]+=b;st[2]+=c;st[3]+=d;
}

static void pg_md5(const unsigned char* data, size_t len, char hex[33]) {
    uint32_t st[4]={0x67452301,0xefcdab89,0x98badcfe,0x10325476};
    unsigned char buf[64];
    size_t i;
    for(i=0;i+64<=len;i+=64) pg_md5_block(st,(const unsigned char*)(data+i));
    size_t rem=len-i;
    memset(buf,0,64);
    memcpy(buf,data+i,rem);
    buf[rem]=0x80;
    if(rem>=56){pg_md5_block(st,buf);memset(buf,0,64);}
    uint64_t bits=len*8;
    for(int j=0;j<8;j++) buf[56+j]=(bits>>(j*8))&0xFF;
    pg_md5_block(st,buf);
    unsigned char dig[16];
    for(int j=0;j<4;j++){dig[j*4]=st[j]&0xFF;dig[j*4+1]=(st[j]>>8)&0xFF;dig[j*4+2]=(st[j]>>16)&0xFF;dig[j*4+3]=(st[j]>>24)&0xFF;}
    for(int j=0;j<16;j++) sprintf(hex+j*2,"%02x",dig[j]);
    hex[32]='\0';
}

static void pg_md5_auth(const char* user, const char* pass, const char salt[4], char out[36]) {
    // md5(md5(password+user)+salt)
    size_t plen=strlen(pass), ulen=strlen(user);
    unsigned char* tmp = malloc(plen+ulen);
    memcpy(tmp, pass, plen);
    memcpy(tmp+plen, user, ulen);
    char h1[33];
    pg_md5(tmp, plen+ulen, h1);
    free(tmp);

    unsigned char tmp2[36];
    memcpy(tmp2, h1, 32);
    memcpy(tmp2+32, salt, 4);
    char h2[33];
    pg_md5(tmp2, 36, h2);
    sprintf(out, "md5%s", h2);
}

// ============================================================
// Result set management
// ============================================================

static void pg_free_results(void) {
    if (g_pg_cells) {
        for (int i = 0; i < g_pg_nrows * g_pg_ncols; i++) {
            if (g_pg_cells[i]) free(g_pg_cells[i]);
        }
        free(g_pg_cells);
        g_pg_cells = NULL;
    }
    g_pg_nrows = 0;
    g_pg_ncols = 0;
    g_pg_cells_cap = 0;
}

static void pg_ensure_cells(int needed) {
    if (needed <= g_pg_cells_cap) return;
    int newcap = needed * 2;
    g_pg_cells = realloc(g_pg_cells, newcap * sizeof(char*));
    for (int i = g_pg_cells_cap; i < newcap; i++) g_pg_cells[i] = NULL;
    g_pg_cells_cap = newcap;
}

// ============================================================
// Connect
// ============================================================

// Forward declarations from db_query.c and db_orm.c
extern int32_t __db_set_dialect(int32_t dialect);
extern int32_t __orm_set_dialect(int32_t dialect);

int32_t __pg_connect(const char* host, int32_t port, const char* dbname,
                     const char* user, const char* password) {
    // Set dialect to Postgres (0) — done here in C because
    // Desi module wrappers must only make a single extern call.
    __db_set_dialect(0);
    __orm_set_dialect(0);
    if (!host) host = "127.0.0.1";
    if (port <= 0) port = 5432;
    if (!dbname) dbname = "postgres";
    if (!user) user = "postgres";
    if (!password) password = "";

    strncpy(g_pg_user, user, sizeof(g_pg_user)-1);
    strncpy(g_pg_password, password, sizeof(g_pg_password)-1);

    // Reset
    pg_free_results();
    g_pg_fd = -1;
    g_pg_connected = 0;
    g_pg_error[0] = '\0';

    pg_log("connecting to %s:%d db=%s user=%s", host, port, dbname, user);

    // TCP connect
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);

    if (inet_pton(AF_INET, host, &addr.sin_addr) <= 0) {
        struct hostent* he = gethostbyname(host);
        if (!he) {
            snprintf(g_pg_error, sizeof(g_pg_error), "Cannot resolve host: %s", host);
            return -1;
        }
        memcpy(&addr.sin_addr, he->h_addr_list[0], he->h_length);
    }

    g_pg_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (g_pg_fd < 0) {
        snprintf(g_pg_error, sizeof(g_pg_error), "Socket failed: %s", strerror(errno));
        return -1;
    }

    if (connect(g_pg_fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        snprintf(g_pg_error, sizeof(g_pg_error), "Connect failed: %s", strerror(errno));
        close(g_pg_fd); g_pg_fd = -1;
        return -1;
    }

    pg_log("TCP connected fd=%d", g_pg_fd);

    // StartupMessage
    char startup[512];
    int pos = 4; // skip length
    pg_write_i32(startup + pos, 196608); pos += 4; // version 3.0
    memcpy(startup+pos,"user",5); pos+=5;
    int ul = strlen(user); memcpy(startup+pos,user,ul+1); pos+=ul+1;
    memcpy(startup+pos,"database",9); pos+=9;
    int dl = strlen(dbname); memcpy(startup+pos,dbname,dl+1); pos+=dl+1;
    memcpy(startup+pos,"client_encoding",16); pos+=16;
    memcpy(startup+pos,"UTF8",5); pos+=5;
    startup[pos++] = '\0';
    pg_write_i32(startup, pos);

    if (pg_send_raw(startup, pos) < 0) {
        snprintf(g_pg_error, sizeof(g_pg_error), "Failed to send startup");
        close(g_pg_fd); g_pg_fd = -1;
        return -1;
    }

    pg_log("StartupMessage sent (%d bytes)", pos);

    // Auth loop
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;

    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) {
            snprintf(g_pg_error, sizeof(g_pg_error), "Failed to read auth response");
            close(g_pg_fd); g_pg_fd = -1;
            return -1;
        }

        if (mtype == 'R') {
            int32_t atype = pg_read_i32(payload);
            pg_log("auth type=%d", atype);
            if (atype == 0) {
                // AuthenticationOk
                pg_log("auth OK");
                continue;
            } else if (atype == 3) {
                // Cleartext password
                char msg[512]; int mp = 0;
                msg[mp++] = 'p';
                int pwl = strlen(password);
                pg_write_i32(msg+mp, 4+pwl+1); mp+=4;
                memcpy(msg+mp, password, pwl+1); mp+=pwl+1;
                pg_send_raw(msg, mp);
                pg_log("sent cleartext password");
            } else if (atype == 5) {
                // MD5
                char salt[4]; memcpy(salt, payload+4, 4);
                char md5res[36];
                pg_md5_auth(user, password, salt, md5res);
                char msg[512]; int mp = 0;
                msg[mp++] = 'p';
                int rl = strlen(md5res);
                pg_write_i32(msg+mp, 4+rl+1); mp+=4;
                memcpy(msg+mp, md5res, rl+1); mp+=rl+1;
                pg_send_raw(msg, mp);
                pg_log("sent MD5 password");
            } else {
                snprintf(g_pg_error, sizeof(g_pg_error),
                    "Unsupported auth method: %d", atype);
                close(g_pg_fd); g_pg_fd = -1;
                return -1;
            }
        } else if (mtype == 'E') {
            char* p = payload;
            while (p < payload + plen && *p) {
                char f = *p++;
                if (f == '\0') break;
                if (f == 'M') { snprintf(g_pg_error, sizeof(g_pg_error), "%s", p); break; }
                p += strlen(p) + 1;
            }
            pg_log("auth error: %s", g_pg_error);
            close(g_pg_fd); g_pg_fd = -1;
            return -1;
        } else if (mtype == 'K' || mtype == 'S') {
            // BackendKeyData / ParameterStatus — skip
            continue;
        } else if (mtype == 'Z') {
            g_pg_connected = 1;
            pg_log("ReadyForQuery — connected!");
            return 0;
        }
    }
}

int32_t __pg_close(void) {
    if (g_pg_fd >= 0) {
        char msg[5] = {'X', 0, 0, 0, 4};
        pg_write_i32(msg+1, 4);
        pg_send_raw(msg, 5);
        close(g_pg_fd);
        g_pg_fd = -1;
    }
    g_pg_connected = 0;
    pg_log("connection closed");
    return 0;
}

int32_t __pg_is_connected(void) {
    return g_pg_connected;
}

char* __pg_last_error(void) {
    return strdup(g_pg_error);
}

// ============================================================
// Query
// ============================================================

int32_t __pg_query(const char* sql) {
    if (!g_pg_connected || g_pg_fd < 0) {
        snprintf(g_pg_error, sizeof(g_pg_error), "Not connected");
        return -1;
    }

    pg_free_results();
    g_pg_error[0] = '\0';

    pg_log("query: %s", sql);

    // Send 'Q' message
    int sql_len = strlen(sql);
    int msg_len = 1 + 4 + sql_len + 1;
    char* msg = malloc(msg_len);
    msg[0] = 'Q';
    pg_write_i32(msg+1, 4 + sql_len + 1);
    memcpy(msg+5, sql, sql_len+1);
    if (pg_send_raw(msg, msg_len) < 0) {
        free(msg);
        snprintf(g_pg_error, sizeof(g_pg_error), "Failed to send query");
        return -1;
    }
    free(msg);

    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    int rows_affected = 0;

    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) {
            snprintf(g_pg_error, sizeof(g_pg_error), "Failed to read response");
            return -1;
        }

        switch (mtype) {
            case 'T': { // RowDescription
                int16_t nc = pg_read_i16(payload);
                g_pg_ncols = nc;
                pg_log("RowDescription: %d columns", nc);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < PG_MAX_COLS; i++) {
                    strncpy(g_pg_colnames[i], p, 127);
                    g_pg_colnames[i][127] = '\0';
                    pg_log("  col %d: %s", i, g_pg_colnames[i]);
                    p += strlen(p) + 1;
                    p += 18; // tableOID(4)+colAttr(2)+typeOID(4)+typeSize(2)+typeMod(4)+fmtCode(2)
                }
                break;
            }
            case 'D': { // DataRow
                int16_t nc = pg_read_i16(payload);
                int row_base = g_pg_nrows * g_pg_ncols;
                pg_ensure_cells(row_base + g_pg_ncols);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < g_pg_ncols; i++) {
                    int32_t clen = pg_read_i32(p); p += 4;
                    if (clen == -1) {
                        g_pg_cells[row_base + i] = strdup("");
                    } else {
                        g_pg_cells[row_base + i] = malloc(clen + 1);
                        memcpy(g_pg_cells[row_base + i], p, clen);
                        g_pg_cells[row_base + i][clen] = '\0';
                        p += clen;
                    }
                }
                g_pg_nrows++;
                break;
            }
            case 'C': { // CommandComplete
                char* tag = payload;
                pg_log("CommandComplete: %s", tag);
                if (strncmp(tag,"INSERT",6)==0 || strncmp(tag,"UPDATE",6)==0 || strncmp(tag,"DELETE",6)==0) {
                    char* sp = strrchr(tag, ' ');
                    if (sp) rows_affected = atoi(sp+1);
                } else if (strncmp(tag,"SELECT",6)==0) {
                    rows_affected = g_pg_nrows;
                } else if (strncmp(tag,"CREATE",6)==0 || strncmp(tag,"DROP",4)==0 || strncmp(tag,"ALTER",5)==0) {
                    rows_affected = 0;
                }
                break;
            }
            case 'E': { // Error
                char* p = payload;
                while (p < payload+plen && *p) {
                    char f = *p++;
                    if (f=='\0') break;
                    if (f=='M') { snprintf(g_pg_error, sizeof(g_pg_error), "%s", p); break; }
                    p += strlen(p)+1;
                }
                pg_log("error: %s", g_pg_error);
                break;
            }
            case 'N': break; // Notice
            case 'Z': { // ReadyForQuery
                pg_log("ReadyForQuery rows=%d", g_pg_nrows);
                if (g_pg_error[0]) return -1;
                return rows_affected;
            }
            default: break;
        }
    }
}

int32_t __pg_execute(const char* sql) { return __pg_query(sql); }

// ============================================================
// Parameterized Query — PG Extended Query Protocol
// Uses Parse → Bind → Describe → Execute → Sync
// Parameters are sent as separate text values, never inlined.
// ============================================================

int32_t __pg_query_params(const char* sql, const char** params, int nparams) {
    if (!g_pg_connected || g_pg_fd < 0) {
        snprintf(g_pg_error, sizeof(g_pg_error), "Not connected");
        return -1;
    }

    pg_free_results();
    g_pg_error[0] = '\0';

    pg_log("query_params: %s  (nparams=%d)", sql, nparams);
    for (int i = 0; i < nparams; i++) {
        pg_log("  param[%d] = '%s'", i, params[i] ? params[i] : "NULL");
    }

    // We'll build all messages into one buffer for a single send.
    // This avoids multiple round-trips.
    char buf[65536];
    int pos = 0;

    // ---- 1. Parse message ('P') ----
    // Format: 'P' + int32(len) + cstring(stmt_name) + cstring(query) + int16(num_param_types) + [int32(type_oid)...]
    {
        int sql_len = strlen(sql);
        // stmt_name = "" (unnamed), query = sql, 0 param types (let server infer)
        int body_len = 4 + 1 + (sql_len + 1) + 2;  // len + "" + query + int16(0)
        buf[pos++] = 'P';
        pg_write_i32(buf + pos, body_len); pos += 4;
        buf[pos++] = '\0';  // unnamed statement
        memcpy(buf + pos, sql, sql_len + 1); pos += sql_len + 1;
        buf[pos++] = 0; buf[pos++] = 0;  // int16(0) = no type OIDs
    }

    // ---- 2. Bind message ('B') ----
    // Format: 'B' + int32(len) + cstring(portal) + cstring(stmt) + int16(num_format_codes) + [int16(fc)...]
    //         + int16(num_params) + [int32(param_len) + bytes(param_val)]...
    //         + int16(num_result_format_codes) + [int16(rfc)...]
    {
        // Calculate body size
        int body = 4;  // len field
        body += 1;     // portal = ""
        body += 1;     // stmt = ""
        body += 2;     // int16(1) = one format code
        body += 2;     // int16(0) = text format
        body += 2;     // int16(nparams)

        for (int i = 0; i < nparams; i++) {
            body += 4;  // int32(param_len)
            if (params[i]) {
                body += strlen(params[i]);
            }
        }
        body += 2;  // int16(1) = one result format code
        body += 2;  // int16(0) = text format

        buf[pos++] = 'B';
        pg_write_i32(buf + pos, body); pos += 4;
        buf[pos++] = '\0';  // portal = ""
        buf[pos++] = '\0';  // stmt = ""

        // Format codes: 1 code, 0 = text (applies to all params)
        buf[pos++] = 0; buf[pos++] = 1;  // int16(1)
        buf[pos++] = 0; buf[pos++] = 0;  // int16(0) = text

        // Parameters
        buf[pos++] = (nparams >> 8) & 0xFF;
        buf[pos++] = nparams & 0xFF;

        for (int i = 0; i < nparams; i++) {
            if (params[i] == NULL) {
                // NULL parameter: length = -1
                pg_write_i32(buf + pos, -1); pos += 4;
            } else {
                int plen = strlen(params[i]);
                pg_write_i32(buf + pos, plen); pos += 4;
                memcpy(buf + pos, params[i], plen); pos += plen;
            }
        }

        // Result format: 1 code, 0 = text
        buf[pos++] = 0; buf[pos++] = 1;  // int16(1)
        buf[pos++] = 0; buf[pos++] = 0;  // int16(0) = text
    }

    // ---- 3. Describe message ('D') ----
    // Describe the portal to get RowDescription
    {
        buf[pos++] = 'D';
        pg_write_i32(buf + pos, 4 + 1 + 1); pos += 4;  // len = 6
        buf[pos++] = 'P';    // describe portal (not statement)
        buf[pos++] = '\0';   // unnamed portal
    }

    // ---- 4. Execute message ('E') ----
    {
        buf[pos++] = 'E';
        pg_write_i32(buf + pos, 4 + 1 + 4); pos += 4;  // len = 9
        buf[pos++] = '\0';                                // unnamed portal
        pg_write_i32(buf + pos, 0); pos += 4;            // max rows = 0 (all)
    }

    // ---- 5. Sync message ('S') ----
    {
        buf[pos++] = 'S';
        pg_write_i32(buf + pos, 4); pos += 4;
    }

    // Send all messages in one write
    if (pg_send_raw(buf, pos) < 0) {
        snprintf(g_pg_error, sizeof(g_pg_error), "Failed to send extended query");
        return -1;
    }

    // ---- Read responses ----
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    int rows_affected = 0;

    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) {
            snprintf(g_pg_error, sizeof(g_pg_error), "Failed to read extended query response");
            return -1;
        }

        switch (mtype) {
            case '1': // ParseComplete
                pg_log("ParseComplete");
                break;
            case '2': // BindComplete
                pg_log("BindComplete");
                break;
            case 'n': // NoData (for queries that don't return rows)
                pg_log("NoData");
                break;
            case 'T': { // RowDescription
                int16_t nc = pg_read_i16(payload);
                g_pg_ncols = nc;
                pg_log("RowDescription: %d columns", nc);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < PG_MAX_COLS; i++) {
                    strncpy(g_pg_colnames[i], p, 127);
                    g_pg_colnames[i][127] = '\0';
                    pg_log("  col %d: %s", i, g_pg_colnames[i]);
                    p += strlen(p) + 1;
                    p += 18; // tableOID(4)+colAttr(2)+typeOID(4)+typeSize(2)+typeMod(4)+fmtCode(2)
                }
                break;
            }
            case 'D': { // DataRow
                int16_t nc = pg_read_i16(payload);
                int row_base = g_pg_nrows * g_pg_ncols;
                pg_ensure_cells(row_base + g_pg_ncols);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < g_pg_ncols; i++) {
                    int32_t clen = pg_read_i32(p); p += 4;
                    if (clen == -1) {
                        g_pg_cells[row_base + i] = strdup("");
                    } else {
                        g_pg_cells[row_base + i] = malloc(clen + 1);
                        memcpy(g_pg_cells[row_base + i], p, clen);
                        g_pg_cells[row_base + i][clen] = '\0';
                        p += clen;
                    }
                }
                g_pg_nrows++;
                break;
            }
            case 'C': { // CommandComplete
                char* tag = payload;
                pg_log("CommandComplete: %s", tag);
                if (strncmp(tag,"INSERT",6)==0 || strncmp(tag,"UPDATE",6)==0 || strncmp(tag,"DELETE",6)==0) {
                    char* sp = strrchr(tag, ' ');
                    if (sp) rows_affected = atoi(sp+1);
                } else if (strncmp(tag,"SELECT",6)==0) {
                    rows_affected = g_pg_nrows;
                } else if (strncmp(tag,"CREATE",6)==0 || strncmp(tag,"DROP",4)==0 || strncmp(tag,"ALTER",5)==0) {
                    rows_affected = 0;
                }
                break;
            }
            case 'E': { // Error
                char* p = payload;
                while (p < payload + plen && *p) {
                    char f = *p++;
                    if (f == '\0') break;
                    if (f == 'M') { snprintf(g_pg_error, sizeof(g_pg_error), "%s", p); break; }
                    p += strlen(p) + 1;
                }
                pg_log("error: %s", g_pg_error);
                break;
            }
            case 'N': break; // Notice
            case 'Z': { // ReadyForQuery
                pg_log("ReadyForQuery rows=%d", g_pg_nrows);
                if (g_pg_error[0]) return -1;
                return rows_affected;
            }
            default: break;
        }
    }
}

// ============================================================
// Result Access
// ============================================================

int32_t __pg_row_count(void) { return g_pg_nrows; }
int32_t __pg_col_count(void) { return g_pg_ncols; }

char* __pg_col_name(int32_t idx) {
    if (idx < 0 || idx >= g_pg_ncols) return strdup("");
    return strdup(g_pg_colnames[idx]);
}

char* __pg_get_value(int32_t row, int32_t col) {
    if (row < 0 || row >= g_pg_nrows || col < 0 || col >= g_pg_ncols) return strdup("");
    if (!g_pg_cells) return strdup("");
    char* v = g_pg_cells[row * g_pg_ncols + col];
    return strdup(v ? v : "");
}

char* __pg_get_field(int32_t row, const char* name) {
    if (!name) return strdup("");
    for (int i = 0; i < g_pg_ncols; i++) {
        if (strcmp(g_pg_colnames[i], name) == 0)
            return __pg_get_value(row, i);
    }
    return strdup("");
}

// ============================================================
// Debug Helpers
// ============================================================

// Print full result set to stderr
int32_t __pg_dump_results(void) {
    fprintf(stderr, "=== PG Result: %d rows x %d cols ===\n", g_pg_nrows, g_pg_ncols);
    // Header
    for (int c = 0; c < g_pg_ncols; c++) {
        if (c > 0) fprintf(stderr, " | ");
        fprintf(stderr, "%-15s", g_pg_colnames[c]);
    }
    fprintf(stderr, "\n");
    for (int c = 0; c < g_pg_ncols; c++) {
        if (c > 0) fprintf(stderr, "-+-");
        fprintf(stderr, "---------------");
    }
    fprintf(stderr, "\n");
    // Rows
    for (int r = 0; r < g_pg_nrows; r++) {
        for (int c = 0; c < g_pg_ncols; c++) {
            if (c > 0) fprintf(stderr, " | ");
            char* v = g_pg_cells ? g_pg_cells[r * g_pg_ncols + c] : NULL;
            fprintf(stderr, "%-15s", v ? v : "(null)");
        }
        fprintf(stderr, "\n");
    }
    fprintf(stderr, "===\n");
    return 0;
}

// Get connection info as string
char* __pg_connection_info(void) {
    char buf[512];
    snprintf(buf, sizeof(buf), "fd=%d connected=%d user=%s error=%s",
        g_pg_fd, g_pg_connected, g_pg_user, g_pg_error);
    return strdup(buf);
}
