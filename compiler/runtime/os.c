// os.c - OS module C runtime
// Provides getenv, getcwd, platform detection, file I/O, and process utilities.
// Cross-platform: Win32 API on Windows, POSIX on Linux/macOS.
// No 3rd-party dependencies — uses only OS-native APIs.

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <errno.h>

#ifdef _WIN32
  #include <windows.h>
  #include <direct.h>      /* _getcwd, _chdir, _mkdir, _rmdir */
  #include <process.h>     /* _getpid */
  #include <io.h>          /* _access */
  #include <sys/stat.h>    /* _stat, _S_IFREG, _S_IFDIR */
  /* strtok_r is strtok_s on MSVC */
  #define strtok_r strtok_s
  /* access() modes — X_OK not meaningful on Windows, use 0 (exist check) */
  #ifndef F_OK
    #define F_OK 0
  #endif
  #define access _access
#else
  #include <unistd.h>
  #include <sys/utsname.h>
  #include <sys/stat.h>
  #include <dirent.h>
#endif

// ============================================================
// Environment Variables
// ============================================================

// Get environment variable, returns empty string if not set
char* __os_getenv(const char* key) {
    const char* val = getenv(key);
    if (val == NULL) return strdup("");
    return strdup(val);
}

// Set environment variable
void __os_setenv(const char* key, const char* val) {
    if (!key || !val) return;
#ifdef _WIN32
    _putenv_s(key, val);
#else
    setenv(key, val, 1);
#endif
}

// ============================================================
// Working Directory
// ============================================================

// Get current working directory
char* __os_getcwd(void) {
    char* buf = (char*)malloc(4096);
    if (!buf) return strdup("");
#ifdef _WIN32
    if (_getcwd(buf, 4096) == NULL) { free(buf); return strdup(""); }
#else
    if (getcwd(buf, 4096) == NULL) { free(buf); return strdup(""); }
#endif
    return buf;
}

// Change current working directory, returns 0 on success
int __os_chdir(const char* path) {
    if (!path) return -1;
#ifdef _WIN32
    return _chdir(path);
#else
    return chdir(path);
#endif
}

// ============================================================
// Platform Info
// ============================================================

// Get platform name ("windows", "linux", "darwin", etc.)
char* __os_platform(void) {
#if defined(_WIN32) || defined(_WIN64)
    return "windows";
#elif defined(__APPLE__)
    return "darwin";
#elif defined(__linux__)
    return "linux";
#elif defined(__FreeBSD__)
    return "freebsd";
#else
    return "unknown";
#endif
}

// Get OS display name
char* __os_name(void) {
#if defined(_WIN32) || defined(_WIN64)
    return "Windows";
#elif defined(__APPLE__)
    return "macOS";
#elif defined(__linux__)
    return "Linux";
#elif defined(__FreeBSD__)
    return "FreeBSD";
#else
    return "Unknown";
#endif
}

// Get CPU architecture
char* __os_arch(void) {
#ifdef _WIN32
    SYSTEM_INFO si;
    GetNativeSystemInfo(&si);
    switch (si.wProcessorArchitecture) {
        case PROCESSOR_ARCHITECTURE_AMD64: return strdup("x86_64");
        case PROCESSOR_ARCHITECTURE_ARM64: return strdup("arm64");
        case PROCESSOR_ARCHITECTURE_INTEL: return strdup("x86");
        default:                           return strdup("unknown");
    }
#else
    struct utsname info;
    if (uname(&info) == 0) return strdup(info.machine);
    return strdup("unknown");
#endif
}

// Get hostname
char* __os_hostname(void) {
#ifdef _WIN32
    char buf[MAX_COMPUTERNAME_LENGTH + 1];
    DWORD size = sizeof(buf);
    if (GetComputerNameA(buf, &size)) return strdup(buf);
    return strdup("unknown");
#else
    char buf[256];
    if (gethostname(buf, sizeof(buf)) == 0) return strdup(buf);
    return strdup("unknown");
#endif
}

// Get process ID
int __os_getpid(void) {
#ifdef _WIN32
    return (int)GetCurrentProcessId();
#else
    return (int)getpid();
#endif
}

// Get number of CPUs/cores
int __os_cpu_count(void) {
#ifdef _WIN32
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    return (int)si.dwNumberOfProcessors;
#elif defined(_SC_NPROCESSORS_ONLN)
    long n = sysconf(_SC_NPROCESSORS_ONLN);
    return (n > 0) ? (int)n : 1;
#else
    return 1;
#endif
}

// Run a shell command, returns exit code
int __os_system(const char* cmd) {
    if (!cmd) return -1;
    return system(cmd);
}

// Exit with status code
void __os_exit(int code) {
    exit(code);
}

// ============================================================
// File I/O — uses ANSI C stdio/stat, works on all platforms
// ============================================================

/* Forward declare DesiList (defined in list.c) */
typedef struct {
    void** data;
    size_t length;
    size_t capacity;
    int type_tag;
    char* (*to_str_fn)(void*);
} DesiList;
extern DesiList* list_new(int type_tag, char* (*to_str_fn)(void*));
extern void list_append(DesiList* list, void* item, int type_tag);

// Read entire file as string. Returns empty string on error.
char* __os_read_file(const char* path) {
    if (!path) return strdup("");
    FILE* f = fopen(path, "rb");
    if (!f) return strdup("");
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (size < 0) { fclose(f); return strdup(""); }
    char* buf = (char*)malloc((size_t)size + 1);
    if (!buf) { fclose(f); return strdup(""); }
    size_t nread = fread(buf, 1, (size_t)size, f);
    fclose(f);
    buf[nread] = '\0';
    return buf;
}

// Write string to file (creates or overwrites). Returns 0 on success.
int __os_write_file(const char* path, const char* data) {
    if (!path || !data) return -1;
    FILE* f = fopen(path, "wb");
    if (!f) return -1;
    size_t len = strlen(data);
    size_t written = fwrite(data, 1, len, f);
    fclose(f);
    return (written == len) ? 0 : -1;
}

// Append string to file (creates if needed). Returns 0 on success.
int __os_append_file(const char* path, const char* data) {
    if (!path || !data) return -1;
    FILE* f = fopen(path, "ab");
    if (!f) return -1;
    size_t len = strlen(data);
    size_t written = fwrite(data, 1, len, f);
    fclose(f);
    return (written == len) ? 0 : -1;
}

// ============================================================
// File/Directory Metadata
// ============================================================

// Check if path exists (file or directory)
int __os_exists(const char* path) {
    if (!path) return 0;
#ifdef _WIN32
    return (GetFileAttributesA(path) != INVALID_FILE_ATTRIBUTES) ? 1 : 0;
#else
    struct stat st;
    return stat(path, &st) == 0;
#endif
}

// Check if path is a regular file
int __os_is_file(const char* path) {
    if (!path) return 0;
#ifdef _WIN32
    DWORD attr = GetFileAttributesA(path);
    if (attr == INVALID_FILE_ATTRIBUTES) return 0;
    return (attr & FILE_ATTRIBUTE_DIRECTORY) ? 0 : 1;
#else
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISREG(st.st_mode) ? 1 : 0;
#endif
}

// Check if path is a directory
int __os_is_dir(const char* path) {
    if (!path) return 0;
#ifdef _WIN32
    DWORD attr = GetFileAttributesA(path);
    if (attr == INVALID_FILE_ATTRIBUTES) return 0;
    return (attr & FILE_ATTRIBUTE_DIRECTORY) ? 1 : 0;
#else
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISDIR(st.st_mode) ? 1 : 0;
#endif
}

// Get file size in bytes, returns -1 on error
long __os_file_size(const char* path) {
    if (!path) return -1;
#ifdef _WIN32
    WIN32_FILE_ATTRIBUTE_DATA info;
    if (!GetFileAttributesExA(path, GetFileExInfoStandard, &info)) return -1;
    LARGE_INTEGER sz;
    sz.HighPart = (LONG)info.nFileSizeHigh;
    sz.LowPart  = info.nFileSizeLow;
    return (long)sz.QuadPart;
#else
    struct stat st;
    if (stat(path, &st) != 0) return -1;
    return (long)st.st_size;
#endif
}

// Create directory. Returns 0 on success.
int __os_mkdir(const char* path) {
    if (!path) return -1;
#ifdef _WIN32
    return CreateDirectoryA(path, NULL) ? 0 : -1;
#else
    return mkdir(path, 0755);
#endif
}

// Remove directory. Returns 0 on success.
int __os_rmdir(const char* path) {
    if (!path) return -1;
#ifdef _WIN32
    return RemoveDirectoryA(path) ? 0 : -1;
#else
    return rmdir(path);
#endif
}

// Remove file. Returns 0 on success.
int __os_remove(const char* path) {
    if (!path) return -1;
    return remove(path);
}

// Rename file or directory. Returns 0 on success.
int __os_rename(const char* old_path, const char* new_path) {
    if (!old_path || !new_path) return -1;
    return rename(old_path, new_path);
}

// List directory contents (excludes . and ..)
DesiList* __os_listdir(const char* path) {
    DesiList* result = list_new(1, NULL);
    if (!path) return result;

#ifdef _WIN32
    /* Windows: use FindFirstFile / FindNextFile */
    char pattern[4096];
    snprintf(pattern, sizeof(pattern), "%s\\*", path);
    WIN32_FIND_DATAA fd;
    HANDLE h = FindFirstFileA(pattern, &fd);
    if (h == INVALID_HANDLE_VALUE) return result;
    do {
        if (strcmp(fd.cFileName, ".") == 0 || strcmp(fd.cFileName, "..") == 0)
            continue;
        list_append(result, (void*)strdup(fd.cFileName), 1);
    } while (FindNextFileA(h, &fd));
    FindClose(h);
#else
    DIR* dir = opendir(path);
    if (!dir) return result;
    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
            continue;
        list_append(result, (void*)strdup(entry->d_name), 1);
    }
    closedir(dir);
#endif
    return result;
}

// ============================================================
// Path Utilities
// ============================================================

// Get user's home directory
char* __os_home_dir(void) {
#ifdef _WIN32
    const char* home = getenv("USERPROFILE");
    if (home) return strdup(home);
    const char* drive = getenv("HOMEDRIVE");
    const char* path  = getenv("HOMEPATH");
    if (drive && path) {
        char* result = (char*)malloc(strlen(drive) + strlen(path) + 1);
        if (result) { strcpy(result, drive); strcat(result, path); return result; }
    }
    return strdup("C:\\");
#else
    const char* home = getenv("HOME");
    return home ? strdup(home) : strdup("");
#endif
}

// Get system temp directory
char* __os_temp_dir(void) {
#ifdef _WIN32
    char buf[MAX_PATH];
    DWORD len = GetTempPathA(sizeof(buf), buf);
    if (len > 0) return strdup(buf);
    return strdup("C:\\Temp\\");
#else
    const char* tmp = getenv("TMPDIR");
    if (!tmp) tmp = "/tmp";
    return strdup(tmp);
#endif
}

// Find executable in PATH (like `which` on Unix / `where` on Windows)
char* __os_which(const char* name) {
    if (!name || !*name) return strdup("");
    const char* path_env = getenv("PATH");
    if (!path_env) return strdup("");

    char* path_copy = strdup(path_env);
    if (!path_copy) return strdup("");

#ifdef _WIN32
    const char* sep = ";";
    /* On Windows, try name as-is and with .exe extension */
    const char* exts[] = { "", ".exe", ".cmd", ".bat", NULL };
#else
    const char* sep = ":";
#endif

    char* saveptr = NULL;
    char* dir = strtok_r(path_copy, sep, &saveptr);
    while (dir) {
        size_t dlen = strlen(dir);
        size_t nlen = strlen(name);
#ifdef _WIN32
        for (int ei = 0; exts[ei] != NULL; ei++) {
            size_t elen = strlen(exts[ei]);
            char* full = (char*)malloc(dlen + nlen + elen + 3);
            if (full) {
                snprintf(full, dlen + nlen + elen + 3, "%s\\%s%s", dir, name, exts[ei]);
                if (_access(full, F_OK) == 0) {
                    free(path_copy);
                    return full;
                }
                free(full);
            }
        }
#else
        char* full = (char*)malloc(dlen + nlen + 2);
        if (full) {
            snprintf(full, dlen + nlen + 2, "%s/%s", dir, name);
            if (access(full, X_OK) == 0) {
                free(path_copy);
                return full;
            }
            free(full);
        }
#endif
        dir = strtok_r(NULL, sep, &saveptr);
    }
    free(path_copy);
    return strdup("");
}

// Sleep for given milliseconds
void __os_sleep_ms(int ms) {
    if (ms <= 0) return;
#ifdef _WIN32
    Sleep((DWORD)ms);
#else
    usleep((useconds_t)ms * 1000);
#endif
}
