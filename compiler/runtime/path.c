// path.c - Path module C runtime
// Provides filesystem path manipulation and query functions.
// Uses __path_ prefix to avoid collisions.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>
#include <libgen.h>

// ============================================================
// Path queries
// ============================================================

// Check if path exists (file or directory)
int __path_exists(const char* p) {
    if (!p) return 0;
    struct stat st;
    return stat(p, &st) == 0;
}

// Check if path is a regular file
int __path_is_file(const char* p) {
    if (!p) return 0;
    struct stat st;
    if (stat(p, &st) != 0) return 0;
    return S_ISREG(st.st_mode);
}

// Check if path is a directory
int __path_is_dir(const char* p) {
    if (!p) return 0;
    struct stat st;
    if (stat(p, &st) != 0) return 0;
    return S_ISDIR(st.st_mode);
}

// Check if path is absolute
int __path_is_abs(const char* p) {
    if (!p || *p == '\0') return 0;
    return p[0] == '/';
}

// Get file size in bytes, -1 on error
int __path_size(const char* p) {
    if (!p) return -1;
    struct stat st;
    if (stat(p, &st) != 0) return -1;
    return (int)st.st_size;
}

// ============================================================
// Path manipulation
// ============================================================

// Get the base name (last component) of a path
char* __path_basename(const char* p) {
    if (!p || *p == '\0') return strdup("");
    // basename() may modify input, so copy it
    char* copy = strdup(p);
    if (!copy) return strdup("");
    char* base = basename(copy);
    char* result = strdup(base);
    free(copy);
    return result ? result : strdup("");
}

// Get the directory name (all but last component)
char* __path_dirname(const char* p) {
    if (!p || *p == '\0') return strdup(".");
    char* copy = strdup(p);
    if (!copy) return strdup(".");
    char* dir = dirname(copy);
    char* result = strdup(dir);
    free(copy);
    return result ? result : strdup(".");
}

// Get file extension (including the dot), or empty string
char* __path_ext(const char* p) {
    if (!p) return strdup("");
    const char* dot = strrchr(p, '.');
    const char* slash = strrchr(p, '/');
    // Ensure dot is in the filename, not in directory part
    if (!dot || (slash && dot < slash)) return strdup("");
    // Don't count hidden files like ".bashrc" as having an extension
    if (dot == p || (slash && dot == slash + 1)) return strdup("");
    return strdup(dot);
}

// Remove extension from path
char* __path_stem(const char* p) {
    if (!p) return strdup("");
    // Get basename first
    char* copy = strdup(p);
    if (!copy) return strdup("");
    char* base = basename(copy);
    char* result = strdup(base);
    free(copy);
    if (!result) return strdup("");

    // Find last dot (not at start)
    char* dot = strrchr(result, '.');
    if (dot && dot != result) {
        *dot = '\0';
    }
    return result;
}

// Join two path components with separator
char* __path_join(const char* a, const char* b) {
    if (!a || *a == '\0') return strdup(b ? b : "");
    if (!b || *b == '\0') return strdup(a);
    // If b is absolute, just return b
    if (b[0] == '/') return strdup(b);

    size_t alen = strlen(a);
    size_t blen = strlen(b);
    int need_sep = (a[alen - 1] != '/');
    size_t total = alen + blen + (need_sep ? 1 : 0);

    char* result = (char*)malloc(total + 1);
    if (!result) return strdup(a);

    memcpy(result, a, alen);
    if (need_sep) result[alen++] = '/';
    memcpy(result + alen, b, blen);
    result[alen + blen] = '\0';
    return result;
}

// Get absolute path (resolve relative to cwd)
char* __path_abs(const char* p) {
    if (!p) return strdup("");
    char* resolved = realpath(p, NULL);
    if (resolved) return resolved;
    // If path doesn't exist, manually join with cwd
    if (p[0] == '/') return strdup(p);
    char cwd[4096];
    if (getcwd(cwd, sizeof(cwd)) == NULL) return strdup(p);
    return __path_join(cwd, p);
}

// Normalize path: resolve . and .. components, remove duplicate slashes
char* __path_clean(const char* p) {
    if (!p || *p == '\0') return strdup(".");

    // Simple approach: use realpath if path exists
    char* resolved = realpath(p, NULL);
    if (resolved) return resolved;

    // For non-existent paths, do basic normalization
    size_t len = strlen(p);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup(p);

    int j = 0;
    int is_abs = (p[0] == '/');
    if (is_abs) result[j++] = '/';

    for (size_t i = (is_abs ? 1 : 0); i < len; ) {
        // Skip duplicate slashes
        while (i < len && p[i] == '/') i++;
        if (i >= len) break;

        // Find end of component
        size_t start = i;
        while (i < len && p[i] != '/') i++;

        size_t comp_len = i - start;
        // Skip "." components
        if (comp_len == 1 && p[start] == '.') continue;

        // Add separator if needed
        if (j > (is_abs ? 1 : 0)) result[j++] = '/';
        memcpy(result + j, p + start, comp_len);
        j += comp_len;
    }

    if (j == 0) {
        result[0] = '.';
        j = 1;
    }
    result[j] = '\0';
    return result;
}
