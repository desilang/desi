#ifndef DESI_STD_H
#define DESI_STD_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ---- I/O shims ---- */

/* Read entire file into a newly allocated NUL-terminated buffer.
   Returns NULL on failure. Caller owns the buffer and must free it
   with desi_mem_free(). */
const char* desi_fs_read_all(const char* path);

/* Write entire buffer to a file. Returns 0 on success, non-zero on error. */
int desi_fs_write_all(const char* path, const char* data);

/* Exit the process with the given code. (Returns only in tests.) */
int desi_os_exit(int code);

/* ---- String / memory shims ---- */

/* Returns a newly allocated string of (a + b).
   If a or b is NULL, treats it as "". Caller must free with desi_mem_free(). */
const char* desi_str_concat(const char* a, const char* b);

/* Free memory returned by runtime shims (concat/read_all). NULL is ok. */
void desi_mem_free(const void* p);

/* Return the byte length of s (strlen). NULL -> 0. */
int desi_str_len(const char* s);

/* Return the unsigned byte at index i (0..len-1), or -1 if OOB or s==NULL. */
int desi_str_at(const char* s, int i);

/* Allocate and return a 1-character string from byte code c (clamped 0..255). */
const char* desi_str_from_code(int c);

#ifdef __cplusplus
}
#endif

#endif /* DESI_STD_H */
