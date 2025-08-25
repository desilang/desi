#ifndef DESI_STD_H
#define DESI_STD_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// Read entire file into an allocated buffer (NUL-terminated).
// Returns NULL on error. Caller may free() the result.
char* desi_fs_read_all(const char* path);

// Write whole string to a file, overwrite. Returns 0 on success, -1 on error.
int desi_fs_write_all(const char* path, const char* data);

// Exit process with the given code.
void desi_os_exit(int code);

#ifdef __cplusplus
}
#endif

#endif /* DESI_STD_H */
