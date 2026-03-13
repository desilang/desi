/*
 * toml.c — TOML config file parsing for Desi stdlib
 *
 * Simple TOML parser supporting:
 *   - Key-value pairs (strings, integers, booleans)
 *   - Sections [section]
 *   - Comments (#)
 *   - Quoted and unquoted string values
 *
 * Design: Parse into flat key-value store with dotted keys.
 * [server] + port = 8080 => "server.port" = "8080"
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// Key-value entry
typedef struct {
    char key[256];
    char value[1024];
} TomlEntry;

#define MAX_TOML_ENTRIES 256

typedef struct {
    TomlEntry entries[MAX_TOML_ENTRIES];
    int count;
} TomlDoc;

// Global document for simple API
static TomlDoc g_toml = {0};

// Trim whitespace in-place
static char* trim(char* s) {
    while (*s == ' ' || *s == '\t') s++;
    int len = strlen(s);
    while (len > 0 && (s[len-1] == ' ' || s[len-1] == '\t' || s[len-1] == '\n' || s[len-1] == '\r')) {
        s[--len] = '\0';
    }
    return s;
}

// Parse TOML string into global doc
int32_t __toml_parse(const char* input) {
    if (!input) return -1;
    
    g_toml.count = 0;
    char section[128] = "";
    
    // Work on a copy
    char* data = strdup(input);
    char* line = strtok(data, "\n");
    
    while (line) {
        char* trimmed = trim(line);
        
        // Skip empty lines and comments
        if (trimmed[0] == '\0' || trimmed[0] == '#') {
            line = strtok(NULL, "\n");
            continue;
        }
        
        // Section header
        if (trimmed[0] == '[') {
            char* end = strchr(trimmed, ']');
            if (end) {
                *end = '\0';
                strncpy(section, trimmed + 1, sizeof(section) - 1);
                trim(section);
            }
            line = strtok(NULL, "\n");
            continue;
        }
        
        // Key = value
        char* eq = strchr(trimmed, '=');
        if (eq && g_toml.count < MAX_TOML_ENTRIES) {
            *eq = '\0';
            char* key = trim(trimmed);
            char* val = trim(eq + 1);
            
            TomlEntry* entry = &g_toml.entries[g_toml.count++];
            
            // Build dotted key
            if (strlen(section) > 0) {
                snprintf(entry->key, sizeof(entry->key), "%s.%s", section, key);
            } else {
                strncpy(entry->key, key, sizeof(entry->key) - 1);
            }
            
            // Strip quotes from value
            int vlen = strlen(val);
            if (vlen >= 2 && ((val[0] == '"' && val[vlen-1] == '"') ||
                              (val[0] == '\'' && val[vlen-1] == '\''))) {
                val[vlen-1] = '\0';
                val++;
            }
            strncpy(entry->value, val, sizeof(entry->value) - 1);
        }
        
        line = strtok(NULL, "\n");
    }
    
    free(data);
    return g_toml.count;
}

// Parse TOML file
int32_t __toml_parse_file(const char* path) {
    if (!path) return -1;
    
    FILE* f = fopen(path, "r");
    if (!f) return -1;
    
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    
    char* data = malloc(size + 1);
    fread(data, 1, size, f);
    data[size] = '\0';
    fclose(f);
    
    int result = __toml_parse(data);
    free(data);
    return result;
}

// Get string value by dotted key (e.g., "server.port")
char* __toml_get(const char* key) {
    if (!key) return strdup("");
    for (int i = 0; i < g_toml.count; i++) {
        if (strcmp(g_toml.entries[i].key, key) == 0) {
            return strdup(g_toml.entries[i].value);
        }
    }
    return strdup("");
}

// Get with default value
char* __toml_get_default(const char* key, const char* default_val) {
    if (!key) return strdup(default_val ? default_val : "");
    for (int i = 0; i < g_toml.count; i++) {
        if (strcmp(g_toml.entries[i].key, key) == 0) {
            return strdup(g_toml.entries[i].value);
        }
    }
    return strdup(default_val ? default_val : "");
}

// Get as integer
int32_t __toml_get_int(const char* key, int32_t default_val) {
    char* val = __toml_get(key);
    if (val && strlen(val) > 0) {
        int result = atoi(val);
        free(val);
        return result;
    }
    if (val) free(val);
    return default_val;
}

// Get as bool (true/false/yes/no/1/0)
int32_t __toml_get_bool(const char* key, int32_t default_val) {
    char* val = __toml_get(key);
    if (val && strlen(val) > 0) {
        int result = default_val;
        if (strcmp(val, "true") == 0 || strcmp(val, "yes") == 0 || strcmp(val, "1") == 0) {
            result = 1;
        } else if (strcmp(val, "false") == 0 || strcmp(val, "no") == 0 || strcmp(val, "0") == 0) {
            result = 0;
        }
        free(val);
        return result;
    }
    if (val) free(val);
    return default_val;
}

// Check if key exists
int32_t __toml_has_key(const char* key) {
    if (!key) return 0;
    for (int i = 0; i < g_toml.count; i++) {
        if (strcmp(g_toml.entries[i].key, key) == 0) return 1;
    }
    return 0;
}

// Get number of entries
int32_t __toml_count(void) {
    return g_toml.count;
}
