// fs.c - File system module
// Pure C, no external deps. Provides file I/O and directory operations.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <dirent.h>
#include <unistd.h>
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include "list.h"

// Helper: string to_str callback for list_new
static const char* str_to_str(const void* item) {
    return (const char*)item;
}

// ============================================================
// File reading/writing
// ============================================================

// Read entire file contents as a string. Returns "" on error.
char* __fs_read(const char* path) {
    if (!path) return strdup("");
    FILE* f = fopen(path, "rb");
    if (!f) return strdup("");

    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);

    if (size < 0) { fclose(f); return strdup(""); }

    char* buf = (char*)malloc(size + 1);
    if (!buf) { fclose(f); return strdup(""); }

    size_t read = fread(buf, 1, size, f);
    fclose(f);
    buf[read] = '\0';
    return buf;
}

// Write string to file (overwrites). Returns 1 on success, 0 on error.
int __fs_write(const char* path, const char* content) {
    if (!path || !content) return 0;
    FILE* f = fopen(path, "wb");
    if (!f) return 0;
    size_t len = strlen(content);
    size_t written = fwrite(content, 1, len, f);
    fclose(f);
    return written == len ? 1 : 0;
}

// Append string to file. Returns 1 on success, 0 on error.
int __fs_append(const char* path, const char* content) {
    if (!path || !content) return 0;
    FILE* f = fopen(path, "ab");
    if (!f) return 0;
    size_t len = strlen(content);
    size_t written = fwrite(content, 1, len, f);
    fclose(f);
    return written == len ? 1 : 0;
}

// Read file as list of lines (strips \n).
DesiList* __fs_read_lines(const char* path) {
    DesiList* result = list_new(1, (ElemToStrFunc)str_to_str); // type_tag 1 = str
    if (!path) return result;
    FILE* f = fopen(path, "r");
    if (!f) return result;

    char line[8192];
    while (fgets(line, sizeof(line), f)) {
        // Strip trailing newline
        size_t len = strlen(line);
        while (len > 0 && (line[len-1] == '\n' || line[len-1] == '\r'))
            line[--len] = '\0';
        list_append(result, (void*)strdup(line), 1);
    }
    fclose(f);
    return result;
}

// Write list of lines to file (adds \n after each line).
int __fs_write_lines(const char* path, DesiList* lines) {
    if (!path || !lines) return 0;
    FILE* f = fopen(path, "wb");
    if (!f) return 0;

    int64_t len = list_len(lines);
    for (int64_t i = 0; i < len; i++) {
        const char* line = (const char*)list_get(lines, i);
        if (line) {
            fputs(line, f);
            fputc('\n', f);
        }
    }
    fclose(f);
    return 1;
}

// ============================================================
// File/directory queries
// ============================================================

int __fs_exists(const char* path) {
    if (!path) return 0;
    struct stat st;
    return stat(path, &st) == 0 ? 1 : 0;
}

int __fs_is_file(const char* path) {
    if (!path) return 0;
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISREG(st.st_mode) ? 1 : 0;
}

int __fs_is_dir(const char* path) {
    if (!path) return 0;
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISDIR(st.st_mode) ? 1 : 0;
}

int __fs_size(const char* path) {
    if (!path) return -1;
    struct stat st;
    if (stat(path, &st) != 0) return -1;
    return (int)st.st_size;
}

// ============================================================
// Directory operations
// ============================================================

// Create directory (with parents). Returns 1 on success.
int __fs_mkdir(const char* path) {
    if (!path) return 0;
    // Use mkdir -p equivalent: create each component
    char* tmp = strdup(path);
    if (!tmp) return 0;
    size_t len = strlen(tmp);
    if (tmp[len - 1] == '/') tmp[len - 1] = '\0';

    for (char* p = tmp + 1; *p; p++) {
        if (*p == '/') {
            *p = '\0';
            mkdir(tmp, 0755);
            *p = '/';
        }
    }
    int result = mkdir(tmp, 0755) == 0 || errno == EEXIST ? 1 : 0;
    free(tmp);
    return result;
}

// List directory contents. Returns list[str] of entry names.
DesiList* __fs_list_dir(const char* path) {
    DesiList* result = list_new(1, (ElemToStrFunc)str_to_str);
    if (!path) return result;
    DIR* dir = opendir(path);
    if (!dir) return result;

    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        // Skip . and ..
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
            continue;
        list_append(result, (void*)strdup(entry->d_name), 1);
    }
    closedir(dir);
    return result;
}

// ============================================================
// File operations
// ============================================================

// Remove file or empty directory. Returns 1 on success.
int __fs_remove(const char* path) {
    if (!path) return 0;
    // Try unlink first (file), then rmdir (empty dir)
    if (unlink(path) == 0) return 1;
    if (rmdir(path) == 0) return 1;
    return 0;
}

// Copy file. Returns 1 on success.
int __fs_copy(const char* src, const char* dst) {
    if (!src || !dst) return 0;
    FILE* in = fopen(src, "rb");
    if (!in) return 0;
    FILE* out = fopen(dst, "wb");
    if (!out) { fclose(in); return 0; }

    char buf[8192];
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), in)) > 0) {
        if (fwrite(buf, 1, n, out) != n) {
            fclose(in);
            fclose(out);
            return 0;
        }
    }
    fclose(in);
    fclose(out);
    return 1;
}

// Move/rename file. Returns 1 on success.
int __fs_move(const char* src, const char* dst) {
    if (!src || !dst) return 0;
    // Try rename first (same filesystem), fallback to copy+delete
    if (rename(src, dst) == 0) return 1;
    if (__fs_copy(src, dst)) {
        unlink(src);
        return 1;
    }
    return 0;
}

// ============================================================
// Absolute path resolution
// ============================================================

char* __fs_abs(const char* path) {
    if (!path) return strdup("");
    char resolved[PATH_MAX];
    if (realpath(path, resolved) != NULL) {
        return strdup(resolved);
    }
    return strdup(path);
}
