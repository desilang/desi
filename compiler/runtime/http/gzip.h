/*
 * gzip.h — Gzip compression for Desi HTTP stack
 *
 * Uses system zlib on macOS/Linux (linked via -lz).
 * On Windows, provides a stub that disables compression gracefully.
 *
 * API:
 *   desi_gzip_compress(in, in_len, out_len)  → malloc'd gzip data or NULL
 *   desi_gzip_decompress(in, in_len, out_len) → malloc'd data or NULL
 *   desi_gzip_available()                    → 1 if gzip is available
 */

#ifndef DESI_GZIP_H
#define DESI_GZIP_H

#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
  /* Windows: try to use zlib if linked, otherwise stub */
  #if defined(DESI_HAS_ZLIB)
    #include <zlib.h>
    #define DESI_GZIP_AVAILABLE 1
  #else
    #define DESI_GZIP_AVAILABLE 0
  #endif
#else
  /* macOS/Linux: zlib is always available */
  #include <zlib.h>
  #define DESI_GZIP_AVAILABLE 1
#endif

static int desi_gzip_available(void) {
    return DESI_GZIP_AVAILABLE;
}

#if DESI_GZIP_AVAILABLE

/*
 * Compress data to gzip format.
 * Returns malloc'd buffer on success (caller must free), NULL on failure.
 * Sets *out_len to the compressed data length.
 */
static unsigned char* desi_gzip_compress(const unsigned char* in, size_t in_len, size_t* out_len) {
    if (!in || in_len == 0) return NULL;

    /* Initialize zlib for gzip format (windowBits = 15 + 16 for gzip) */
    z_stream strm;
    memset(&strm, 0, sizeof(strm));
    if (deflateInit2(&strm, Z_DEFAULT_COMPRESSION, Z_DEFLATED,
                     15 + 16, /* gzip wrapper */
                     8, Z_DEFAULT_STRATEGY) != Z_OK) {
        return NULL;
    }

    /* Worst case: input + overhead */
    size_t max_out = deflateBound(&strm, (uLong)in_len);
    unsigned char* buf = (unsigned char*)malloc(max_out);
    if (!buf) {
        deflateEnd(&strm);
        return NULL;
    }

    strm.next_in = (Bytef*)in;
    strm.avail_in = (uInt)in_len;
    strm.next_out = buf;
    strm.avail_out = (uInt)max_out;

    int ret = deflate(&strm, Z_FINISH);
    if (ret != Z_STREAM_END) {
        deflateEnd(&strm);
        free(buf);
        return NULL;
    }

    *out_len = strm.total_out;
    deflateEnd(&strm);
    return buf;
}

/*
 * Decompress gzip data.
 * Returns malloc'd buffer on success (caller must free), NULL on failure.
 * Sets *out_len to the decompressed data length.
 */
static unsigned char* desi_gzip_decompress(const unsigned char* in, size_t in_len, size_t* out_len) {
    if (!in || in_len == 0) return NULL;

    /* Initialize zlib for gzip auto-detect (windowBits = 15 + 32) */
    z_stream strm;
    memset(&strm, 0, sizeof(strm));
    if (inflateInit2(&strm, 15 + 32) != Z_OK) {
        return NULL;
    }

    /* Start with 4x input size, grow if needed */
    size_t buf_size = in_len * 4;
    if (buf_size < 4096) buf_size = 4096;
    unsigned char* buf = (unsigned char*)malloc(buf_size);
    if (!buf) {
        inflateEnd(&strm);
        return NULL;
    }

    strm.next_in = (Bytef*)in;
    strm.avail_in = (uInt)in_len;

    size_t total_out = 0;
    int ret;

    do {
        if (total_out >= buf_size) {
            buf_size *= 2;
            unsigned char* new_buf = (unsigned char*)realloc(buf, buf_size);
            if (!new_buf) {
                inflateEnd(&strm);
                free(buf);
                return NULL;
            }
            buf = new_buf;
        }

        strm.next_out = buf + total_out;
        strm.avail_out = (uInt)(buf_size - total_out);

        ret = inflate(&strm, Z_NO_FLUSH);
        if (ret != Z_OK && ret != Z_STREAM_END) {
            inflateEnd(&strm);
            free(buf);
            return NULL;
        }

        total_out = strm.total_out;
    } while (ret != Z_STREAM_END);

    *out_len = total_out;
    inflateEnd(&strm);
    return buf;
}

#else /* !DESI_GZIP_AVAILABLE — stubs for Windows without zlib */

static unsigned char* desi_gzip_compress(const unsigned char* in, size_t in_len, size_t* out_len) {
    (void)in; (void)in_len; (void)out_len;
    return NULL;
}

static unsigned char* desi_gzip_decompress(const unsigned char* in, size_t in_len, size_t* out_len) {
    (void)in; (void)in_len; (void)out_len;
    return NULL;
}

#endif /* DESI_GZIP_AVAILABLE */

#endif /* DESI_GZIP_H */
