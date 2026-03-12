// template.c — Simple string template rendering for Desi stdlib
// Supports {{key}} placeholder substitution from a dict[str, str]
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Forward declare DesiList/DesiDict
typedef struct {
    void** data;
    size_t length;
    size_t capacity;
    int type_tag;
    char* (*to_str_fn)(void*);
} DesiList;
extern DesiList* list_new(int type_tag, char* (*to_str_fn)(void*));
extern void list_append(DesiList* list, void* item, int type_tag);

// ============================================================
// Public API
// ============================================================

// Render a template by replacing {{key}} with values from parallel lists.
// keys and values are list[str] with matching indices.
char* __template_render(const char* tmpl, DesiList* keys, DesiList* values) {
    if (!tmpl) return strdup("");
    if (!keys || !values) return strdup(tmpl);

    // Start with a copy of the template and iteratively replace
    size_t buf_cap = strlen(tmpl) * 2 + 64;
    size_t buf_len = 0;
    char* buf = (char*)malloc(buf_cap);
    buf[0] = '\0';

    const char* p = tmpl;
    while (*p) {
        // Look for {{ pattern
        if (p[0] == '{' && p[1] == '{') {
            // Find closing }}
            const char* start = p + 2;
            const char* end = strstr(start, "}}");
            if (end) {
                // Extract key name (trim whitespace)
                size_t key_len = (size_t)(end - start);
                char* key = (char*)malloc(key_len + 1);
                memcpy(key, start, key_len);
                key[key_len] = '\0';

                // Trim leading/trailing whitespace
                char* k = key;
                while (*k == ' ' || *k == '\t') k++;
                char* ke = key + key_len - 1;
                while (ke > k && (*ke == ' ' || *ke == '\t')) { *ke = '\0'; ke--; }

                // Look up key in keys list, get value from values list
                const char* replacement = NULL;
                for (size_t i = 0; i < keys->length && i < values->length; i++) {
                    const char* kname = (const char*)keys->data[i];
                    if (kname && strcmp(kname, k) == 0) {
                        replacement = (const char*)values->data[i];
                        break;
                    }
                }

                if (replacement) {
                    size_t rlen = strlen(replacement);
                    while (buf_len + rlen + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                    memcpy(buf + buf_len, replacement, rlen);
                    buf_len += rlen;
                } else {
                    // Keep original placeholder if key not found
                    size_t orig_len = (size_t)(end + 2 - p);
                    while (buf_len + orig_len + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                    memcpy(buf + buf_len, p, orig_len);
                    buf_len += orig_len;
                }
                free(key);
                p = end + 2;
                continue;
            }
        }
        // Regular character
        while (buf_len + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
        buf[buf_len++] = *p;
        p++;
    }
    buf[buf_len] = '\0';
    return buf;
}

// Render with a simple key=value format: render_kv(tmpl, "key1", "val1", "key2", "val2", ...)
// keys_values is a list[str] where even indices are keys and odd are values
char* __template_render_pairs(const char* tmpl, DesiList* pairs) {
    if (!tmpl) return strdup("");
    if (!pairs || pairs->length < 2) return strdup(tmpl);

    // Build keys/values lists from pairs
    DesiList* keys = list_new(1, NULL);
    DesiList* values = list_new(1, NULL);
    for (size_t i = 0; i + 1 < pairs->length; i += 2) {
        list_append(keys, pairs->data[i], 1);
        list_append(values, pairs->data[i + 1], 1);
    }

    char* result = __template_render(tmpl, keys, values);
    // Don't free keys/values - they reference the same string pointers as pairs
    free(keys);
    free(values);
    return result;
}

// Escape HTML special characters
char* __template_escape_html(const char* input) {
    if (!input) return strdup("");
    size_t len = strlen(input);
    // Worst case: every char expands to &amp; (5 chars)
    size_t buf_cap = len * 5 + 1;
    char* buf = (char*)malloc(buf_cap);
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        switch (input[i]) {
            case '&':  memcpy(buf + j, "&amp;", 5); j += 5; break;
            case '<':  memcpy(buf + j, "&lt;", 4); j += 4; break;
            case '>':  memcpy(buf + j, "&gt;", 4); j += 4; break;
            case '"':  memcpy(buf + j, "&quot;", 6); j += 6; break;
            case '\'': memcpy(buf + j, "&#39;", 5); j += 5; break;
            default:   buf[j++] = input[i]; break;
        }
    }
    buf[j] = '\0';
    return buf;
}
