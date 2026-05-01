// hash.c - Cryptographic hashing module
// Self-contained: uses CommonCrypto on macOS, embedded implementations on Linux.
// NO OpenSSL dependency for hashing.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#if defined(__APPLE__)
#define COMMON_DIGEST_FOR_OPENSSL
#include <CommonCrypto/CommonDigest.h>
#include <CommonCrypto/CommonHmac.h>
#endif

static char* to_hex(const unsigned char* data, size_t len) {
    char* hex = (char*)malloc(2 * len + 1);
    if (!hex) return strdup("");
    for (size_t i = 0; i < len; i++) {
        sprintf(hex + 2 * i, "%02x", data[i]);
    }
    hex[2 * len] = '\0';
    return hex;
}

// ============================================================
// Self-contained implementations (used on non-Apple platforms)
// ============================================================
#if !defined(__APPLE__)

// --- MD5 (RFC 1321) ---
#define MD5_DIGEST_LENGTH 16

static const uint32_t md5_s[] = {
    7,12,17,22, 7,12,17,22, 7,12,17,22, 7,12,17,22,
    5, 9,14,20, 5, 9,14,20, 5, 9,14,20, 5, 9,14,20,
    4,11,16,23, 4,11,16,23, 4,11,16,23, 4,11,16,23,
    6,10,15,21, 6,10,15,21, 6,10,15,21, 6,10,15,21
};
static const uint32_t md5_K[] = {
    0xd76aa478,0xe8c7b756,0x242070db,0xc1bdceee,0xf57c0faf,0x4787c62a,0xa8304613,0xfd469501,
    0x698098d8,0x8b44f7af,0xffff5bb1,0x895cd7be,0x6b901122,0xfd987193,0xa679438e,0x49b40821,
    0xf61e2562,0xc040b340,0x265e5a51,0xe9b6c7aa,0xd62f105d,0x02441453,0xd8a1e681,0xe7d3fbc8,
    0x21e1cde6,0xc33707d6,0xf4d50d87,0x455a14ed,0xa9e3e905,0xfcefa3f8,0x676f02d9,0x8d2a4c8a,
    0xfffa3942,0x8771f681,0x6d9d6122,0xfde5380c,0xa4beea44,0x4bdecfa9,0xf6bb4b60,0xbebfbc70,
    0x289b7ec6,0xeaa127fa,0xd4ef3085,0x04881d05,0xd9d4d039,0xe6db99e5,0x1fa27cf8,0xc4ac5665,
    0xf4292244,0x432aff97,0xab9423a7,0xfc93a039,0x655b59c3,0x8f0ccc92,0xffeff47d,0x85845dd1,
    0x6fa87e4f,0xfe2ce6e0,0xa3014314,0x4e0811a1,0xf7537e82,0xbd3af235,0x2ad7d2bb,0xeb86d391
};

static void md5_compute(const uint8_t* msg, size_t len, uint8_t out[16]) {
    uint32_t a0=0x67452301, b0=0xefcdab89, c0=0x98badcfe, d0=0x10325476;
    size_t new_len = ((len + 8) / 64 + 1) * 64;
    uint8_t* buf = (uint8_t*)calloc(new_len, 1);
    memcpy(buf, msg, len);
    buf[len] = 0x80;
    uint64_t bits = (uint64_t)len * 8;
    memcpy(buf + new_len - 8, &bits, 8);
    for (size_t off = 0; off < new_len; off += 64) {
        uint32_t* M = (uint32_t*)(buf + off);
        uint32_t A=a0, B=b0, C=c0, D=d0;
        for (int i = 0; i < 64; i++) {
            uint32_t F, g;
            if (i < 16)      { F=(B&C)|((~B)&D); g=i; }
            else if (i < 32) { F=(D&B)|((~D)&C); g=(5*i+1)%16; }
            else if (i < 48) { F=B^C^D;          g=(3*i+5)%16; }
            else              { F=C^(B|(~D));      g=(7*i)%16; }
            F += A + md5_K[i] + M[g];
            A = D; D = C; C = B;
            B += (F << md5_s[i]) | (F >> (32 - md5_s[i]));
        }
        a0+=A; b0+=B; c0+=C; d0+=D;
    }
    free(buf);
    memcpy(out,    &a0, 4); memcpy(out+4,  &b0, 4);
    memcpy(out+8,  &c0, 4); memcpy(out+12, &d0, 4);
}

// --- SHA-1 (FIPS 180-4) ---
#define SHA1_DIGEST_LENGTH 20

static void sha1_compute(const uint8_t* msg, size_t len, uint8_t out[20]) {
    uint32_t h0=0x67452301, h1=0xEFCDAB89, h2=0x98BADCFE, h3=0x10325476, h4=0xC3D2E1F0;
    size_t new_len = ((len + 8) / 64 + 1) * 64;
    uint8_t* buf = (uint8_t*)calloc(new_len, 1);
    memcpy(buf, msg, len);
    buf[len] = 0x80;
    uint64_t bits = __builtin_bswap64((uint64_t)len * 8);
    memcpy(buf + new_len - 8, &bits, 8);
    for (size_t off = 0; off < new_len; off += 64) {
        uint32_t w[80];
        for (int i = 0; i < 16; i++) {
            w[i] = __builtin_bswap32(((uint32_t*)(buf+off))[i]);
        }
        for (int i = 16; i < 80; i++) {
            uint32_t t = w[i-3]^w[i-8]^w[i-14]^w[i-16];
            w[i] = (t<<1)|(t>>31);
        }
        uint32_t a=h0,b=h1,c=h2,d=h3,e=h4;
        for (int i = 0; i < 80; i++) {
            uint32_t f, k;
            if (i<20)      { f=(b&c)|((~b)&d); k=0x5A827999; }
            else if (i<40) { f=b^c^d;           k=0x6ED9EBA1; }
            else if (i<60) { f=(b&c)|(b&d)|(c&d); k=0x8F1BBCDC; }
            else           { f=b^c^d;           k=0xCA62C1D6; }
            uint32_t t = ((a<<5)|(a>>27)) + f + e + k + w[i];
            e=d; d=c; c=(b<<30)|(b>>2); b=a; a=t;
        }
        h0+=a; h1+=b; h2+=c; h3+=d; h4+=e;
    }
    free(buf);
    h0=__builtin_bswap32(h0); h1=__builtin_bswap32(h1);
    h2=__builtin_bswap32(h2); h3=__builtin_bswap32(h3); h4=__builtin_bswap32(h4);
    memcpy(out,   &h0,4); memcpy(out+4, &h1,4); memcpy(out+8, &h2,4);
    memcpy(out+12,&h3,4); memcpy(out+16,&h4,4);
}

// --- SHA-256 (FIPS 180-4) ---
#define SHA256_DIGEST_LENGTH 32

static const uint32_t sha256_k[64] = {
    0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
    0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
    0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
    0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
    0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
    0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
    0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
    0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2
};

#define RR(x,n) (((x)>>(n))|((x)<<(32-(n))))
#define CH(x,y,z) (((x)&(y))^((~(x))&(z)))
#define MAJ(x,y,z) (((x)&(y))^((x)&(z))^((y)&(z)))
#define EP0(x) (RR(x,2)^RR(x,13)^RR(x,22))
#define EP1(x) (RR(x,6)^RR(x,11)^RR(x,25))
#define SIG0(x) (RR(x,7)^RR(x,18)^((x)>>3))
#define SIG1(x) (RR(x,17)^RR(x,19)^((x)>>10))

typedef struct { uint32_t state[8]; uint8_t buf[64]; uint64_t count; } sha256_ctx;

static void sha256_init(sha256_ctx* c) {
    c->state[0]=0x6a09e667; c->state[1]=0xbb67ae85; c->state[2]=0x3c6ef372; c->state[3]=0xa54ff53a;
    c->state[4]=0x510e527f; c->state[5]=0x9b05688c; c->state[6]=0x1f83d9ab; c->state[7]=0x5be0cd19;
    c->count = 0;
}

static void sha256_transform(sha256_ctx* c, const uint8_t blk[64]) {
    uint32_t w[64], a,b,cc,d,e,f,g,h;
    for (int i=0;i<16;i++) w[i]=__builtin_bswap32(((uint32_t*)blk)[i]);
    for (int i=16;i<64;i++) w[i]=SIG1(w[i-2])+w[i-7]+SIG0(w[i-15])+w[i-16];
    a=c->state[0];b=c->state[1];cc=c->state[2];d=c->state[3];
    e=c->state[4];f=c->state[5];g=c->state[6];h=c->state[7];
    for (int i=0;i<64;i++) {
        uint32_t t1=h+EP1(e)+CH(e,f,g)+sha256_k[i]+w[i];
        uint32_t t2=EP0(a)+MAJ(a,b,cc);
        h=g;g=f;f=e;e=d+t1;d=cc;cc=b;b=a;a=t1+t2;
    }
    c->state[0]+=a;c->state[1]+=b;c->state[2]+=cc;c->state[3]+=d;
    c->state[4]+=e;c->state[5]+=f;c->state[6]+=g;c->state[7]+=h;
}

static void sha256_update(sha256_ctx* c, const uint8_t* data, size_t len) {
    for (size_t i=0;i<len;i++) {
        c->buf[c->count%64]=(uint8_t)data[i];
        c->count++;
        if (c->count%64==0) sha256_transform(c, c->buf);
    }
}

static void sha256_final(sha256_ctx* c, uint8_t out[32]) {
    uint64_t bits=c->count*8;
    size_t pad=c->count%64;
    c->buf[pad++]=0x80;
    if (pad>56) { memset(c->buf+pad,0,64-pad); sha256_transform(c,c->buf); pad=0; }
    memset(c->buf+pad,0,56-pad);
    uint64_t be_bits=__builtin_bswap64(bits);
    memcpy(c->buf+56,&be_bits,8);
    sha256_transform(c,c->buf);
    for (int i=0;i<8;i++) { uint32_t v=__builtin_bswap32(c->state[i]); memcpy(out+i*4,&v,4); }
}

// --- SHA-512 (FIPS 180-4) ---
#define SHA512_DIGEST_LENGTH 64

static const uint64_t sha512_k[80] = {
    0x428a2f98d728ae22ULL,0x7137449123ef65cdULL,0xb5c0fbcfec4d3b2fULL,0xe9b5dba58189dbbcULL,
    0x3956c25bf348b538ULL,0x59f111f1b605d019ULL,0x923f82a4af194f9bULL,0xab1c5ed5da6d8118ULL,
    0xd807aa98a3030242ULL,0x12835b0145706fbeULL,0x243185be4ee4b28cULL,0x550c7dc3d5ffb4e2ULL,
    0x72be5d74f27b896fULL,0x80deb1fe3b1696b1ULL,0x9bdc06a725c71235ULL,0xc19bf174cf692694ULL,
    0xe49b69c19ef14ad2ULL,0xefbe4786384f25e3ULL,0x0fc19dc68b8cd5b5ULL,0x240ca1cc77ac9c65ULL,
    0x2de92c6f592b0275ULL,0x4a7484aa6ea6e483ULL,0x5cb0a9dcbd41fbd4ULL,0x76f988da831153b5ULL,
    0x983e5152ee66dfabULL,0xa831c66d2db43210ULL,0xb00327c898fb213fULL,0xbf597fc7beef0ee4ULL,
    0xc6e00bf33da88fc2ULL,0xd5a79147930aa725ULL,0x06ca6351e003826fULL,0x142929670a0e6e70ULL,
    0x27b70a8546d22ffcULL,0x2e1b21385c26c926ULL,0x4d2c6dfc5ac42aedULL,0x53380d139d95b3dfULL,
    0x650a73548baf63deULL,0x766a0abb3c77b2a8ULL,0x81c2c92e47edaee6ULL,0x92722c851482353bULL,
    0xa2bfe8a14cf10364ULL,0xa81a664bbc423001ULL,0xc24b8b70d0f89791ULL,0xc76c51a30654be30ULL,
    0xd192e819d6ef5218ULL,0xd69906245565a910ULL,0xf40e35855771202aULL,0x106aa07032bbd1b8ULL,
    0x19a4c116b8d2d0c8ULL,0x1e376c085141ab53ULL,0x2748774cdf8eeb99ULL,0x34b0bcb5e19b48a8ULL,
    0x391c0cb3c5c95a63ULL,0x4ed8aa4ae3418acbULL,0x5b9cca4f7763e373ULL,0x682e6ff3d6b2b8a3ULL,
    0x748f82ee5defb2fcULL,0x78a5636f43172f60ULL,0x84c87814a1f0ab72ULL,0x8cc702081a6439ecULL,
    0x90befffa23631e28ULL,0xa4506cebde82bde9ULL,0xbef9a3f7b2c67915ULL,0xc67178f2e372532bULL,
    0xca273eceea26619cULL,0xd186b8c721c0c207ULL,0xeada7dd6cde0eb1eULL,0xf57d4f7fee6ed178ULL,
    0x06f067aa72176fbaULL,0x0a637dc5a2c898a6ULL,0x113f9804bef90daeULL,0x1b710b35131c471bULL,
    0x28db77f523047d84ULL,0x32caab7b40c72493ULL,0x3c9ebe0a15c9bebcULL,0x431d67c49c100d4cULL,
    0x4cc5d4becb3e42b6ULL,0x597f299cfc657e2aULL,0x5fcb6fab3ad6faecULL,0x6c44198c4a475817ULL
};

#define RR64(x,n) (((x)>>(n))|((x)<<(64-(n))))

static void sha512_compute(const uint8_t* msg, size_t len, uint8_t out[64]) {
    uint64_t h[8] = {
        0x6a09e667f3bcc908ULL,0xbb67ae8584caa73bULL,0x3c6ef372fe94f82bULL,0xa54ff53a5f1d36f1ULL,
        0x510e527fade682d1ULL,0x9b05688c2b3e6c1fULL,0x1f83d9abfb41bd6bULL,0x5be0cd19137e2179ULL
    };
    size_t new_len = ((len + 16) / 128 + 1) * 128;
    uint8_t* buf = (uint8_t*)calloc(new_len, 1);
    memcpy(buf, msg, len);
    buf[len] = 0x80;
    uint64_t bits = __builtin_bswap64((uint64_t)len * 8);
    memcpy(buf + new_len - 8, &bits, 8);
    for (size_t off = 0; off < new_len; off += 128) {
        uint64_t w[80];
        for (int i=0;i<16;i++) {
            uint64_t v; memcpy(&v, buf+off+i*8, 8);
            w[i] = __builtin_bswap64(v);
        }
        for (int i=16;i<80;i++) {
            uint64_t s0 = RR64(w[i-15],1)^RR64(w[i-15],8)^(w[i-15]>>7);
            uint64_t s1 = RR64(w[i-2],19)^RR64(w[i-2],61)^(w[i-2]>>6);
            w[i] = w[i-16]+s0+w[i-7]+s1;
        }
        uint64_t a=h[0],b=h[1],c=h[2],d=h[3],e=h[4],f=h[5],g=h[6],hh=h[7];
        for (int i=0;i<80;i++) {
            uint64_t S1=RR64(e,14)^RR64(e,18)^RR64(e,41);
            uint64_t ch=(e&f)^((~e)&g);
            uint64_t t1=hh+S1+ch+sha512_k[i]+w[i];
            uint64_t S0=RR64(a,28)^RR64(a,34)^RR64(a,39);
            uint64_t mj=(a&b)^(a&c)^(b&c);
            uint64_t t2=S0+mj;
            hh=g;g=f;f=e;e=d+t1;d=c;c=b;b=a;a=t1+t2;
        }
        h[0]+=a;h[1]+=b;h[2]+=c;h[3]+=d;h[4]+=e;h[5]+=f;h[6]+=g;h[7]+=hh;
    }
    free(buf);
    for (int i=0;i<8;i++) { uint64_t v=__builtin_bswap64(h[i]); memcpy(out+i*8,&v,8); }
}

// --- HMAC-SHA256 (RFC 2104) ---
static void hmac_sha256_compute(const char* key, size_t klen, const char* msg, size_t mlen, uint8_t out[32]) {
    uint8_t k_pad[64];
    memset(k_pad, 0, 64);
    if (klen > 64) {
        sha256_ctx c; sha256_init(&c);
        sha256_update(&c, (const uint8_t*)key, klen);
        sha256_final(&c, k_pad);
    } else {
        memcpy(k_pad, key, klen);
    }
    uint8_t i_pad[64], o_pad[64];
    for (int i=0;i<64;i++) { i_pad[i]=k_pad[i]^0x36; o_pad[i]=k_pad[i]^0x5c; }
    // inner hash
    sha256_ctx c; sha256_init(&c);
    sha256_update(&c, i_pad, 64);
    sha256_update(&c, (const uint8_t*)msg, mlen);
    uint8_t inner[32]; sha256_final(&c, inner);
    // outer hash
    sha256_init(&c);
    sha256_update(&c, o_pad, 64);
    sha256_update(&c, inner, 32);
    sha256_final(&c, out);
}

// Compatibility aliases for the CC_* API used by the rest of the file
#define CC_MD5_DIGEST_LENGTH  MD5_DIGEST_LENGTH
#define CC_SHA1_DIGEST_LENGTH SHA1_DIGEST_LENGTH
#define CC_SHA256_DIGEST_LENGTH SHA256_DIGEST_LENGTH
#define CC_SHA512_DIGEST_LENGTH SHA512_DIGEST_LENGTH

#endif // !__APPLE__

// ============================================================
// Public hash functions
// ============================================================

char* __hash_md5(const char* input) {
    if (!input) return strdup("");
    unsigned char digest[CC_MD5_DIGEST_LENGTH];
#if defined(__APPLE__)
    CC_MD5(input, (CC_LONG)strlen(input), digest);
#else
    md5_compute((const uint8_t*)input, strlen(input), digest);
#endif
    return to_hex(digest, CC_MD5_DIGEST_LENGTH);
}

char* __hash_sha1(const char* input) {
    if (!input) return strdup("");
    unsigned char digest[CC_SHA1_DIGEST_LENGTH];
#if defined(__APPLE__)
    CC_SHA1(input, (CC_LONG)strlen(input), digest);
#else
    sha1_compute((const uint8_t*)input, strlen(input), digest);
#endif
    return to_hex(digest, CC_SHA1_DIGEST_LENGTH);
}

char* __hash_sha256(const char* input) {
    if (!input) return strdup("");
    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
#if defined(__APPLE__)
    CC_SHA256(input, (CC_LONG)strlen(input), digest);
#else
    sha256_ctx c; sha256_init(&c);
    sha256_update(&c, (const uint8_t*)input, strlen(input));
    sha256_final(&c, digest);
#endif
    return to_hex(digest, CC_SHA256_DIGEST_LENGTH);
}

char* __hash_sha512(const char* input) {
    if (!input) return strdup("");
    unsigned char digest[CC_SHA512_DIGEST_LENGTH];
#if defined(__APPLE__)
    CC_SHA512(input, (CC_LONG)strlen(input), digest);
#else
    sha512_compute((const uint8_t*)input, strlen(input), digest);
#endif
    return to_hex(digest, CC_SHA512_DIGEST_LENGTH);
}

// ============================================================
// HMAC
// ============================================================

char* __hash_hmac_sha256(const char* key, const char* msg) {
    if (!key || !msg) return strdup("");
    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
#if defined(__APPLE__)
    CCHmac(kCCHmacAlgSHA256, key, strlen(key), msg, strlen(msg), digest);
#else
    hmac_sha256_compute(key, strlen(key), msg, strlen(msg), digest);
#endif
    return to_hex(digest, CC_SHA256_DIGEST_LENGTH);
}

// ============================================================
// File hashing
// ============================================================

char* __hash_file(const char* path) {
    if (!path) return strdup("");
    FILE* f = fopen(path, "rb");
    if (!f) return strdup("");

#if defined(__APPLE__)
    CC_SHA256_CTX cc_ctx;
    CC_SHA256_Init(&cc_ctx);
    unsigned char buf[8192]; size_t n;
    while ((n = fread(buf, 1, sizeof(buf), f)) > 0)
        CC_SHA256_Update(&cc_ctx, buf, (CC_LONG)n);
    fclose(f);
    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
    CC_SHA256_Final(digest, &cc_ctx);
#else
    sha256_ctx ctx;
    sha256_init(&ctx);
    unsigned char buf[8192]; size_t n;
    while ((n = fread(buf, 1, sizeof(buf), f)) > 0)
        sha256_update(&ctx, buf, n);
    fclose(f);
    unsigned char digest[SHA256_DIGEST_LENGTH];
    sha256_final(&ctx, digest);
#endif
    return to_hex(digest, CC_SHA256_DIGEST_LENGTH);
}
