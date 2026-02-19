// env.c - Environment file loading module
// Inspired by Node.js dotenv. Desi exclusive.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

// ============================================================
// .env file loading
// ============================================================

// Load .env file: parses KEY=VALUE lines, sets env vars
int __env_load(const char* path) {
    if (!path) return 0;
    FILE* f = fopen(path, "r");
    if (!f) return 0;

    char line[4096];
    int count = 0;
    while (fgets(line, sizeof(line), f)) {
        // Strip newline
        size_t len = strlen(line);
        while (len > 0 && (line[len-1] == '\n' || line[len-1] == '\r'))
            line[--len] = '\0';

        // Skip empty lines and comments
        char* p = line;
        while (*p && isspace((unsigned char)*p)) p++;
        if (*p == '\0' || *p == '#') continue;

        // Find KEY=VALUE
        char* eq = strchr(p, '=');
        if (!eq) continue;

        // Extract key (trim trailing spaces)
        size_t klen = eq - p;
        while (klen > 0 && isspace((unsigned char)p[klen-1])) klen--;
        char* key = (char*)malloc(klen + 1);
        if (!key) continue;
        memcpy(key, p, klen);
        key[klen] = '\0';

        // Extract value (trim leading spaces and quotes)
        char* val_start = eq + 1;
        while (*val_start && isspace((unsigned char)*val_start)) val_start++;

        // Remove surrounding quotes
        size_t vlen = strlen(val_start);
        if (vlen >= 2) {
            if ((val_start[0] == '"' && val_start[vlen-1] == '"') ||
                (val_start[0] == '\'' && val_start[vlen-1] == '\'')) {
                val_start++;
                vlen -= 2;
            }
        }

        char* val = (char*)malloc(vlen + 1);
        if (!val) { free(key); continue; }
        memcpy(val, val_start, vlen);
        val[vlen] = '\0';

        setenv(key, val, 0); // Don't overwrite existing
        count++;
        free(key);
        free(val);
    }
    fclose(f);
    return count > 0 ? 1 : 0;
}

// Load .env from current working directory
int __env_load_default(void) {
    return __env_load(".env");
}

// Get env var with default fallback
char* __env_get(const char* key, const char* default_val) {
    if (!key) return strdup(default_val ? default_val : "");
    const char* val = getenv(key);
    if (val) return strdup(val);
    return strdup(default_val ? default_val : "");
}

// Get env var, panic if missing
char* __env_require(const char* key) {
    if (!key) {
        fprintf(stderr, "env.require: key is null\n");
        exit(1);
    }
    const char* val = getenv(key);
    if (!val) {
        fprintf(stderr, "env.require: '%s' is not set\n", key);
        exit(1);
    }
    return strdup(val);
}

// Check if env var exists
int __env_is_set(const char* key) {
    if (!key) return 0;
    return getenv(key) != NULL;
}
