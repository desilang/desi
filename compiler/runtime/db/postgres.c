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

// Optional TLS/SSL support (compile-time detection)
#ifdef __has_include
  #if __has_include(<openssl/ssl.h>)
    #define PG_HAS_SSL 1
    #include <openssl/ssl.h>
    #include <openssl/err.h>
  #endif
#endif
#ifndef PG_HAS_SSL
  #define PG_HAS_SSL 0
#endif

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
// Connection State — struct-based for connection pooling
// ============================================================

#define PG_MAX_COLS 64
#define PG_MAX_ROWS 4096
#define PG_BUF_SIZE 65536

// PGConn holds all state for a single PostgreSQL connection.
// For connection pooling, each pool slot gets its own PGConn*.
typedef struct {
    // Connection
    int fd;
    int connected;
    char error[512];
    char user[128];
    char password[256];

    // TLS/SSL
#if PG_HAS_SSL
    SSL_CTX* ssl_ctx;
    SSL* ssl;
#endif
    int use_ssl;  // 1 if SSL is active

    // Result set
    int ncols;
    char colnames[PG_MAX_COLS][128];
    int nrows;
    char** cells;   // flat array: [row * ncols + col]
    int cells_cap;
} PGConn;

// The active connection — backward compatible with existing code.
// Connection pooling will swap this pointer.
static PGConn* g_pg = NULL;
static int g_pg_debug = 0;

// Ensure g_pg is allocated (lazy init)
static void pg_ensure_conn(void) {
    if (!g_pg) {
        g_pg = (PGConn*)calloc(1, sizeof(PGConn));
        g_pg->fd = -1;
    }
}

// Pool accessors — used by db_pool.c to swap the active connection
void* __pg_get_conn_ptr(void) { return (void*)g_pg; }
void  __pg_set_conn_ptr(void* conn) { g_pg = (PGConn*)conn; }

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
    if (g_pg->fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n;
#if PG_HAS_SSL
        if (g_pg->use_ssl && g_pg->ssl)
            n = SSL_write(g_pg->ssl, data + total, len - total);
        else
#endif
            n = (int)write(g_pg->fd, data + total, len - total);
        if (n <= 0) return -1;
        total += n;
    }
    return 0;
}

static int pg_recv_raw(char* buf, int len) {
    if (g_pg->fd < 0) return -1;
    int total = 0;
    while (total < len) {
        int n;
#if PG_HAS_SSL
        if (g_pg->use_ssl && g_pg->ssl)
            n = SSL_read(g_pg->ssl, buf + total, len - total);
        else
#endif
            n = (int)read(g_pg->fd, buf + total, len - total);
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
// SHA-256 (inline, for SCRAM-SHA-256)
// ============================================================

#define PG_ROTR32(x,n) (((x)>>(n))|((x)<<(32-(n))))
#define PG_CH(x,y,z) (((x)&(y))^((~(x))&(z)))
#define PG_MAJ(x,y,z) (((x)&(y))^((x)&(z))^((y)&(z)))
#define PG_SIG0(x) (PG_ROTR32(x,2)^PG_ROTR32(x,13)^PG_ROTR32(x,22))
#define PG_SIG1(x) (PG_ROTR32(x,6)^PG_ROTR32(x,11)^PG_ROTR32(x,25))
#define PG_SG0(x) (PG_ROTR32(x,7)^PG_ROTR32(x,18)^((x)>>3))
#define PG_SG1(x) (PG_ROTR32(x,17)^PG_ROTR32(x,19)^((x)>>10))

static const uint32_t pg_sha256_K[64] = {
    0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
    0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
    0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
    0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
    0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
    0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
    0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
    0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2
};

static void pg_sha256_block(uint32_t st[8], const unsigned char blk[64]) {
    uint32_t W[64], a,b,c,d,e,f,g,h;
    for (int i=0;i<16;i++)
        W[i]=((uint32_t)blk[i*4]<<24)|((uint32_t)blk[i*4+1]<<16)|((uint32_t)blk[i*4+2]<<8)|(uint32_t)blk[i*4+3];
    for (int i=16;i<64;i++)
        W[i]=PG_SG1(W[i-2])+W[i-7]+PG_SG0(W[i-15])+W[i-16];
    a=st[0];b=st[1];c=st[2];d=st[3];e=st[4];f=st[5];g=st[6];h=st[7];
    for (int i=0;i<64;i++) {
        uint32_t t1=h+PG_SIG1(e)+PG_CH(e,f,g)+pg_sha256_K[i]+W[i];
        uint32_t t2=PG_SIG0(a)+PG_MAJ(a,b,c);
        h=g;g=f;f=e;e=d+t1;d=c;c=b;b=a;a=t1+t2;
    }
    st[0]+=a;st[1]+=b;st[2]+=c;st[3]+=d;st[4]+=e;st[5]+=f;st[6]+=g;st[7]+=h;
}

// SHA-256: produces 32-byte digest
static void pg_sha256(const unsigned char* data, size_t len, unsigned char out[32]) {
    uint32_t st[8]={0x6a09e667,0xbb67ae85,0x3c6ef372,0xa54ff53a,0x510e527f,0x9b05688c,0x1f83d9ab,0x5be0cd19};
    unsigned char buf[64];
    size_t i;
    for (i=0;i+64<=len;i+=64) pg_sha256_block(st,(const unsigned char*)(data+i));
    size_t rem=len-i;
    memset(buf,0,64);
    memcpy(buf,data+i,rem);
    buf[rem]=0x80;
    if (rem>=56) { pg_sha256_block(st,buf); memset(buf,0,64); }
    uint64_t bits=len*8;
    for (int j=0;j<8;j++) buf[63-j]=(bits>>(j*8))&0xFF;
    pg_sha256_block(st,buf);
    for (int j=0;j<8;j++) {
        out[j*4]=(st[j]>>24)&0xFF; out[j*4+1]=(st[j]>>16)&0xFF;
        out[j*4+2]=(st[j]>>8)&0xFF; out[j*4+3]=st[j]&0xFF;
    }
}

// HMAC-SHA-256
static void pg_hmac_sha256(const unsigned char* key, size_t klen,
                           const unsigned char* data, size_t dlen,
                           unsigned char out[32]) {
    unsigned char kpad[64];
    memset(kpad, 0, 64);
    if (klen > 64) {
        pg_sha256(key, klen, kpad); // hash long keys
    } else {
        memcpy(kpad, key, klen);
    }

    unsigned char ipad[64], opad[64];
    for (int i=0;i<64;i++) { ipad[i]=kpad[i]^0x36; opad[i]=kpad[i]^0x5c; }

    // inner hash = SHA256(ipad || data)
    unsigned char* inner = malloc(64+dlen);
    memcpy(inner, ipad, 64);
    memcpy(inner+64, data, dlen);
    unsigned char ihash[32];
    pg_sha256(inner, 64+dlen, ihash);
    free(inner);

    // outer hash = SHA256(opad || inner_hash)
    unsigned char outer[96];
    memcpy(outer, opad, 64);
    memcpy(outer+64, ihash, 32);
    pg_sha256(outer, 96, out);
}

// PBKDF2-HMAC-SHA-256 (RFC 2898) — produces 32 bytes
static void pg_pbkdf2_sha256(const char* password, size_t plen,
                             const unsigned char* salt, size_t slen,
                             int iterations, unsigned char out[32]) {
    // U1 = HMAC(password, salt || INT(1))
    unsigned char* salti = malloc(slen + 4);
    memcpy(salti, salt, slen);
    salti[slen]=0; salti[slen+1]=0; salti[slen+2]=0; salti[slen+3]=1; // BE int32(1)

    unsigned char U[32], T[32];
    pg_hmac_sha256((const unsigned char*)password, plen, salti, slen+4, U);
    free(salti);
    memcpy(T, U, 32);

    for (int i=1; i<iterations; i++) {
        unsigned char Unew[32];
        pg_hmac_sha256((const unsigned char*)password, plen, U, 32, Unew);
        memcpy(U, Unew, 32);
        for (int j=0;j<32;j++) T[j]^=U[j];
    }
    memcpy(out, T, 32);
}

// ============================================================
// Base64 encode/decode (for SCRAM)
// ============================================================

static const char pg_b64_chars[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

static int pg_b64_encode(const unsigned char* in, size_t len, char* out) {
    int o=0;
    for (size_t i=0; i<len; i+=3) {
        uint32_t val = (uint32_t)in[i]<<16;
        if (i+1<len) val |= (uint32_t)in[i+1]<<8;
        if (i+2<len) val |= (uint32_t)in[i+2];
        out[o++]=pg_b64_chars[(val>>18)&63];
        out[o++]=pg_b64_chars[(val>>12)&63];
        out[o++]=(i+1<len) ? pg_b64_chars[(val>>6)&63] : '=';
        out[o++]=(i+2<len) ? pg_b64_chars[val&63] : '=';
    }
    out[o]='\0';
    return o;
}

static int pg_b64_decode(const char* in, unsigned char* out) {
    static int tab_init=0;
    static unsigned char tab[256];
    if (!tab_init) {
        memset(tab,64,256);
        for (int i=0;i<64;i++) tab[(unsigned char)pg_b64_chars[i]]=i;
        tab_init=1;
    }
    int o=0, len=strlen(in);
    for (int i=0;i<len;i+=4) {
        uint32_t v=0; int pad=0;
        for (int j=0;j<4;j++) {
            if (i+j<len && in[i+j]!='=') v=(v<<6)|tab[(unsigned char)in[i+j]];
            else { v<<=6; pad++; }
        }
        out[o++]=(v>>16)&0xFF;
        if (pad<2) out[o++]=(v>>8)&0xFF;
        if (pad<1) out[o++]=v&0xFF;
    }
    return o;
}

// ============================================================
// SCRAM-SHA-256 Authentication (RFC 5802 + PG wire)
// ============================================================

// XOR two 32-byte buffers
static void pg_xor32(unsigned char* out, const unsigned char* a, const unsigned char* b) {
    for (int i=0;i<32;i++) out[i]=a[i]^b[i];
}

// Perform the full SCRAM-SHA-256 exchange.
// Called when auth type == 10 (SASL).
// Returns 0 on success, -1 on failure.
static int pg_scram_auth(const char* user, const char* password) {
    // ---- Step 1: Send SASLInitialResponse ----
    // Generate client nonce (24 random bytes, base64-encoded)
    unsigned char nonce_raw[24];
    // Simple nonce — use /dev/urandom if available, else fallback
    FILE* urand = fopen("/dev/urandom", "rb");
    if (urand) {
        fread(nonce_raw, 1, 24, urand);
        fclose(urand);
    } else {
        // Fallback: use time + pid (less secure but functional)
        for (int i=0;i<24;i++) nonce_raw[i]=(unsigned char)(rand()^(i*17));
    }
    char client_nonce[48];
    pg_b64_encode(nonce_raw, 24, client_nonce);

    // client-first-message-bare = "n=,r=<nonce>" (no username per PG spec)
    char cfm_bare[256];
    snprintf(cfm_bare, sizeof(cfm_bare), "n=,r=%s", client_nonce);

    // client-first-message = "n,," + cfm_bare (GS2 header for no channel binding)
    char cfm[300];
    snprintf(cfm, sizeof(cfm), "n,,%s", cfm_bare);

    // Build SASLInitialResponse: 'p' + len + "SCRAM-SHA-256\0" + int32(cfm_len) + cfm
    {
        const char* mech = "SCRAM-SHA-256";
        int mech_len = strlen(mech);
        int cfm_len = strlen(cfm);
        int body_len = 4 + (mech_len+1) + 4 + cfm_len;
        char msg[512]; int mp = 0;
        msg[mp++] = 'p';
        pg_write_i32(msg+mp, body_len); mp += 4;
        memcpy(msg+mp, mech, mech_len+1); mp += mech_len+1;
        pg_write_i32(msg+mp, cfm_len); mp += 4;
        memcpy(msg+mp, cfm, cfm_len); mp += cfm_len;
        if (pg_send_raw(msg, mp) < 0) return -1;
        pg_log("SCRAM: sent SASLInitialResponse");
    }

    // ---- Step 2: Read AuthenticationSASLContinue (type 11) ----
    char mtype; char payload[PG_BUF_SIZE]; int plen;
    if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;
    if (mtype != 'R') return -1;
    int32_t atype2 = pg_read_i32(payload);
    if (atype2 != 11) {
        snprintf(g_pg->error, sizeof(g_pg->error), "Expected SASLContinue(11), got %d", atype2);
        return -1;
    }

    // server-first-message is payload+4
    char sfm[2048];
    int sfm_len = plen - 4;
    memcpy(sfm, payload+4, sfm_len);
    sfm[sfm_len] = '\0';
    pg_log("SCRAM: server-first-message = %s", sfm);

    // Parse server-first-message: r=<nonce>,s=<salt_b64>,i=<iterations>
    char* server_nonce = NULL;
    char* salt_b64 = NULL;
    int iterations = 4096;
    {
        char* p = sfm;
        while (*p) {
            if (p[0]=='r' && p[1]=='=') { server_nonce = p+2; }
            else if (p[0]=='s' && p[1]=='=') { salt_b64 = p+2; }
            else if (p[0]=='i' && p[1]=='=') { iterations = atoi(p+2); }
            // Advance to next ','
            while (*p && *p!=',') p++;
            if (*p==',') { *p='\0'; p++; }
        }
    }
    if (!server_nonce || !salt_b64) {
        snprintf(g_pg->error, sizeof(g_pg->error), "SCRAM: invalid server-first-message");
        return -1;
    }

    // Decode salt
    unsigned char salt[256];
    int salt_len = pg_b64_decode(salt_b64, salt);

    // Derive SaltedPassword = PBKDF2(password, salt, iterations)
    unsigned char salted_password[32];
    pg_pbkdf2_sha256(password, strlen(password), salt, salt_len, iterations, salted_password);

    // ClientKey = HMAC(SaltedPassword, "Client Key")
    unsigned char client_key[32];
    pg_hmac_sha256(salted_password, 32, (const unsigned char*)"Client Key", 10, client_key);

    // StoredKey = SHA256(ClientKey)
    unsigned char stored_key[32];
    pg_sha256(client_key, 32, stored_key);

    // ServerKey = HMAC(SaltedPassword, "Server Key")
    unsigned char server_key[32];
    pg_hmac_sha256(salted_password, 32, (const unsigned char*)"Server Key", 10, server_key);

    // ---- Step 3: Send SASLResponse (client-final-message) ----
    // channel-binding = "c=biws" (base64 of "n,,")
    // client-final-message-without-proof = "c=biws,r=<server_nonce>"
    char cfm_wo_proof[512];
    snprintf(cfm_wo_proof, sizeof(cfm_wo_proof), "c=biws,r=%s", server_nonce);

    // AuthMessage = client-first-message-bare + "," + server-first-message + "," + cfm_wo_proof
    // Reconstruct server-first-message (we null-terminated commas, need original)
    // Re-read sfm from payload
    memcpy(sfm, payload+4, sfm_len);
    sfm[sfm_len] = '\0';

    char auth_message[4096];
    snprintf(auth_message, sizeof(auth_message), "%s,%s,%s", cfm_bare, sfm, cfm_wo_proof);

    // ClientSignature = HMAC(StoredKey, AuthMessage)
    unsigned char client_sig[32];
    pg_hmac_sha256(stored_key, 32, (const unsigned char*)auth_message, strlen(auth_message), client_sig);

    // ClientProof = ClientKey XOR ClientSignature
    unsigned char client_proof[32];
    pg_xor32(client_proof, client_key, client_sig);

    // Encode proof as base64
    char proof_b64[64];
    pg_b64_encode(client_proof, 32, proof_b64);

    // client-final-message = cfm_wo_proof + ",p=" + proof_b64
    char cfm_final[1024];
    snprintf(cfm_final, sizeof(cfm_final), "%s,p=%s", cfm_wo_proof, proof_b64);

    // ServerSignature = HMAC(ServerKey, AuthMessage) — for verification
    unsigned char expected_server_sig[32];
    pg_hmac_sha256(server_key, 32, (const unsigned char*)auth_message, strlen(auth_message), expected_server_sig);

    // Send SASLResponse: 'p' + len + cfm_final
    {
        int cfm_final_len = strlen(cfm_final);
        int body_len = 4 + cfm_final_len;
        char msg[1536]; int mp = 0;
        msg[mp++] = 'p';
        pg_write_i32(msg+mp, body_len); mp += 4;
        memcpy(msg+mp, cfm_final, cfm_final_len); mp += cfm_final_len;
        if (pg_send_raw(msg, mp) < 0) return -1;
        pg_log("SCRAM: sent SASLResponse (client-final-message)");
    }

    // ---- Step 4: Read AuthenticationSASLFinal (type 12) ----
    if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;
    if (mtype != 'R') {
        if (mtype == 'E') {
            char* p = payload;
            while (p < payload+plen && *p) {
                char f = *p++;
                if (f=='\0') break;
                if (f=='M') { snprintf(g_pg->error, sizeof(g_pg->error), "%s", p); break; }
                p += strlen(p)+1;
            }
        }
        return -1;
    }
    int32_t atype3 = pg_read_i32(payload);
    if (atype3 == 12) {
        // Verify server signature
        char* sv_str = payload + 4;
        if (sv_str[0]=='v' && sv_str[1]=='=') {
            unsigned char got_sig[32];
            pg_b64_decode(sv_str+2, got_sig);
            if (memcmp(got_sig, expected_server_sig, 32) != 0) {
                snprintf(g_pg->error, sizeof(g_pg->error), "SCRAM: server signature mismatch");
                return -1;
            }
            pg_log("SCRAM: server signature verified");
        }
    } else if (atype3 != 0) {
        snprintf(g_pg->error, sizeof(g_pg->error), "SCRAM: unexpected auth type %d", atype3);
        return -1;
    }

    // Read AuthenticationOk (type 0) if we got SASLFinal
    if (atype3 == 12) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;
        if (mtype == 'R') {
            int32_t ok = pg_read_i32(payload);
            if (ok != 0) {
                snprintf(g_pg->error, sizeof(g_pg->error), "SCRAM: auth not OK after final, got %d", ok);
                return -1;
            }
        }
    }
    pg_log("SCRAM: authentication successful");
    return 0;
}

// ============================================================
// Result set management
// ============================================================

static void pg_free_results(void) {
    if (g_pg->cells) {
        for (int i = 0; i < g_pg->nrows * g_pg->ncols; i++) {
            if (g_pg->cells[i]) free(g_pg->cells[i]);
        }
        free(g_pg->cells);
        g_pg->cells = NULL;
    }
    g_pg->nrows = 0;
    g_pg->ncols = 0;
    g_pg->cells_cap = 0;
}

static void pg_ensure_cells(int needed) {
    if (needed <= g_pg->cells_cap) return;
    int newcap = needed * 2;
    g_pg->cells = realloc(g_pg->cells, newcap * sizeof(char*));
    for (int i = g_pg->cells_cap; i < newcap; i++) g_pg->cells[i] = NULL;
    g_pg->cells_cap = newcap;
}

// ============================================================
// Connect
// ============================================================

// Forward declarations from db_query.c and db_orm.c
extern int32_t __db_set_dialect(int32_t dialect);
extern int32_t __orm_set_dialect(int32_t dialect);

int32_t __pg_connect(const char* host, int32_t port, const char* dbname,
                     const char* user, const char* password) {
    pg_ensure_conn();
    // Set dialect to Postgres (0) — done here in C because
    // Desi module wrappers must only make a single extern call.
    __db_set_dialect(0);
    __orm_set_dialect(0);
    if (!host) host = "127.0.0.1";
    if (port <= 0) port = 5432;
    if (!dbname) dbname = "postgres";
    if (!user) user = "postgres";
    if (!password) password = "";

    strncpy(g_pg->user, user, sizeof(g_pg->user)-1);
    strncpy(g_pg->password, password, sizeof(g_pg->password)-1);

    // Reset
    pg_free_results();
    g_pg->fd = -1;
    g_pg->connected = 0;
    g_pg->error[0] = '\0';

    pg_log("connecting to %s:%d db=%s user=%s", host, port, dbname, user);

    // TCP connect
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);

    if (inet_pton(AF_INET, host, &addr.sin_addr) <= 0) {
        struct hostent* he = gethostbyname(host);
        if (!he) {
            snprintf(g_pg->error, sizeof(g_pg->error), "Cannot resolve host: %s", host);
            return -1;
        }
        memcpy(&addr.sin_addr, he->h_addr_list[0], he->h_length);
    }

    g_pg->fd = socket(AF_INET, SOCK_STREAM, 0);
    if (g_pg->fd < 0) {
        snprintf(g_pg->error, sizeof(g_pg->error), "Socket failed: %s", strerror(errno));
        return -1;
    }

    if (connect(g_pg->fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        snprintf(g_pg->error, sizeof(g_pg->error), "Connect failed: %s", strerror(errno));
        close(g_pg->fd); g_pg->fd = -1;
        return -1;
    }

    pg_log("TCP connected fd=%d", g_pg->fd);

    // ---- SSL/TLS Negotiation ----
    // Try SSL upgrade before sending StartupMessage.
    // Send SSLRequest (int32 len=8, int32 magic=80877103).
    // Server replies: 'S' = SSL ok, 'N' = no SSL.
    g_pg->use_ssl = 0;
#if PG_HAS_SSL
    {
        char ssl_req[8];
        pg_write_i32(ssl_req, 8);         // message length
        pg_write_i32(ssl_req+4, 80877103); // SSL request code
        // Use raw write since SSL isn't active yet
        int wr = (int)write(g_pg->fd, ssl_req, 8);
        if (wr == 8) {
            char ssl_resp;
            int rd = (int)read(g_pg->fd, &ssl_resp, 1);
            if (rd == 1 && ssl_resp == 'S') {
                // Server accepted SSL — perform TLS handshake
                SSL_library_init();
                SSL_load_error_strings();
                g_pg->ssl_ctx = SSL_CTX_new(TLS_client_method());
                if (g_pg->ssl_ctx) {
                    g_pg->ssl = SSL_new(g_pg->ssl_ctx);
                    SSL_set_fd(g_pg->ssl, g_pg->fd);
                    if (SSL_connect(g_pg->ssl) == 1) {
                        g_pg->use_ssl = 1;
                        pg_log("SSL/TLS connection established (%s)", SSL_get_version(g_pg->ssl));
                    } else {
                        pg_log("SSL_connect failed, falling back to plaintext");
                        SSL_free(g_pg->ssl); g_pg->ssl = NULL;
                        SSL_CTX_free(g_pg->ssl_ctx); g_pg->ssl_ctx = NULL;
                        // Reconnect for plaintext (server closed the SSL attempt)
                        close(g_pg->fd);
                        g_pg->fd = socket(AF_INET, SOCK_STREAM, 0);
                        if (g_pg->fd < 0 || connect(g_pg->fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
                            snprintf(g_pg->error, sizeof(g_pg->error), "SSL fallback reconnect failed");
                            return -1;
                        }
                    }
                }
            } else {
                pg_log("Server declined SSL (response='%c'), continuing plaintext", ssl_resp);
            }
        }
    }
#endif

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
        snprintf(g_pg->error, sizeof(g_pg->error), "Failed to send startup");
        close(g_pg->fd); g_pg->fd = -1;
        return -1;
    }

    pg_log("StartupMessage sent (%d bytes)", pos);

    // Auth loop
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;

    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) {
            snprintf(g_pg->error, sizeof(g_pg->error), "Failed to read auth response");
            close(g_pg->fd); g_pg->fd = -1;
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
            } else if (atype == 10) {
                // SASL (SCRAM-SHA-256) — PG 14+ default
                pg_log("auth SASL/SCRAM-SHA-256 requested");
                if (pg_scram_auth(user, password) < 0) {
                    if (!g_pg->error[0])
                        snprintf(g_pg->error, sizeof(g_pg->error), "SCRAM-SHA-256 authentication failed");
                    close(g_pg->fd); g_pg->fd = -1;
                    return -1;
                }
                // SCRAM consumed its own auth messages; continue to ReadyForQuery
                continue;
            } else {
                snprintf(g_pg->error, sizeof(g_pg->error),
                    "Unsupported auth method: %d", atype);
                close(g_pg->fd); g_pg->fd = -1;
                return -1;
            }
        } else if (mtype == 'E') {
            char* p = payload;
            while (p < payload + plen && *p) {
                char f = *p++;
                if (f == '\0') break;
                if (f == 'M') { snprintf(g_pg->error, sizeof(g_pg->error), "%s", p); break; }
                p += strlen(p) + 1;
            }
            pg_log("auth error: %s", g_pg->error);
            close(g_pg->fd); g_pg->fd = -1;
            return -1;
        } else if (mtype == 'K' || mtype == 'S') {
            // BackendKeyData / ParameterStatus — skip
            continue;
        } else if (mtype == 'Z') {
            g_pg->connected = 1;
            pg_log("ReadyForQuery — connected!");
            return 0;
        }
    }
}

int32_t __pg_close(void) {
    if (!g_pg) return 0;
    if (g_pg->fd >= 0) {
        char msg[5] = {'X', 0, 0, 0, 4};
        pg_write_i32(msg+1, 4);
        pg_send_raw(msg, 5);
#if PG_HAS_SSL
        if (g_pg->ssl) { SSL_shutdown(g_pg->ssl); SSL_free(g_pg->ssl); g_pg->ssl = NULL; }
        if (g_pg->ssl_ctx) { SSL_CTX_free(g_pg->ssl_ctx); g_pg->ssl_ctx = NULL; }
#endif
        g_pg->use_ssl = 0;
        close(g_pg->fd);
        g_pg->fd = -1;
    }
    g_pg->connected = 0;
    pg_log("connection closed");
    return 0;
}

int32_t __pg_is_connected(void) {
    return g_pg ? g_pg->connected : 0;
}

char* __pg_last_error(void) {
    return g_pg ? strdup(g_pg->error) : strdup("");
}

// ============================================================
// Query
// ============================================================

int32_t __pg_query(const char* sql) {
    if (!g_pg || !g_pg->connected || g_pg->fd < 0) {
        if (g_pg) snprintf(g_pg->error, sizeof(g_pg->error), "Not connected");
        return -1;
    }

    pg_free_results();
    g_pg->error[0] = '\0';

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
        snprintf(g_pg->error, sizeof(g_pg->error), "Failed to send query");
        return -1;
    }
    free(msg);

    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    int rows_affected = 0;

    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) {
            snprintf(g_pg->error, sizeof(g_pg->error), "Failed to read response");
            return -1;
        }

        switch (mtype) {
            case 'T': { // RowDescription
                int16_t nc = pg_read_i16(payload);
                g_pg->ncols = nc;
                pg_log("RowDescription: %d columns", nc);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < PG_MAX_COLS; i++) {
                    strncpy(g_pg->colnames[i], p, 127);
                    g_pg->colnames[i][127] = '\0';
                    pg_log("  col %d: %s", i, g_pg->colnames[i]);
                    p += strlen(p) + 1;
                    p += 18; // tableOID(4)+colAttr(2)+typeOID(4)+typeSize(2)+typeMod(4)+fmtCode(2)
                }
                break;
            }
            case 'D': { // DataRow
                int16_t nc = pg_read_i16(payload);
                int row_base = g_pg->nrows * g_pg->ncols;
                pg_ensure_cells(row_base + g_pg->ncols);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < g_pg->ncols; i++) {
                    int32_t clen = pg_read_i32(p); p += 4;
                    if (clen == -1) {
                        g_pg->cells[row_base + i] = strdup("");
                    } else {
                        g_pg->cells[row_base + i] = malloc(clen + 1);
                        memcpy(g_pg->cells[row_base + i], p, clen);
                        g_pg->cells[row_base + i][clen] = '\0';
                        p += clen;
                    }
                }
                g_pg->nrows++;
                break;
            }
            case 'C': { // CommandComplete
                char* tag = payload;
                pg_log("CommandComplete: %s", tag);
                if (strncmp(tag,"INSERT",6)==0 || strncmp(tag,"UPDATE",6)==0 || strncmp(tag,"DELETE",6)==0) {
                    char* sp = strrchr(tag, ' ');
                    if (sp) rows_affected = atoi(sp+1);
                } else if (strncmp(tag,"SELECT",6)==0) {
                    rows_affected = g_pg->nrows;
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
                    if (f=='M') { snprintf(g_pg->error, sizeof(g_pg->error), "%s", p); break; }
                    p += strlen(p)+1;
                }
                pg_log("error: %s", g_pg->error);
                break;
            }
            case 'N': break; // Notice
            case 'Z': { // ReadyForQuery
                pg_log("ReadyForQuery rows=%d", g_pg->nrows);
                if (g_pg->error[0]) return -1;
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
    if (!g_pg || !g_pg->connected || g_pg->fd < 0) {
        if (g_pg) snprintf(g_pg->error, sizeof(g_pg->error), "Not connected");
        return -1;
    }

    pg_free_results();
    g_pg->error[0] = '\0';

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
        snprintf(g_pg->error, sizeof(g_pg->error), "Failed to send extended query");
        return -1;
    }

    // ---- Read responses ----
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    int rows_affected = 0;

    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) {
            snprintf(g_pg->error, sizeof(g_pg->error), "Failed to read extended query response");
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
                g_pg->ncols = nc;
                pg_log("RowDescription: %d columns", nc);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < PG_MAX_COLS; i++) {
                    strncpy(g_pg->colnames[i], p, 127);
                    g_pg->colnames[i][127] = '\0';
                    pg_log("  col %d: %s", i, g_pg->colnames[i]);
                    p += strlen(p) + 1;
                    p += 18; // tableOID(4)+colAttr(2)+typeOID(4)+typeSize(2)+typeMod(4)+fmtCode(2)
                }
                break;
            }
            case 'D': { // DataRow
                int16_t nc = pg_read_i16(payload);
                int row_base = g_pg->nrows * g_pg->ncols;
                pg_ensure_cells(row_base + g_pg->ncols);
                char* p = payload + 2;
                for (int i = 0; i < nc && i < g_pg->ncols; i++) {
                    int32_t clen = pg_read_i32(p); p += 4;
                    if (clen == -1) {
                        g_pg->cells[row_base + i] = strdup("");
                    } else {
                        g_pg->cells[row_base + i] = malloc(clen + 1);
                        memcpy(g_pg->cells[row_base + i], p, clen);
                        g_pg->cells[row_base + i][clen] = '\0';
                        p += clen;
                    }
                }
                g_pg->nrows++;
                break;
            }
            case 'C': { // CommandComplete
                char* tag = payload;
                pg_log("CommandComplete: %s", tag);
                if (strncmp(tag,"INSERT",6)==0 || strncmp(tag,"UPDATE",6)==0 || strncmp(tag,"DELETE",6)==0) {
                    char* sp = strrchr(tag, ' ');
                    if (sp) rows_affected = atoi(sp+1);
                } else if (strncmp(tag,"SELECT",6)==0) {
                    rows_affected = g_pg->nrows;
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
                    if (f == 'M') { snprintf(g_pg->error, sizeof(g_pg->error), "%s", p); break; }
                    p += strlen(p) + 1;
                }
                pg_log("error: %s", g_pg->error);
                break;
            }
            case 'N': break; // Notice
            case 'Z': { // ReadyForQuery
                pg_log("ReadyForQuery rows=%d", g_pg->nrows);
                if (g_pg->error[0]) return -1;
                return rows_affected;
            }
            default: break;
        }
    }
}

// ============================================================
// Result Access
// ============================================================

int32_t __pg_row_count(void) { return g_pg ? g_pg->nrows : 0; }
int32_t __pg_col_count(void) { return g_pg ? g_pg->ncols : 0; }

char* __pg_col_name(int32_t idx) {
    if (!g_pg || idx < 0 || idx >= g_pg->ncols) return strdup("");
    return strdup(g_pg->colnames[idx]);
}

char* __pg_get_value(int32_t row, int32_t col) {
    if (!g_pg || row < 0 || row >= g_pg->nrows || col < 0 || col >= g_pg->ncols) return strdup("");
    if (!g_pg->cells) return strdup("");
    char* v = g_pg->cells[row * g_pg->ncols + col];
    return strdup(v ? v : "");
}

char* __pg_get_field(int32_t row, const char* name) {
    if (!g_pg || !name) return strdup("");
    for (int i = 0; i < g_pg->ncols; i++) {
        if (strcmp(g_pg->colnames[i], name) == 0)
            return __pg_get_value(row, i);
    }
    return strdup("");
}

// ============================================================
// Debug Helpers
// ============================================================

// Print full result set to stderr
int32_t __pg_dump_results(void) {
    if (!g_pg) { fprintf(stderr, "=== PG: no connection ===\n"); return 0; }
    fprintf(stderr, "=== PG Result: %d rows x %d cols ===\n", g_pg->nrows, g_pg->ncols);
    // Header
    for (int c = 0; c < g_pg->ncols; c++) {
        if (c > 0) fprintf(stderr, " | ");
        fprintf(stderr, "%-15s", g_pg->colnames[c]);
    }
    fprintf(stderr, "\n");
    for (int c = 0; c < g_pg->ncols; c++) {
        if (c > 0) fprintf(stderr, "-+-");
        fprintf(stderr, "---------------");
    }
    fprintf(stderr, "\n");
    // Rows
    for (int r = 0; r < g_pg->nrows; r++) {
        for (int c = 0; c < g_pg->ncols; c++) {
            if (c > 0) fprintf(stderr, " | ");
            char* v = g_pg->cells ? g_pg->cells[r * g_pg->ncols + c] : NULL;
            fprintf(stderr, "%-15s", v ? v : "(null)");
        }
        fprintf(stderr, "\n");
    }
    fprintf(stderr, "===\n");
    return 0;
}

// Get connection info as string
char* __pg_connection_info(void) {
    if (!g_pg) return strdup("not initialized");
    char buf[512];
    snprintf(buf, sizeof(buf), "fd=%d connected=%d user=%s error=%s",
        g_pg->fd, g_pg->connected, g_pg->user, g_pg->error);
    return strdup(buf);
}

// ============================================================
// COPY Protocol — High-Speed Bulk Import
// ============================================================
//
// PG COPY protocol flow:
//   1. Send Query("COPY table FROM STDIN ...") → server responds with CopyInResponse ('G')
//   2. Send CopyData ('d') messages — one per row, tab-separated text
//   3. Send CopyDone ('c') to finalize
//   4. Server responds with CommandComplete ('C')
//
// This is ~10x faster than INSERT for bulk loads.

// Begin COPY IN mode: sends COPY command, waits for CopyInResponse
int32_t __pg_copy_begin(const char* table_name, const char* columns) {
    if (!g_pg || !g_pg->connected) return -1;
    if (!table_name) return -1;

    char sql[1024];
    if (columns && columns[0]) {
        snprintf(sql, sizeof(sql), "COPY %s (%s) FROM STDIN WITH (FORMAT text)", table_name, columns);
    } else {
        snprintf(sql, sizeof(sql), "COPY %s FROM STDIN WITH (FORMAT text)", table_name);
    }

    pg_log("COPY begin: %s", sql);

    // Send as a Query message: 'Q' + len + sql\0
    int sql_len = strlen(sql);
    int msg_len = 4 + sql_len + 1;
    char* msg = malloc(1 + msg_len);
    msg[0] = 'Q';
    pg_write_i32(msg + 1, msg_len);
    memcpy(msg + 5, sql, sql_len + 1);
    int rc = pg_send_raw(msg, 1 + msg_len);
    free(msg);
    if (rc < 0) return -1;

    // Read response — expect CopyInResponse ('G')
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;

    if (mtype == 'G') {
        // CopyInResponse — we're in COPY IN mode
        pg_log("COPY IN mode active");
        return 0;
    } else if (mtype == 'E') {
        // Error
        char* p = payload;
        while (p < payload + plen && *p) {
            char f = *p++;
            if (f == '\0') break;
            if (f == 'M') {
                snprintf(g_pg->error, sizeof(g_pg->error), "COPY error: %s", p);
                break;
            }
            p += strlen(p) + 1;
        }
        // Drain ReadyForQuery
        pg_read_msg(&mtype, payload, &plen);
        return -1;
    }

    snprintf(g_pg->error, sizeof(g_pg->error), "COPY: unexpected response '%c'", mtype);
    return -1;
}

// Send a row of tab-separated data during COPY IN
// row: tab-separated string (e.g., "1\tAlice\t30\n")
int32_t __pg_copy_row(const char* row) {
    if (!g_pg || !g_pg->connected || !row) return -1;

    int row_len = strlen(row);

    // CopyData message: 'd' + int32(len) + data
    int msg_len = 4 + row_len;
    char* msg = malloc(1 + msg_len);
    msg[0] = 'd';
    pg_write_i32(msg + 1, msg_len);
    memcpy(msg + 5, row, row_len);
    int rc = pg_send_raw(msg, 1 + msg_len);
    free(msg);
    return rc;
}

// End COPY IN mode — sends CopyDone, reads CommandComplete
int32_t __pg_copy_end(void) {
    if (!g_pg || !g_pg->connected) return -1;

    // CopyDone message: 'c' + int32(4)
    char done[5];
    done[0] = 'c';
    pg_write_i32(done + 1, 4);
    if (pg_send_raw(done, 5) < 0) return -1;

    pg_log("COPY done sent");

    // Read responses until ReadyForQuery
    int rows_copied = 0;
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;

    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;

        if (mtype == 'C') {
            // CommandComplete — e.g., "COPY 1000"
            payload[plen] = '\0';
            pg_log("COPY complete: %s", payload);
            // Parse row count from "COPY <N>"
            if (strncmp(payload, "COPY ", 5) == 0) {
                rows_copied = atoi(payload + 5);
            }
        } else if (mtype == 'Z') {
            // ReadyForQuery — done
            break;
        } else if (mtype == 'E') {
            // Error during COPY
            char* p = payload;
            while (p < payload + plen && *p) {
                char f = *p++;
                if (f == '\0') break;
                if (f == 'M') {
                    snprintf(g_pg->error, sizeof(g_pg->error), "COPY error: %s", p);
                    break;
                }
                p += strlen(p) + 1;
            }
            // Keep reading until ReadyForQuery
            continue;
        }
    }

    return rows_copied;
}

// ============================================================
// LISTEN / NOTIFY — Async Event Pub/Sub
// ============================================================
//
// PG LISTEN/NOTIFY protocol:
//   - LISTEN channel_name: subscribes to notifications
//   - NOTIFY channel_name, 'payload': sends notification to all listeners
//   - Notifications arrive as async messages (type 'A') anytime

// Subscribe to a notification channel
int32_t __pg_listen(const char* channel) {
    if (!g_pg || !g_pg->connected || !channel) return -1;

    char sql[256];
    snprintf(sql, sizeof(sql), "LISTEN %s", channel);

    // Send Query
    int sql_len = strlen(sql);
    int msg_len = 4 + sql_len + 1;
    char* msg = malloc(1 + msg_len);
    msg[0] = 'Q';
    pg_write_i32(msg + 1, msg_len);
    memcpy(msg + 5, sql, sql_len + 1);
    int rc = pg_send_raw(msg, 1 + msg_len);
    free(msg);
    if (rc < 0) return -1;

    // Read until ReadyForQuery
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;
        if (mtype == 'Z') break;
        if (mtype == 'E') {
            char* p = payload;
            while (p < payload + plen && *p) {
                char f = *p++;
                if (f == '\0') break;
                if (f == 'M') {
                    snprintf(g_pg->error, sizeof(g_pg->error), "LISTEN error: %s", p);
                    break;
                }
                p += strlen(p) + 1;
            }
            return -1;
        }
    }

    pg_log("LISTEN %s OK", channel);
    return 0;
}

// Unsubscribe from a notification channel
int32_t __pg_unlisten(const char* channel) {
    if (!g_pg || !g_pg->connected || !channel) return -1;

    char sql[256];
    snprintf(sql, sizeof(sql), "UNLISTEN %s", channel);

    int sql_len = strlen(sql);
    int msg_len = 4 + sql_len + 1;
    char* msg = malloc(1 + msg_len);
    msg[0] = 'Q';
    pg_write_i32(msg + 1, msg_len);
    memcpy(msg + 5, sql, sql_len + 1);
    int rc = pg_send_raw(msg, 1 + msg_len);
    free(msg);
    if (rc < 0) return -1;

    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;
        if (mtype == 'Z') break;
    }

    pg_log("UNLISTEN %s OK", channel);
    return 0;
}

// Send a notification on a channel with payload
int32_t __pg_notify(const char* channel, const char* payload_str) {
    if (!g_pg || !g_pg->connected || !channel) return -1;

    char sql[1024];
    if (payload_str && payload_str[0]) {
        snprintf(sql, sizeof(sql), "NOTIFY %s, '%s'", channel, payload_str);
    } else {
        snprintf(sql, sizeof(sql), "NOTIFY %s", channel);
    }

    int sql_len = strlen(sql);
    int msg_len = 4 + sql_len + 1;
    char* msg = malloc(1 + msg_len);
    msg[0] = 'Q';
    pg_write_i32(msg + 1, msg_len);
    memcpy(msg + 5, sql, sql_len + 1);
    int rc = pg_send_raw(msg, 1 + msg_len);
    free(msg);
    if (rc < 0) return -1;

    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    while (1) {
        if (pg_read_msg(&mtype, payload, &plen) < 0) return -1;
        if (mtype == 'Z') break;
    }

    pg_log("NOTIFY %s OK", channel);
    return 0;
}

// Poll for pending notifications — non-blocking
// Returns: channel\tpayload\tpid  (tab-separated) or "" if nothing
// The caller should handle "" as "no notification available"
//
// This works by checking if any pending 'A' (NotificationResponse)
// messages arrived during previous query responses.
// For real-time use, call after a simple query like "SELECT 1"
// to trigger reading the notification off the wire.
char* __pg_poll_notify(void) {
    if (!g_pg || !g_pg->connected) return strdup("");

    // Send a trivial query to flush any pending notifications
    const char* sql = "";
    int sql_len = 0;

    // Instead of sending an empty query, we use a Sync message approach:
    // Check if data is available on the socket using select()
    fd_set rfds;
    struct timeval tv;
    FD_ZERO(&rfds);
    FD_SET(g_pg->fd, &rfds);
    tv.tv_sec = 0;
    tv.tv_usec = 0; // non-blocking

    int ready = select(g_pg->fd + 1, &rfds, NULL, NULL, &tv);
    if (ready <= 0) return strdup(""); // nothing pending

    // Data available — read message
    char mtype;
    char payload[PG_BUF_SIZE];
    int plen;
    if (pg_read_msg(&mtype, payload, &plen) < 0) return strdup("");

    if (mtype == 'A') {
        // NotificationResponse: int32 pid + str channel + str payload
        int pid = pg_read_i32(payload);
        char* channel = payload + 4;
        char* notif_payload = channel + strlen(channel) + 1;
        char result[2048];
        snprintf(result, sizeof(result), "%s\t%s\t%d", channel, notif_payload, pid);
        pg_log("NOTIFY received: channel=%s payload=%s pid=%d", channel, notif_payload, pid);
        return strdup(result);
    }

    return strdup("");
}
