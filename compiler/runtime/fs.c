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

// ============================================================
// Recursive directory walking
// ============================================================

// Helper: recursively collect paths into a DesiList
static void walk_recurse(const char* dir_path, DesiList* result) {
    DIR* dir = opendir(dir_path);
    if (!dir) return;

    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
            continue;

        // Build full path: dir_path + "/" + entry->d_name
        size_t dir_len = strlen(dir_path);
        size_t name_len = strlen(entry->d_name);
        char* full_path = (char*)malloc(dir_len + 1 + name_len + 1);
        if (!full_path) continue;

        memcpy(full_path, dir_path, dir_len);
        full_path[dir_len] = '/';
        memcpy(full_path + dir_len + 1, entry->d_name, name_len);
        full_path[dir_len + 1 + name_len] = '\0';

        list_append(result, (void*)full_path, 1);

        // Recurse into subdirectories
        struct stat st;
        if (stat(full_path, &st) == 0 && S_ISDIR(st.st_mode)) {
            walk_recurse(full_path, result);
        }
    }
    closedir(dir);
}

// Walk directory recursively. Returns list[str] of all full paths.
// Python: os.walk() / pathlib.Path.rglob()
// Go: filepath.Walk()
// Rust: walkdir::WalkDir
DesiList* __fs_walk(const char* path) {
    DesiList* result = list_new(1, (ElemToStrFunc)str_to_str);
    if (!path) return result;
    walk_recurse(path, result);
    return result;
}

// ============================================================
// Temporary files and directories
// ============================================================

// Create a temporary file. Returns the path as a heap string.
// Python: tempfile.mkstemp()
// Go: os.CreateTemp()
char* __fs_temp_file(void) {
#ifdef _WIN32
    char tmp_path[MAX_PATH];
    char tmp_file[MAX_PATH];
    GetTempPathA(MAX_PATH, tmp_path);
    if (GetTempFileNameA(tmp_path, "desi", 0, tmp_file)) {
        return strdup(tmp_file);
    }
    return strdup("");
#else
    const char* tmpdir = getenv("TMPDIR");
    if (!tmpdir) tmpdir = "/tmp";
    size_t len = strlen(tmpdir) + 20;
    char* tmpl = (char*)malloc(len);
    if (!tmpl) return strdup("");
    snprintf(tmpl, len, "%s/desi_XXXXXX", tmpdir);
    int fd = mkstemp(tmpl);
    if (fd < 0) {
        free(tmpl);
        return strdup("");
    }
    close(fd);
    return tmpl; // caller owns this string
#endif
}

// Create a temporary directory. Returns the path as a heap string.
// Python: tempfile.mkdtemp()
// Go: os.MkdirTemp()
char* __fs_temp_dir(void) {
#ifdef _WIN32
    char tmp_path[MAX_PATH];
    GetTempPathA(MAX_PATH, tmp_path);
    char dir_name[MAX_PATH];
    snprintf(dir_name, MAX_PATH, "%sdesi_%lu", tmp_path, (unsigned long)GetCurrentProcessId());
    CreateDirectoryA(dir_name, NULL);
    return strdup(dir_name);
#else
    const char* tmpdir = getenv("TMPDIR");
    if (!tmpdir) tmpdir = "/tmp";
    size_t len = strlen(tmpdir) + 20;
    char* tmpl = (char*)malloc(len);
    if (!tmpl) return strdup("");
    snprintf(tmpl, len, "%s/desi_XXXXXX", tmpdir);
    char* result = mkdtemp(tmpl);
    if (!result) {
        free(tmpl);
        return strdup("");
    }
    return tmpl; // mkdtemp modifies tmpl in-place
#endif
}

// ============================================================
// Recursive removal
// ============================================================

// Remove directory and all contents recursively (like rm -rf).
// Returns 1 on success, 0 on error.
// Python: shutil.rmtree()
// Go: os.RemoveAll()
int __fs_remove_all(const char* path) {
    if (!path) return 0;

    struct stat st;
    if (stat(path, &st) != 0) return 0;

    // If it's a file, just unlink
    if (!S_ISDIR(st.st_mode)) {
        return unlink(path) == 0 ? 1 : 0;
    }

    // It's a directory — recurse
    DIR* dir = opendir(path);
    if (!dir) return 0;

    int success = 1;
    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
            continue;

        size_t path_len = strlen(path);
        size_t name_len = strlen(entry->d_name);
        char* child = (char*)malloc(path_len + 1 + name_len + 1);
        if (!child) { success = 0; continue; }

        snprintf(child, path_len + 1 + name_len + 1, "%s/%s", path, entry->d_name);

        if (!__fs_remove_all(child)) {
            success = 0;
        }
        free(child);
    }
    closedir(dir);

    if (rmdir(path) != 0) success = 0;
    return success;
}

// ============================================================
// Recursive directory copy
// ============================================================

// Copy directory and all contents recursively (like cp -r).
// Returns 1 on success, 0 on error.
// Python: shutil.copytree()
// Go: (no stdlib equivalent, manual recursion)
int __fs_copy_dir(const char* src, const char* dst) {
    if (!src || !dst) return 0;

    struct stat st;
    if (stat(src, &st) != 0) return 0;

    // If source is a file, copy it directly
    if (!S_ISDIR(st.st_mode)) {
        return __fs_copy(src, dst);
    }

    // Create destination directory
    if (mkdir(dst, st.st_mode) != 0 && errno != EEXIST) return 0;

    DIR* dir = opendir(src);
    if (!dir) return 0;

    int success = 1;
    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
            continue;

        size_t src_len = strlen(src);
        size_t dst_len = strlen(dst);
        size_t name_len = strlen(entry->d_name);

        char* src_child = (char*)malloc(src_len + 1 + name_len + 1);
        char* dst_child = (char*)malloc(dst_len + 1 + name_len + 1);
        if (!src_child || !dst_child) {
            free(src_child); free(dst_child);
            success = 0;
            continue;
        }

        snprintf(src_child, src_len + 1 + name_len + 1, "%s/%s", src, entry->d_name);
        snprintf(dst_child, dst_len + 1 + name_len + 1, "%s/%s", dst, entry->d_name);

        if (!__fs_copy_dir(src_child, dst_child)) {
            success = 0;
        }
        free(src_child);
        free(dst_child);
    }
    closedir(dir);
    return success;
}

// ============================================================
// File metadata
// ============================================================

// Return file size in bytes (int64 for large files).
// Returns -1 on error.
int64_t __fs_file_size(const char* path) {
    if (!path) return -1;
    struct stat st;
    if (stat(path, &st) != 0) return -1;
    return (int64_t)st.st_size;
}

// Return file modification time as Unix timestamp (seconds since epoch).
// Returns -1 on error.
int64_t __fs_mtime(const char* path) {
    if (!path) return -1;
    struct stat st;
    if (stat(path, &st) != 0) return -1;
    return (int64_t)st.st_mtime;
}

// Return file permissions as octal integer (e.g. 0755 → 493).
// Returns -1 on error.
int __fs_permissions(const char* path) {
    if (!path) return -1;
    struct stat st;
    if (stat(path, &st) != 0) return -1;
    return (int)(st.st_mode & 0777);
}

// ============================================================
// Executable lookup (which)
// ============================================================

// Find an executable on the system PATH.
// Returns the full path if found, "" if not found.
// Python: shutil.which()
// Go: exec.LookPath()
char* __fs_which(const char* cmd) {
    if (!cmd || !*cmd) return strdup("");

    // If cmd contains a slash, check it directly
    if (strchr(cmd, '/') != NULL) {
        if (access(cmd, X_OK) == 0) {
            return strdup(cmd);
        }
        return strdup("");
    }

    const char* path_env = getenv("PATH");
    if (!path_env) return strdup("");

    char* path_copy = strdup(path_env);
    if (!path_copy) return strdup("");

    char* dir = path_copy;
    char* next;
    while (dir && *dir) {
        next = strchr(dir, ':');
        if (next) *next = '\0';

        // Build full path: dir/cmd
        size_t dir_len = strlen(dir);
        size_t cmd_len = strlen(cmd);
        char* full = (char*)malloc(dir_len + 1 + cmd_len + 1);
        if (!full) { dir = next ? next + 1 : NULL; continue; }

        snprintf(full, dir_len + 1 + cmd_len + 1, "%s/%s", dir, cmd);

        if (access(full, X_OK) == 0) {
            free(path_copy);
            return full;
        }
        free(full);

        dir = next ? next + 1 : NULL;
    }

    free(path_copy);
    return strdup("");
}

// ============================================================
// Set file permissions (chmod)
// ============================================================

// Set file permissions. Mode is octal (e.g. 0o755 → 493 decimal).
// Returns 1 on success, 0 on error.
// Python: os.chmod()
// Go: os.Chmod()
int __fs_chmod(const char* path, int mode) {
    if (!path) return 0;
#ifdef _WIN32
    return 0; // Windows doesn't support chmod in POSIX style
#else
    return chmod(path, (mode_t)mode) == 0 ? 1 : 0;
#endif
}

