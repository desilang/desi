// os.c - OS module C runtime
// Provides getenv, getcwd, platform detection, and exit
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

// Get environment variable, returns empty string if not set
char* __os_getenv(const char* key) {
    const char* val = getenv(key);
    if (val == NULL) return "";
    char* result = (char*)malloc(strlen(val) + 1);
    if (!result) return "";
    strcpy(result, val);
    return result;
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

// Exit with status code
void __os_exit(int code) {
    exit(code);
}
