/*
 * shell.c — Shell scripting helpers for Desi stdlib
 *
 * High-level wrappers for common shell operations.
 * Makes Desi a viable replacement for bash scripts.
 *
 * Public API:
 *   __shell_exec(cmd)                → run shell command, return stdout
 *   __shell_exec_status(cmd)         → run command, return exit code
 *   __shell_glob(pattern)            → glob files, return "\n"-separated paths
 *   __shell_which(cmd)               → find command in PATH
 *   __shell_cp(src, dst)             → copy file
 *   __shell_mv(src, dst)             → move/rename file
 *   __shell_rm(path)                 → remove file
 *   __shell_mkdir_p(path)            → mkdir -p
 *   __shell_rmdir(path)              → rmdir (empty dir only)
 *   __shell_exists(path)             → check file/dir exists
 *   __shell_is_file(path)            → check if regular file
 *   __shell_is_dir(path)             → check if directory
 *   __shell_cat(path)                → read file contents
 *   __shell_write(path, content)     → write content to file
 *   __shell_append(path, content)    → append to file
 *   __shell_lines(path)              → read file lines count
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <sys/stat.h>
#include <errno.h>

#ifdef _WIN32
  #include <windows.h>
  #include <direct.h>      /* _mkdir, _rmdir */
  #include <io.h>          /* _unlink */
  #define popen  _popen
  #define pclose _pclose
  #define unlink _unlink
  #define rmdir  _rmdir
  #define mkdir(path, mode) _mkdir(path)
  #ifndef S_ISREG
    #define S_ISREG(m) (((m) & _S_IFMT) == _S_IFREG)
  #endif
  #ifndef S_ISDIR
    #define S_ISDIR(m) (((m) & _S_IFMT) == _S_IFDIR)
  #endif
#else
  #include <unistd.h>
  #include <glob.h>
#endif

/* ============================================================
 * Command execution
 * ============================================================ */

char* __shell_exec(const char* cmd) {
    if (!cmd) return strdup("");
    FILE* fp = popen(cmd, "r");
    if (!fp) return strdup("");

    size_t cap = 4096, len = 0;
    char* buf = (char*)malloc(cap);
    size_t n;
    while ((n = fread(buf + len, 1, cap - len - 1, fp)) > 0) {
        len += n;
        if (len >= cap - 1) {
            cap *= 2;
            buf = (char*)realloc(buf, cap);
        }
    }
    buf[len] = '\0';
    pclose(fp);

    /* Trim trailing newline */
    while (len > 0 && buf[len-1] == '\n') buf[--len] = '\0';
    return buf;
}

int32_t __shell_exec_status(const char* cmd) {
    if (!cmd) return -1;
    return system(cmd);
}

/* ============================================================
 * File globbing
 * ============================================================ */

#ifdef _WIN32
static int shell_cmp_str(const void* a, const void* b) {
    return strcmp(*(const char* const*)a, *(const char* const*)b);
}

/* FindFirstFile-based glob: supports * and ? in the final path component
 * (which covers the common "dir/*.ext" patterns; no braces/tilde). Returns
 * dir-prefixed paths, sorted, "\n"-separated — same shape as POSIX glob. */
char* __shell_glob(const char* pattern) {
    if (!pattern) return strdup("");

    /* Split into directory prefix + mask */
    const char* sep1 = strrchr(pattern, '/');
    const char* sep2 = strrchr(pattern, '\\');
    const char* sep = (sep1 > sep2) ? sep1 : sep2;
    size_t prefix_len = sep ? (size_t)(sep - pattern) + 1 : 0;

    WIN32_FIND_DATAA fd;
    HANDLE h = FindFirstFileA(pattern, &fd);
    if (h == INVALID_HANDLE_VALUE) return strdup("");

    size_t n = 0, ncap = 16;
    char** names = (char**)malloc(ncap * sizeof(char*));
    do {
        if (strcmp(fd.cFileName, ".") == 0 || strcmp(fd.cFileName, "..") == 0)
            continue;
        size_t name_len = strlen(fd.cFileName);
        char* full = (char*)malloc(prefix_len + name_len + 1);
        memcpy(full, pattern, prefix_len);
        memcpy(full + prefix_len, fd.cFileName, name_len + 1);
        if (n >= ncap) { ncap *= 2; names = (char**)realloc(names, ncap * sizeof(char*)); }
        names[n++] = full;
    } while (FindNextFileA(h, &fd));
    FindClose(h);

    qsort(names, n, sizeof(char*), shell_cmp_str);

    size_t cap = 4096, len = 0;
    char* buf = (char*)malloc(cap);
    buf[0] = '\0';
    for (size_t i = 0; i < n; i++) {
        size_t plen = strlen(names[i]);
        while (len + plen + 2 >= cap) { cap *= 2; buf = (char*)realloc(buf, cap); }
        if (i > 0) buf[len++] = '\n';
        memcpy(buf + len, names[i], plen);
        len += plen;
        free(names[i]);
    }
    buf[len] = '\0';
    free(names);
    return buf;
}
#else
char* __shell_glob(const char* pattern) {
    if (!pattern) return strdup("");
    glob_t results;
    int ret = glob(pattern, GLOB_TILDE | GLOB_BRACE, NULL, &results);
    if (ret != 0) return strdup("");

    size_t cap = 4096, len = 0;
    char* buf = (char*)malloc(cap);
    buf[0] = '\0';

    for (size_t i = 0; i < results.gl_pathc; i++) {
        size_t plen = strlen(results.gl_pathv[i]);
        while (len + plen + 2 >= cap) { cap *= 2; buf = realloc(buf, cap); }
        if (i > 0) buf[len++] = '\n';
        memcpy(buf + len, results.gl_pathv[i], plen);
        len += plen;
    }
    buf[len] = '\0';

    globfree(&results);
    return buf;
}
#endif

/* ============================================================
 * Path lookup
 * ============================================================ */

char* __shell_which(const char* cmd) {
    if (!cmd) return strdup("");
    char find_cmd[512];
#ifdef _WIN32
    snprintf(find_cmd, sizeof(find_cmd), "where %s 2>nul", cmd);
    char* out = __shell_exec(find_cmd);
    /* `where` lists every match — keep only the first line, like which */
    char* nl = strchr(out, '\n');
    if (nl) *nl = '\0';
    char* cr = strchr(out, '\r');
    if (cr) *cr = '\0';
    return out;
#else
    snprintf(find_cmd, sizeof(find_cmd), "which %s 2>/dev/null", cmd);
    return __shell_exec(find_cmd);
#endif
}

/* ============================================================
 * File operations
 * ============================================================ */

int32_t __shell_cp(const char* src, const char* dst) {
    if (!src || !dst) return -1;
    FILE* in = fopen(src, "rb");
    if (!in) return -1;
    FILE* out = fopen(dst, "wb");
    if (!out) { fclose(in); return -1; }

    char buf[8192];
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), in)) > 0) {
        fwrite(buf, 1, n, out);
    }
    fclose(in);
    fclose(out);
    return 0;
}

int32_t __shell_mv(const char* src, const char* dst) {
    if (!src || !dst) return -1;
    if (rename(src, dst) == 0) return 0;
    /* If rename fails (cross-device), try cp + rm */
    if (__shell_cp(src, dst) == 0) {
        unlink(src);
        return 0;
    }
    return -1;
}

int32_t __shell_rm(const char* path) {
    if (!path) return -1;
    return unlink(path) == 0 ? 0 : -1;
}

int32_t __shell_mkdir_p(const char* path) {
    if (!path) return -1;
    char tmp[1024];
    snprintf(tmp, sizeof(tmp), "%s", path);
#ifdef _WIN32
    for (char* p = tmp + 1; *p; p++) {
        if (*p == '/' || *p == '\\') {
            char saved = *p;
            *p = '\0';
            mkdir(tmp, 0755);
            *p = saved;
        }
    }
#else
    for (char* p = tmp + 1; *p; p++) {
        if (*p == '/') {
            *p = '\0';
            mkdir(tmp, 0755);
            *p = '/';
        }
    }
#endif
    return mkdir(tmp, 0755) == 0 || errno == EEXIST ? 0 : -1;
}

int32_t __shell_rmdir(const char* path) {
    if (!path) return -1;
    return rmdir(path) == 0 ? 0 : -1;
}

int32_t __shell_exists(const char* path) {
    if (!path) return 0;
    struct stat st;
    return stat(path, &st) == 0 ? 1 : 0;
}

int32_t __shell_is_file(const char* path) {
    if (!path) return 0;
    struct stat st;
    return (stat(path, &st) == 0 && S_ISREG(st.st_mode)) ? 1 : 0;
}

int32_t __shell_is_dir(const char* path) {
    if (!path) return 0;
    struct stat st;
    return (stat(path, &st) == 0 && S_ISDIR(st.st_mode)) ? 1 : 0;
}

char* __shell_cat(const char* path) {
    if (!path) return strdup("");
    FILE* f = fopen(path, "r");
    if (!f) return strdup("");
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* buf = (char*)malloc(sz + 1);
    fread(buf, 1, sz, f);
    buf[sz] = '\0';
    fclose(f);
    return buf;
}

int32_t __shell_write(const char* path, const char* content) {
    if (!path) return -1;
    FILE* f = fopen(path, "w");
    if (!f) return -1;
    if (content) fputs(content, f);
    fclose(f);
    return 0;
}

int32_t __shell_append(const char* path, const char* content) {
    if (!path) return -1;
    FILE* f = fopen(path, "a");
    if (!f) return -1;
    if (content) fputs(content, f);
    fclose(f);
    return 0;
}

int32_t __shell_lines(const char* path) {
    if (!path) return 0;
    FILE* f = fopen(path, "r");
    if (!f) return 0;
    int count = 0;
    int c;
    while ((c = fgetc(f)) != EOF) {
        if (c == '\n') count++;
    }
    fclose(f);
    return count;
}
