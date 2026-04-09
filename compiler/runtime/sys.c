/*
 * sys.c — System information for Desi
 *
 * Provides version info, runtime limits, and system metadata.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <limits.h>

// Desi version
const char* __sys_version(void) {
    return "0.1.0";
}

// Desi version components
int32_t __sys_version_major(void) { return 0; }
int32_t __sys_version_minor(void) { return 1; }
int32_t __sys_version_patch(void) { return 0; }

// Maximum integer value (64-bit)
int64_t __sys_maxsize(void) {
    return INT64_MAX;
}

// Pointer size in bytes (4 on 32-bit, 8 on 64-bit)
int32_t __sys_sizeof_ptr(void) {
    return (int32_t)sizeof(void*);
}

// Byte order: "little" or "big"
const char* __sys_byteorder(void) {
    volatile uint32_t test = 1;
    return (*(volatile uint8_t*)&test) ? "little" : "big";
}

// Get the recursion limit (from limits.c)
extern int64_t __desi_get_max_recursion(void);
int32_t __sys_recursion_limit(void) {
    return (int32_t)__desi_get_max_recursion();
}

// Get current call stack depth
extern int64_t __desi_get_call_depth(void);
int32_t __sys_call_depth(void) {
    return (int32_t)__desi_get_call_depth();
}
