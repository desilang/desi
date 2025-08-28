#include "desi_std.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>

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
  /* In a real runtime we’d call exit(code). Keeping it returnable eases testing. */
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
