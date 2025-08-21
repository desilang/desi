#ifndef DESI_RT_H
#define DESI_RT_H

#include <stdint.h>
#include <stdatomic.h>

#ifdef __cplusplus
extern "C" {
#endif

// Reference-counted header (atomic for cross-thread safety).
typedef struct { _Atomic uint32_t rc; } desi_hdr;

// Retain/release — Stage-0 stubs; real impl arrives with ARC runtime.
void* desi_retain(void* p);
void  desi_release(void* p);

#ifdef __cplusplus
}
#endif

#endif // DESI_RT_H
