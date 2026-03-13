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

// ============================================================
// Flag Definitions & Auto-Help
// ============================================================

typedef struct {
    char long_name[64];   // --name
    char short_name[8];   // -n
    char description[256];
    char default_val[256];
    int is_bool;          // 1 for boolean flags (no value)
    int is_required;
} FlagDef;

#define MAX_FLAGS 64
static FlagDef g_flags[MAX_FLAGS];
static int g_flag_count = 0;
static char g_program_desc[512] = "";

// Set program description for help output
int32_t __args_set_description(const char* desc) {
    if (desc) {
        strncpy(g_program_desc, desc, sizeof(g_program_desc) - 1);
        g_program_desc[sizeof(g_program_desc) - 1] = '\0';
    }
    return 0;
}

// Define a flag: args.define("--name", "-n", "User name", "World")
int32_t __args_define(const char* long_name, const char* short_name,
                   const char* description, const char* default_val) {
    if (g_flag_count >= MAX_FLAGS) return 0;
    FlagDef* f = &g_flags[g_flag_count++];
    strncpy(f->long_name, long_name ? long_name : "", sizeof(f->long_name) - 1);
    strncpy(f->short_name, short_name ? short_name : "", sizeof(f->short_name) - 1);
    strncpy(f->description, description ? description : "", sizeof(f->description) - 1);
    strncpy(f->default_val, default_val ? default_val : "", sizeof(f->default_val) - 1);
    f->is_bool = (default_val == NULL || strlen(default_val) == 0) ? 1 : 0;
    f->is_required = 0;
    return 0;
}

// Define a boolean flag (no value expected)
int32_t __args_define_bool(const char* long_name, const char* short_name,
                        const char* description) {
    if (g_flag_count >= MAX_FLAGS) return 0;
    FlagDef* f = &g_flags[g_flag_count++];
    strncpy(f->long_name, long_name ? long_name : "", sizeof(f->long_name) - 1);
    strncpy(f->short_name, short_name ? short_name : "", sizeof(f->short_name) - 1);
    strncpy(f->description, description ? description : "", sizeof(f->description) - 1);
    f->default_val[0] = '\0';
    f->is_bool = 1;
    f->is_required = 0;
    return 0;
}

// Mark a flag as required
int32_t __args_require(const char* flag_name) {
    for (int i = 0; i < g_flag_count; i++) {
        if (strcmp(g_flags[i].long_name, flag_name) == 0 ||
            strcmp(g_flags[i].short_name, flag_name) == 0) {
            g_flags[i].is_required = 1;
            return 0;
        }
    }
    return 0;
}

// Print auto-generated help
int32_t __args_print_help(void) {
    const char* prog = (g_argc > 0 && g_argv && g_argv[0]) ? g_argv[0] : "program";

    if (strlen(g_program_desc) > 0) {
        fprintf(stdout, "%s\n\n", g_program_desc);
    }
    fprintf(stdout, "Usage: %s [options]\n\n", prog);
    fprintf(stdout, "Options:\n");

    for (int i = 0; i < g_flag_count; i++) {
        FlagDef* f = &g_flags[i];
        if (strlen(f->short_name) > 0) {
            fprintf(stdout, "  %s, %-12s %s", f->short_name, f->long_name, f->description);
        } else {
            fprintf(stdout, "  %-16s %s", f->long_name, f->description);
        }
        if (!f->is_bool && strlen(f->default_val) > 0) {
            fprintf(stdout, " (default: %s)", f->default_val);
        }
        if (f->is_required) {
            fprintf(stdout, " [required]");
        }
        fprintf(stdout, "\n");
    }
    fprintf(stdout, "  -h, --help         Show this help message\n");
    return 0;
}

// Check --help flag and print help if present, returns 1 if help was shown
int32_t __args_check_help(void) {
    if (__args_has_flag("--help") || __args_has_flag("-h")) {
        __args_print_help();
        return 1;
    }
    return 0;
}

// Validate that all required flags are present
// Returns empty string if ok, error message if missing
char* __args_validate(void) {
    for (int i = 0; i < g_flag_count; i++) {
        if (g_flags[i].is_required) {
            if (!__args_has_flag(g_flags[i].long_name) &&
                (strlen(g_flags[i].short_name) == 0 ||
                 !__args_has_flag(g_flags[i].short_name))) {
                char buf[512];
                snprintf(buf, sizeof(buf), "missing required flag: %s", g_flags[i].long_name);
                return strdup(buf);
            }
        }
    }
    return strdup("");
}

// Get flag value using defined flags (checks both long and short names)
char* __args_get_defined(const char* long_name) {
    // Find definition
    FlagDef* def = NULL;
    for (int i = 0; i < g_flag_count; i++) {
        if (strcmp(g_flags[i].long_name, long_name) == 0) {
            def = &g_flags[i];
            break;
        }
    }

    // Check long name
    char* val = __args_get_flag(long_name, NULL);
    if (val && strlen(val) > 0) return val;
    if (val) free(val);

    // Check short name
    if (def && strlen(def->short_name) > 0) {
        val = __args_get_flag(def->short_name, NULL);
        if (val && strlen(val) > 0) return val;
        if (val) free(val);
    }

    // Return default
    if (def) return strdup(def->default_val);
    return strdup("");
}

// ============================================================
// Subcommands
// ============================================================

// Get subcommand (first positional argument that doesn't start with -)
char* __args_subcommand(void) {
    for (int i = 1; i < g_argc; i++) {
        if (g_argv[i][0] != '-') {
            return strdup(g_argv[i]);
        }
    }
    return strdup("");
}

// Get arguments after the subcommand
DesiList* __args_subcommand_args(void) {
    DesiList* result = list_new(1, NULL);
    int found_sub = 0;
    for (int i = 1; i < g_argc; i++) {
        if (!found_sub && g_argv[i][0] != '-') {
            found_sub = 1;
            continue; // skip subcommand itself
        }
        if (found_sub) {
            list_append(result, strdup(g_argv[i]), 1);
        }
    }
    return result;
}
