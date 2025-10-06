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

/* Concatenate a and b and return a newly allocated string (heap).
   If a or b is NULL, treats it as "".
   On hard allocation failure, returns NULL.
   Caller must free non-NULL with desi_mem_free(). */
const char* desi_str_concat(const char* a, const char* b);

/* Free memory returned by runtime shims (concat/read_all/from_code). NULL is ok. */
void desi_mem_free(const void* p);

/* Return the byte length of s (strlen). NULL -> 0. */
int desi_str_len(const char* s);

/* Return the unsigned byte at index i (0..len-1), or -1 if OOB or s==NULL. */
int desi_str_at(const char* s, int i);

/* Allocate and return a 1-character string from byte code c (clamped 0..255).
   On hard allocation failure, returns NULL.
   Caller must free non-NULL with desi_mem_free(). */
const char* desi_str_from_code(int c);

/* ---- Minimal future/executor (M11) ---- */

/* A generic single-thread future handle with small vtable. */
typedef struct desi_future {
  int  (*poll)(void* self);              /* return 0:Pending, 1:Ready */
  void (*destroy)(void* self);           /* optional */
  int  (*get_int)(void* self);           /* optional: for Future[int] */
  const char* (*get_str)(void* self);    /* optional: for Future[str] */
  void* self;                            /* user state */
} desi_future;

/* Construct a future handle. All callbacks may be NULL except poll. */
desi_future desi_future_make(
  int (*poll)(void*),
  void (*destroy)(void*),
  int (*get_int)(void*),
  const char* (*get_str)(void*),
  void* self
);

/* Convenience helpers */
static inline int desi_future_poll(desi_future f) {
  return f.poll ? f.poll(f.self) : 1;
}
static inline void desi_future_destroy(desi_future f) {
  if (f.destroy) f.destroy(f.self);
}

/* Executor: run-to-completion (spin-poll). */
int desi_task_block_on_int(desi_future f);       /* returns result of get_int */
void desi_task_block_on_void(desi_future f);     /* waits only */

/* Timer future (becomes ready >= ms after creation). */
desi_future desi_task_sleep_ms(int ms);

#ifdef __cplusplus
}
#endif

#endif /* DESI_STD_H */
