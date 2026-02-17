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
