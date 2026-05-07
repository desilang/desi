/*
 * ini.c — INI file parser for Desi stdlib
 *
 * Parses standard INI files with sections, key=value pairs, and comments.
 *
 * Public API:
 *   __ini_parse(content)            → parse INI string into opaque handle
 *   __ini_parse_file(path)          → parse INI file
 *   __ini_get(ini, section, key)    → get value
 *   __ini_get_default(ini, section, key, default) → get with fallback
 *   __ini_has_section(ini, section) → check if section exists
 *   __ini_has_key(ini, section, key)→ check if key exists
 *   __ini_sections(ini)             → list sections as "\n"-separated string
 *   __ini_keys(ini, section)        → list keys in section
 *   __ini_free(ini)                 → destroy
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdint.h>

/* ---- Internal types ---- */

typedef struct IniEntry {
    char* key;
    char* value;
    struct IniEntry* next;
} IniEntry;

typedef struct IniSection {
    char* name;
    IniEntry* entries;
    struct IniSection* next;
} IniSection;

typedef struct {
    IniSection* sections;
} IniFile;

/* ---- Helpers ---- */

static char* ini_trim(char* s) {
    while (*s && isspace((unsigned char)*s)) s++;
    char* end = s + strlen(s) - 1;
    while (end > s && isspace((unsigned char)*end)) *end-- = '\0';
    return s;
}

static IniSection* find_section(IniFile* ini, const char* name) {
    IniSection* s = ini->sections;
    while (s) {
        if (strcmp(s->name, name) == 0) return s;
        s = s->next;
    }
    return NULL;
}

static IniSection* get_or_create_section(IniFile* ini, const char* name) {
    IniSection* s = find_section(ini, name);
    if (s) return s;
    s = (IniSection*)calloc(1, sizeof(IniSection));
    s->name = strdup(name);
    s->next = ini->sections;
    ini->sections = s;
    return s;
}

static void section_set(IniSection* sec, const char* key, const char* value) {
    /* Check for existing key */
    IniEntry* e = sec->entries;
    while (e) {
        if (strcmp(e->key, key) == 0) {
            free(e->value);
            e->value = strdup(value);
            return;
        }
        e = e->next;
    }
    /* New entry */
    e = (IniEntry*)malloc(sizeof(IniEntry));
    e->key = strdup(key);
    e->value = strdup(value);
    e->next = sec->entries;
    sec->entries = e;
}

/* ============================================================
 * Parser
 * ============================================================ */

IniFile* __ini_parse(const char* content) {
    IniFile* ini = (IniFile*)calloc(1, sizeof(IniFile));
    if (!content) return ini;

    char* buf = strdup(content);
    char* current_section = "";
    IniSection* sec = get_or_create_section(ini, "");

    char* line = strtok(buf, "\n");
    while (line) {
        char* trimmed = ini_trim(line);

        /* Skip empty lines and comments */
        if (trimmed[0] == '\0' || trimmed[0] == '#' || trimmed[0] == ';') {
            line = strtok(NULL, "\n");
            continue;
        }

        /* Section header: [section] */
        if (trimmed[0] == '[') {
            char* end = strchr(trimmed, ']');
            if (end) {
                *end = '\0';
                current_section = trimmed + 1;
                sec = get_or_create_section(ini, current_section);
            }
            line = strtok(NULL, "\n");
            continue;
        }

        /* Key = value (or key : value) */
        char* sep = strchr(trimmed, '=');
        if (!sep) sep = strchr(trimmed, ':');
        if (sep) {
            *sep = '\0';
            char* key = ini_trim(trimmed);
            char* val = ini_trim(sep + 1);

            /* Strip inline comments */
            char* comment = strchr(val, '#');
            if (!comment) comment = strchr(val, ';');
            if (comment) {
                /* Only if preceded by whitespace */
                if (comment > val && isspace((unsigned char)*(comment - 1))) {
                    *comment = '\0';
                    val = ini_trim(val);
                }
            }

            /* Strip quotes */
            size_t vlen = strlen(val);
            if (vlen >= 2 && ((val[0] == '"' && val[vlen-1] == '"') ||
                               (val[0] == '\'' && val[vlen-1] == '\''))) {
                val[vlen-1] = '\0';
                val++;
            }

            section_set(sec, key, val);
        }

        line = strtok(NULL, "\n");
    }

    free(buf);
    return ini;
}

IniFile* __ini_parse_file(const char* path) {
    if (!path) return __ini_parse("");

    FILE* f = fopen(path, "r");
    if (!f) return __ini_parse("");

    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);

    char* content = (char*)malloc(sz + 1);
    fread(content, 1, sz, f);
    content[sz] = '\0';
    fclose(f);

    IniFile* ini = __ini_parse(content);
    free(content);
    return ini;
}

/* ============================================================
 * Accessors
 * ============================================================ */

const char* __ini_get(IniFile* ini, const char* section, const char* key) {
    if (!ini || !key) return "";
    if (!section) section = "";
    IniSection* s = find_section(ini, section);
    if (!s) return "";
    IniEntry* e = s->entries;
    while (e) {
        if (strcmp(e->key, key) == 0) return e->value;
        e = e->next;
    }
    return "";
}

const char* __ini_get_default(IniFile* ini, const char* section,
                               const char* key, const char* default_val) {
    const char* v = __ini_get(ini, section, key);
    return (v && v[0]) ? v : (default_val ? default_val : "");
}

int32_t __ini_has_section(IniFile* ini, const char* section) {
    return find_section(ini, section ? section : "") ? 1 : 0;
}

int32_t __ini_has_key(IniFile* ini, const char* section, const char* key) {
    const char* v = __ini_get(ini, section, key);
    return (v && v[0]) ? 1 : 0;
}

char* __ini_sections(IniFile* ini) {
    if (!ini) return strdup("");
    char buf[4096];
    int len = 0;
    IniSection* s = ini->sections;
    while (s) {
        if (s->name[0]) /* skip default section */
            len += snprintf(buf + len, sizeof(buf) - len, "%s\n", s->name);
        s = s->next;
    }
    return strdup(buf);
}

char* __ini_keys(IniFile* ini, const char* section) {
    if (!ini) return strdup("");
    IniSection* s = find_section(ini, section ? section : "");
    if (!s) return strdup("");
    char buf[4096];
    int len = 0;
    IniEntry* e = s->entries;
    while (e) {
        len += snprintf(buf + len, sizeof(buf) - len, "%s\n", e->key);
        e = e->next;
    }
    return strdup(buf);
}

/* ============================================================
 * Cleanup
 * ============================================================ */

void __ini_free(IniFile* ini) {
    if (!ini) return;
    IniSection* s = ini->sections;
    while (s) {
        IniSection* sn = s->next;
        IniEntry* e = s->entries;
        while (e) {
            IniEntry* en = e->next;
            free(e->key);
            free(e->value);
            free(e);
            e = en;
        }
        free(s->name);
        free(s);
        s = sn;
    }
    free(ini);
}
