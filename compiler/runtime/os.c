// os.c - OS module C runtime
// Provides getenv, getcwd, platform detection, and exit
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/utsname.h>

// Get environment variable, returns empty string if not set
char* __os_getenv(const char* key) {
    const char* val = getenv(key);
    if (val == NULL) return "";
    char* result = (char*)malloc(strlen(val) + 1);
    if (!result) return "";
    strcpy(result, val);
    return result;
}

// Set environment variable (1 = overwrite)
void __os_setenv(const char* key, const char* val) {
    if (key && val) setenv(key, val, 1);
}

// Get current working directory
char* __os_getcwd(void) {
    char* buf = (char*)malloc(4096);
    if (!buf) return "";
    if (getcwd(buf, 4096) == NULL) {
        free(buf);
        return "";
    }
    return buf;
}

// Change current working directory, returns 0 on success
int __os_chdir(const char* path) {
    if (!path) return -1;
    return chdir(path);
}

// Get platform name
char* __os_platform(void) {
#if defined(__APPLE__)
    return "darwin";
#elif defined(__linux__)
    return "linux";
#elif defined(_WIN32) || defined(_WIN64)
    return "windows";
#else
    return "unknown";
#endif
}

// Get OS name (more descriptive than platform)
char* __os_name(void) {
#if defined(__APPLE__)
    return "macOS";
#elif defined(__linux__)
    return "Linux";
#elif defined(_WIN32) || defined(_WIN64)
    return "Windows";
#elif defined(__FreeBSD__)
    return "FreeBSD";
#else
    return "Unknown";
#endif
}

// Get CPU architecture
char* __os_arch(void) {
    struct utsname info;
    if (uname(&info) == 0)
        return strdup(info.machine);
    return "unknown";
}

// Get hostname
char* __os_hostname(void) {
    char buf[256];
    if (gethostname(buf, sizeof(buf)) == 0)
        return strdup(buf);
    return strdup("unknown");
}

// Get process ID
int __os_getpid(void) {
    return (int)getpid();
}

// Get number of CPUs/cores
int __os_cpu_count(void) {
#if defined(_SC_NPROCESSORS_ONLN)
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
// File I/O
// ============================================================

#include <sys/stat.h>
#include <dirent.h>
#include <errno.h>

// Forward declare DesiList from list.h
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

// Check if path exists (file or directory)
int __os_exists(const char* path) {
    if (!path) return 0;
    struct stat st;
    return stat(path, &st) == 0;
}

// Check if path is a regular file
int __os_is_file(const char* path) {
    if (!path) return 0;
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISREG(st.st_mode);
}

// Check if path is a directory
int __os_is_dir(const char* path) {
    if (!path) return 0;
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISDIR(st.st_mode);
}

// Get file size in bytes, returns -1 on error
long __os_file_size(const char* path) {
    if (!path) return -1;
    struct stat st;
    if (stat(path, &st) != 0) return -1;
    return (long)st.st_size;
}

// Create directory. Returns 0 on success.
int __os_mkdir(const char* path) {
    if (!path) return -1;
#ifdef _WIN32
    return _mkdir(path);
#else
    return mkdir(path, 0755);
#endif
}

// Remove directory. Returns 0 on success.
int __os_rmdir(const char* path) {
    if (!path) return -1;
    return rmdir(path);
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

// List directory contents, returns list of filenames (excludes . and ..)
DesiList* __os_listdir(const char* path) {
    DesiList* result = list_new(1, NULL); // type_tag 1 = str
    if (!path) return result;
    DIR* dir = opendir(path);
    if (!dir) return result;
    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
            continue;
        list_append(result, (void*)strdup(entry->d_name), 1);
    }
    closedir(dir);
    return result;
}

// ============================================================
// Path Utilities (highly requested, cross-platform)
// ============================================================

// Get user's home directory
char* __os_home_dir(void) {
#ifdef _WIN32
    const char* home = getenv("USERPROFILE");
    if (!home) {
        const char* drive = getenv("HOMEDRIVE");
        const char* path = getenv("HOMEPATH");
        if (drive && path) {
            char* result = (char*)malloc(strlen(drive) + strlen(path) + 1);
            if (result) { strcpy(result, drive); strcat(result, path); return result; }
        }
    }
#else
    const char* home = getenv("HOME");
#endif
    return home ? strdup(home) : strdup("");
}

// Get system temp directory
char* __os_temp_dir(void) {
#ifdef _WIN32
    const char* tmp = getenv("TEMP");
    if (!tmp) tmp = getenv("TMP");
    if (!tmp) tmp = "C:\\Temp";
    return strdup(tmp);
#else
    const char* tmp = getenv("TMPDIR");
    if (!tmp) tmp = "/tmp";
    return strdup(tmp);
#endif
}

// Find executable in PATH (like `which` command)
char* __os_which(const char* name) {
    if (!name || !*name) return strdup("");
    const char* path_env = getenv("PATH");
    if (!path_env) return strdup("");
    
    char* path_copy = strdup(path_env);
    if (!path_copy) return strdup("");
    
#ifdef _WIN32
    const char* sep = ";";
#else
    const char* sep = ":";
#endif
    
    char* saveptr = NULL;
    char* dir = strtok_r(path_copy, sep, &saveptr);
    while (dir) {
        size_t dlen = strlen(dir);
        size_t nlen = strlen(name);
        char* full = (char*)malloc(dlen + nlen + 2);
        if (full) {
            sprintf(full, "%s/%s", dir, name);
            if (access(full, X_OK) == 0) {
                free(path_copy);
                return full;
            }
            free(full);
        }
        dir = strtok_r(NULL, sep, &saveptr);
    }
    free(path_copy);
    return strdup("");
}

// Sleep for given milliseconds
void __os_sleep_ms(int ms) {
    if (ms <= 0) return;
#ifdef _WIN32
    extern void Sleep(unsigned long dwMilliseconds);
    Sleep((unsigned long)ms);
#else
    usleep((useconds_t)ms * 1000);
#endif
}
