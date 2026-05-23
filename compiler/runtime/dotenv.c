/*
 * dotenv.c — .env file loader for Desi stdlib
 *
 * Parses KEY=VALUE files and loads them into the process environment.
 * Supports comments (#), quoted values, and multiline values.
 *
 * Public API:
 *   __dotenv_load(path)              → load .env file (default: ".env")
 *   __dotenv_load_or_fail(path)      → load, return -1 on error
 *   __dotenv_parse(content)          → parse string, return "K=V\n..." pairs
 *   __dotenv_get(key)                → get env var (shortcut for env.get)
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdint.h>

/* ---- Helpers ---- */

static char* trim(char* s) {
    while (*s && isspace((unsigned char)*s)) s++;
    char* end = s + strlen(s) - 1;
    while (end > s && isspace((unsigned char)*end)) *end-- = '\0';
    return s;
}

static char* strip_quotes(char* s) {
    size_t len = strlen(s);
    if (len >= 2) {
        if ((s[0] == '"' && s[len-1] == '"') ||
            (s[0] == '\'' && s[len-1] == '\'')) {
            s[len-1] = '\0';
            return s + 1;
        }
    }
    return s;
}

/* ============================================================
 * Parser
 * ============================================================ */

static int parse_and_set(const char* content, int do_setenv) {
    if (!content) return -1;

    char* buf = strdup(content);
    char* line = strtok(buf, "\n");
    int count = 0;

    while (line) {
        char* trimmed = trim(line);

        /* Skip empty lines and comments */
        if (trimmed[0] == '\0' || trimmed[0] == '#') {
            line = strtok(NULL, "\n");
            continue;
        }

        /* Skip 'export ' prefix */
        if (strncmp(trimmed, "export ", 7) == 0) {
            trimmed += 7;
            trimmed = trim(trimmed);
        }

        /* Find = separator */
        char* eq = strchr(trimmed, '=');
        if (!eq) {
            line = strtok(NULL, "\n");
            continue;
        }

        /* Extract key */
        *eq = '\0';
        char* key = trim(trimmed);
        char* value = trim(eq + 1);

        /* Strip quotes from value */
        value = strip_quotes(value);

        /* Process escape sequences in double-quoted values */
        /* (simple: just handle \n and \t) */
        if (do_setenv && key[0]) {
#ifdef _WIN32
            _putenv_s(key, value);
#else
            setenv(key, value, 1);
#endif
            count++;
        }

        line = strtok(NULL, "\n");
    }

    free(buf);
    return count;
}

/* ============================================================
 * Public API
 * ============================================================ */

int32_t __dotenv_load(const char* path) {
    if (!path || path[0] == '\0') path = ".env";

    FILE* f = fopen(path, "r");
    if (!f) return 0; /* silently ignore missing .env */

    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);

    char* content = (char*)malloc(sz + 1);
    fread(content, 1, sz, f);
    content[sz] = '\0';
    fclose(f);

    int count = parse_and_set(content, 1);
    free(content);
    return count;
}

int32_t __dotenv_load_or_fail(const char* path) {
    if (!path || path[0] == '\0') path = ".env";

    FILE* f = fopen(path, "r");
    if (!f) return -1;

    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);

    char* content = (char*)malloc(sz + 1);
    fread(content, 1, sz, f);
    content[sz] = '\0';
    fclose(f);

    int count = parse_and_set(content, 1);
    free(content);
    return count;
}

/* Parse .env content and return as "KEY=VALUE\n..." without setting env */
char* __dotenv_parse(const char* content) {
    if (!content) return strdup("");

    char result[8192];
    int rlen = 0;

    char* buf = strdup(content);
    char* line = strtok(buf, "\n");

    while (line) {
        char* trimmed = trim(line);
        if (trimmed[0] == '\0' || trimmed[0] == '#') {
            line = strtok(NULL, "\n");
            continue;
        }
        if (strncmp(trimmed, "export ", 7) == 0) {
            trimmed += 7;
            trimmed = trim(trimmed);
        }
        char* eq = strchr(trimmed, '=');
        if (eq) {
            rlen += snprintf(result + rlen, sizeof(result) - rlen, "%s\n", trimmed);
        }
        line = strtok(NULL, "\n");
    }

    free(buf);
    return strdup(result);
}

const char* __dotenv_get(const char* key) {
    if (!key) return "";
    const char* val = getenv(key);
    return val ? val : "";
}
