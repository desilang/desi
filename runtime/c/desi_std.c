#include "desi_std.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#ifdef _WIN32
  #include <windows.h>
#else
  #include <sys/time.h>
  #include <unistd.h>
#endif

/* ---- I/O shims ---- */

const char* desi_fs_read_all(const char* path) {
  if (!path) return NULL;
  FILE* f = fopen(path, "rb");
  if (!f) return NULL;

  if (fseek(f, 0, SEEK_END) != 0) { fclose(f); return NULL; }
  long len = ftell(f);
  if (len < 0) { fclose(f); return NULL; }
  if (fseek(f, 0, SEEK_SET) != 0) { fclose(f); return NULL; }

  size_t n = (size_t)len;
  char* buf = (char*)malloc(n + 1);
  if (!buf) { fclose(f); return NULL; }

  size_t rd = fread(buf, 1, n, f);
  fclose(f);
  if (rd != n) { free(buf); return NULL; }

  buf[n] = '\0';
  return (const char*)buf;
}

int desi_fs_write_all(const char* path, const char* data) {
  if (!path || !data) return -1;
  FILE* f = fopen(path, "wb");
  if (!f) return -1;
  size_t n = strlen(data);
  size_t wr = fwrite(data, 1, n, f);
  int rc = 0;
  if (wr != n) rc = -1;
  if (fclose(f) != 0) rc = -1;
  return rc;
}

int desi_os_exit(int code) {
#ifdef DESI_RUNTIME_EXIT_CALLS_EXIT
  exit(code);
#endif
  return code;
}

/* ---- String / memory shims ---- */

const char* desi_str_concat(const char* a, const char* b) {
  if (!a) a = "";
  if (!b) b = "";
  size_t na = strlen(a);
  size_t nb = strlen(b);
  char* out = (char*)malloc(na + nb + 1);
  if (!out) return "";
  memcpy(out, a, na);
  memcpy(out + na, b, nb);
  out[na + nb] = '\0';
  return (const char*)out;
}

void desi_mem_free(const void* p) {
  if (p) free((void*)p);
}

int desi_str_len(const char* s) {
  if (!s) return 0;
  size_t n = strlen(s);
  if (n > 0x7fffffff) n = 0x7fffffff;
  return (int)n;
}

int desi_str_at(const char* s, int i) {
  if (!s || i < 0) return -1;
  size_t n = strlen(s);
  if ((size_t)i >= n) return -1;
  unsigned char ch = (unsigned char)s[i];
  return (int)ch;
}

const char* desi_str_from_code(int c) {
  if (c < 0) c = 0;
  if (c > 255) c = 255;
  char* out = (char*)malloc(2);
  if (!out) return "";
  out[0] = (char)(unsigned char)c;
  out[1] = '\0';
  return (const char*)out;
}

/* ---- Minimal future/executor (M11) ---- */

desi_future desi_future_make(
  int (*poll)(void*),
  void (*destroy)(void*),
  int (*get_int)(void*),
  const char* (*get_str)(void*),
  void* self
) {
  desi_future f;
  f.poll    = poll;
  f.destroy = destroy;
  f.get_int = get_int;
  f.get_str = get_str;
  f.self    = self;
  return f;
}

/* wall-clock now (us) */
static uint64_t desi_now_us(void) {
#ifdef _WIN32
  LARGE_INTEGER freq, counter;
  QueryPerformanceFrequency(&freq);
  QueryPerformanceCounter(&counter);
  return (uint64_t)((counter.QuadPart * 1000000ULL) / freq.QuadPart);
#else
  struct timeval tv;
  gettimeofday(&tv, NULL);
  return (uint64_t)tv.tv_sec * 1000000ULL + (uint64_t)tv.tv_usec;
#endif
}

/* tiny sleep to avoid hot spinning in examples */
static void desi_tiny_sleep_us(int us) {
#ifdef _WIN32
  if (us <= 0) return;
  /* Sleep works in ms granularity */
  int ms = us / 1000;
  if (ms <= 0) ms = 1;
  Sleep((DWORD)ms);
#else
  if (us <= 0) return;
  usleep((useconds_t)us);
#endif
}

/* ---- timer future ---- */

typedef struct {
  int ready;
  uint64_t wake_us;
} desi_sleep_state;

static int desi_sleep_poll(void* selfv) {
  desi_sleep_state* s = (desi_sleep_state*)selfv;
  if (!s) return 1;
  if (s->ready) return 1;
  uint64_t now = desi_now_us();
  if (now >= s->wake_us) {
    s->ready = 1;
    return 1;
  }
  /* cooperative backoff */
  desi_tiny_sleep_us(1000);
  return 0;
}

static void desi_sleep_destroy(void* selfv) {
  if (selfv) free(selfv);
}

desi_future desi_task_sleep_ms(int ms) {
  if (ms < 0) ms = 0;
  desi_sleep_state* s = (desi_sleep_state*)malloc(sizeof(desi_sleep_state));
  if (!s) {
    /* degenerate immediately-ready future */
    desi_sleep_state* z = NULL;
    return desi_future_make(desi_sleep_poll, desi_sleep_destroy, NULL, NULL, z);
  }
  s->ready = 0;
  s->wake_us = desi_now_us() + (uint64_t)ms * 1000ULL;
  return desi_future_make(desi_sleep_poll, desi_sleep_destroy, NULL, NULL, s);
}

/* ---- executor ---- */

int desi_task_block_on_int(desi_future f) {
  while (!desi_future_poll(f)) {
    /* cooperative spin; sleep a hair to avoid pegging CPU */
    desi_tiny_sleep_us(1000);
  }
  int out = 0;
  if (f.get_int) out = f.get_int(f.self);
  desi_future_destroy(f);
  return out;
}

void desi_task_block_on_void(desi_future f) {
  while (!desi_future_poll(f)) {
    desi_tiny_sleep_us(1000);
  }
  desi_future_destroy(f);
}
