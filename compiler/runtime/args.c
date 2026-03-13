/*
 * args.c — CLI argument parsing for Desi stdlib
 *
 * Simple, Go/Python-inspired flag parsing:
 *   - Positional arguments
 *   - Named flags: --name value, --flag (bool)
 *   - Short flags: -n value, -f (bool)
 *   - Auto-generated help
 *
 * Design: All args are stored in a global list at startup.
 * The parser scans for flags and positional args.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "list.h"

// Store argc/argv globally (set by main before user code)
static int g_argc = 0;
static char** g_argv = NULL;

// Called by runtime entry to save args
void __args_init(int argc, char** argv) {
    g_argc = argc;
    g_argv = argv;
}

// Get argument count (excluding program name)
int32_t __args_count(void) {
    return g_argc > 1 ? g_argc - 1 : 0;
}

// Get program name
char* __args_program(void) {
    if (g_argc > 0 && g_argv && g_argv[0]) return strdup(g_argv[0]);
    return strdup("");
}

// Get all positional arguments (non-flag args) as list[str]
DesiList* __args_positional(void) {
    DesiList* result = list_new(1, NULL);
    for (int i = 1; i < g_argc; i++) {
        if (g_argv[i][0] == '-') {
            // Skip flag and its value (if not a bool flag)
            if (i + 1 < g_argc && g_argv[i+1][0] != '-') i++;
            continue;
        }
        list_append(result, strdup(g_argv[i]), 1);
    }
    return result;
}

// Get all arguments as list[str]
DesiList* __args_all(void) {
    DesiList* result = list_new(1, NULL);
    for (int i = 1; i < g_argc; i++) {
        list_append(result, strdup(g_argv[i]), 1);
    }
    return result;
}

// Get a specific positional argument by index (0-based, after program name)
char* __args_get(int32_t index) {
    int pos = index + 1; // skip program name
    if (pos >= 0 && pos < g_argc && g_argv[pos]) return strdup(g_argv[pos]);
    return strdup("");
}

// Check if a flag exists: --name or -n
int32_t __args_has_flag(const char* name) {
    if (!name) return 0;
    for (int i = 1; i < g_argc; i++) {
        if (strcmp(g_argv[i], name) == 0) return 1;
        // Check --name=value format
        size_t nlen = strlen(name);
        if (strncmp(g_argv[i], name, nlen) == 0 && g_argv[i][nlen] == '=') return 1;
    }
    return 0;
}

// Get flag value: --name value or --name=value, returns default if not found
char* __args_get_flag(const char* name, const char* default_val) {
    if (!name) return strdup(default_val ? default_val : "");
    size_t nlen = strlen(name);

    for (int i = 1; i < g_argc; i++) {
        // Check --name=value format
        if (strncmp(g_argv[i], name, nlen) == 0 && g_argv[i][nlen] == '=') {
            return strdup(g_argv[i] + nlen + 1);
        }
        // Check --name value format
        if (strcmp(g_argv[i], name) == 0 && i + 1 < g_argc) {
            return strdup(g_argv[i + 1]);
        }
    }
    return strdup(default_val ? default_val : "");
}

// Get flag as integer, returns default if not found
int32_t __args_get_flag_int(const char* name, int32_t default_val) {
    char* val = __args_get_flag(name, NULL);
    if (val && strlen(val) > 0) {
        int result = atoi(val);
        free(val);
        return result;
    }
    if (val) free(val);
    return default_val;
}

// Check if a bool flag is set (present without value)
int32_t __args_get_flag_bool(const char* name) {
    return __args_has_flag(name);
}
