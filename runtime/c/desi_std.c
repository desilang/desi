#include "desi_std.h"

#include <stdio.h>
#include <stdlib.h>

char* desi_fs_read_all(const char* path) {
  FILE* f = fopen(path, "rb");
  if (!f) return NULL;
  if (fseek(f, 0, SEEK_END) != 0) { fclose(f); return NULL; }
  long n = ftell(f);
  if (n < 0) { fclose(f); return NULL; }
  if (fseek(f, 0, SEEK_SET) != 0) { fclose(f); return NULL; }
  char* buf = (char*)malloc((size_t)n + 1);
  if (!buf) { fclose(f); return NULL; }
  size_t rd = fread(buf, 1, (size_t)n, f);
  fclose(f);
  if (rd != (size_t)n) { free(buf); return NULL; }
  buf[n] = '\0';
  return buf;
}

int desi_fs_write_all(const char* path, const char* data) {
  FILE* f = fopen(path, "wb");
  if (!f) return -1;
  size_t len = 0;
  if (data) for (; data[len]; ++len) {}
  size_t wr = fwrite(data, 1, len, f);
  fclose(f);
  return wr == len ? 0 : -1;
}

void desi_os_exit(int code) {
  exit(code);
}
