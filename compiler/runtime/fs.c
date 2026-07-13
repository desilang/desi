// fs.c - File system module
// Pure C, no external deps. Provides file I/O and directory operations.
// Cross-platform: Win32 API on Windows, POSIX on Linux/macOS.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <errno.h>
#include <limits.h>

#ifdef _WIN32
  #include <windows.h>
  #include <direct.h>      /* _mkdir, _rmdir */
  #include <io.h>          /* _unlink, _access */

  #define mkdir(path, mode) _mkdir(path)
  #define rmdir _rmdir
  #define unlink _unlink
  #define access _access
  #ifndef X_OK
    #define X_OK 0         /* existence check — Windows has no exec bit */
  #endif
  #ifndef PATH_MAX
    #define PATH_MAX MAX_PATH
  #endif
  #ifndef S_ISREG
    #define S_ISREG(m) (((m) & _S_IFMT) == _S_IFREG)
  #endif
  #ifndef S_ISDIR
    #define S_ISDIR(m) (((m) & _S_IFMT) == _S_IFDIR)
  #endif
  /* realpath(p, buf) — resolve only when the path exists, like POSIX */
  static char* desi_fs_realpath(const char* p, char* buf) {
      struct stat st;
      if (stat(p, &st) != 0) return NULL;
      return _fullpath(buf, p, PATH_MAX);
  }
  #define realpath(p, buf) desi_fs_realpath((p), (buf))

  /* Minimal dirent shim over FindFirstFile/FindNextFile */
  struct dirent { char d_name[MAX_PATH]; };
  typedef struct {
      HANDLE handle;
      WIN32_FIND_DATAA fd;
      struct dirent entry;
      int first;
  } DIR;

  static DIR* opendir(const char* path) {
      if (!path) return NULL;
      size_t len = strlen(path);
      char* pattern = (char*)malloc(len + 3);
      if (!pattern) return NULL;
      memcpy(pattern, path, len);
      /* append \* (or just * after an existing separator) */
      if (len > 0 && path[len - 1] != '/' && path[len - 1] != '\\') {
          pattern[len++] = '\\';
      }
      pattern[len++] = '*';
      pattern[len] = '\0';

      DIR* d = (DIR*)malloc(sizeof(DIR));
      if (!d) { free(pattern); return NULL; }
      d->handle = FindFirstFileA(pattern, &d->fd);
      free(pattern);
      if (d->handle == INVALID_HANDLE_VALUE) { free(d); return NULL; }
      d->first = 1;
      return d;
  }

  static struct dirent* readdir(DIR* d) {
      if (!d) return NULL;
      if (d->first) {
          d->first = 0;
      } else if (!FindNextFileA(d->handle, &d->fd)) {
          return NULL;
      }
      strncpy(d->entry.d_name, d->fd.cFileName, MAX_PATH - 1);
      d->entry.d_name[MAX_PATH - 1] = '\0';
      return &d->entry;
  }

  static void closedir(DIR* d) {
      if (!d) return;
      FindClose(d->handle);
      free(d);
  }
#else
  #include <dirent.h>
  #include <unistd.h>
  #include <fcntl.h>
#endif

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
#ifdef _WIN32
    if (tmp[len - 1] == '/' || tmp[len - 1] == '\\') tmp[len - 1] = '\0';

    for (char* p = tmp + 1; *p; p++) {
        if (*p == '/' || *p == '\\') {
            char saved = *p;
            *p = '\0';
            mkdir(tmp, 0755);
            *p = saved;
        }
    }
#else
    if (tmp[len - 1] == '/') tmp[len - 1] = '\0';

    for (char* p = tmp + 1; *p; p++) {
        if (*p == '/') {
            *p = '\0';
            mkdir(tmp, 0755);
            *p = '/';
        }
    }
#endif
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

#ifdef _WIN32
    const char PATH_LIST_SEP = ';';
    int has_sep = strchr(cmd, '/') != NULL || strchr(cmd, '\\') != NULL;
#else
    const char PATH_LIST_SEP = ':';
    int has_sep = strchr(cmd, '/') != NULL;
#endif

    // If cmd contains a separator, check it directly
    if (has_sep) {
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
        next = strchr(dir, PATH_LIST_SEP);
        if (next) *next = '\0';

        // Build full path: dir/cmd (+ room for ".exe" on Windows)
        size_t dir_len = strlen(dir);
        size_t cmd_len = strlen(cmd);
        char* full = (char*)malloc(dir_len + 1 + cmd_len + 5);
        if (!full) { dir = next ? next + 1 : NULL; continue; }

        snprintf(full, dir_len + 1 + cmd_len + 1, "%s/%s", dir, cmd);

        if (access(full, X_OK) == 0) {
            free(path_copy);
            return full;
        }
#ifdef _WIN32
        // Retry with .exe appended (PATHEXT-lite)
        strcat(full, ".exe");
        if (access(full, X_OK) == 0) {
            free(path_copy);
            return full;
        }
#endif
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

