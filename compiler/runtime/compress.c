/*
 * compress.c — gzip/zlib compression for Desi stdlib
 *
 * Uses zlib (already linked via -lz):
 *   - gzip compress/decompress (strings)
 *   - zlib deflate/inflate (raw)
 *   - File compression/decompression
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <zlib.h>

// ============================================================
// In-memory Compression
// ============================================================

// Compress string data using zlib deflate
// Returns base64-like binary string (caller must handle encoding)
// For simplicity, returns hex-encoded compressed data
static char* to_hex(const unsigned char* data, int len) {
    char* hex = malloc(len * 2 + 1);
    for (int i = 0; i < len; i++) {
        sprintf(hex + i * 2, "%02x", data[i]);
    }
    hex[len * 2] = '\0';
    return hex;
}

static int from_hex_char(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return 0;
}

static unsigned char* from_hex(const char* hex, int* out_len) {
    int hlen = strlen(hex);
    *out_len = hlen / 2;
    unsigned char* data = malloc(*out_len);
    for (int i = 0; i < *out_len; i++) {
        data[i] = (from_hex_char(hex[i*2]) << 4) | from_hex_char(hex[i*2+1]);
    }
    return data;
}

// Compress a string, returns hex-encoded compressed data
char* __compress_deflate(const char* input) {
    if (!input) return strdup("");
    
    uLong src_len = strlen(input);
    uLong dst_len = compressBound(src_len);
    unsigned char* dst = malloc(dst_len);
    
    int ret = compress2(dst, &dst_len, (const unsigned char*)input, src_len, Z_DEFAULT_COMPRESSION);
    if (ret != Z_OK) {
        free(dst);
        return strdup("");
    }
    
    char* hex = to_hex(dst, (int)dst_len);
    free(dst);
    return hex;
}

// Decompress hex-encoded compressed data back to string
char* __compress_inflate(const char* hex_data) {
    if (!hex_data || strlen(hex_data) == 0) return strdup("");
    
    int comp_len;
    unsigned char* comp = from_hex(hex_data, &comp_len);
    
    // Start with a reasonable buffer, expand if needed
    uLong dst_len = comp_len * 10;
    if (dst_len < 256) dst_len = 256;
    unsigned char* dst = malloc(dst_len);
    
    int ret = uncompress(dst, &dst_len, comp, comp_len);
    if (ret == Z_BUF_ERROR) {
        // Buffer too small, try larger
        dst_len = comp_len * 100;
        dst = realloc(dst, dst_len);
        ret = uncompress(dst, &dst_len, comp, comp_len);
    }
    
    free(comp);
    
    if (ret != Z_OK) {
        free(dst);
        return strdup("");
    }
    
    char* result = malloc(dst_len + 1);
    memcpy(result, dst, dst_len);
    result[dst_len] = '\0';
    free(dst);
    return result;
}

// Get compressed size ratio as percentage
// Returns ratio * 100 (e.g., 45 means 45% of original)
int32_t __compress_ratio(const char* original, const char* compressed_hex) {
    if (!original || !compressed_hex) return 100;
    int orig_len = strlen(original);
    int comp_len = strlen(compressed_hex) / 2; // hex = 2 chars per byte
    if (orig_len == 0) return 100;
    return (int)((double)comp_len / (double)orig_len * 100.0);
}

// ============================================================
// File Compression
// ============================================================

// Compress a file using gzip, saves to output_path
int32_t __compress_gzip_file(const char* input_path, const char* output_path) {
    if (!input_path || !output_path) return -1;
    
    FILE* in = fopen(input_path, "rb");
    if (!in) return -1;
    
    gzFile out = gzopen(output_path, "wb9");
    if (!out) { fclose(in); return -1; }
    
    char buf[8192];
    int bytes_read;
    while ((bytes_read = fread(buf, 1, sizeof(buf), in)) > 0) {
        gzwrite(out, buf, bytes_read);
    }
    
    fclose(in);
    gzclose(out);
    return 0;
}

// Decompress a gzip file
int32_t __compress_gunzip_file(const char* input_path, const char* output_path) {
    if (!input_path || !output_path) return -1;
    
    gzFile in = gzopen(input_path, "rb");
    if (!in) return -1;
    
    FILE* out = fopen(output_path, "wb");
    if (!out) { gzclose(in); return -1; }
    
    char buf[8192];
    int bytes_read;
    while ((bytes_read = gzread(in, buf, sizeof(buf))) > 0) {
        fwrite(buf, 1, bytes_read, out);
    }
    
    gzclose(in);
    fclose(out);
    return 0;
}
