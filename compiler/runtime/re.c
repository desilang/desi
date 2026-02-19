// re.c - Regular expression module
// Uses POSIX <regex.h> — available on macOS/Linux with no external deps.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <regex.h>
#include "list.h"

// ============================================================
// Core regex functions
// ============================================================

// Test if entire string matches pattern
int __re_match(const char* pattern, const char* s) {
    if (!pattern || !s) return 0;
    regex_t reg;
    if (regcomp(&reg, pattern, REG_EXTENDED | REG_NOSUB) != 0) return 0;

    // Match against full string by anchoring
    size_t plen = strlen(pattern);
    char* anchored = (char*)malloc(plen + 3); // ^ + pattern + $ + \0
    if (!anchored) { regfree(&reg); return 0; }
    sprintf(anchored, "^%s$", pattern);

    regex_t areg;
    int result = 0;
    if (regcomp(&areg, anchored, REG_EXTENDED | REG_NOSUB) == 0) {
        result = (regexec(&areg, s, 0, NULL, 0) == 0) ? 1 : 0;
        regfree(&areg);
    }
    free(anchored);
    regfree(&reg);
    return result;
}

// Test if pattern matches anywhere in string
int __re_is_match(const char* pattern, const char* s) {
    if (!pattern || !s) return 0;
    regex_t reg;
    if (regcomp(&reg, pattern, REG_EXTENDED | REG_NOSUB) != 0) return 0;
    int result = (regexec(&reg, s, 0, NULL, 0) == 0) ? 1 : 0;
    regfree(&reg);
    return result;
}

// Find first match, return matched substring
char* __re_search(const char* pattern, const char* s) {
    if (!pattern || !s) return strdup("");
    regex_t reg;
    if (regcomp(&reg, pattern, REG_EXTENDED) != 0) return strdup("");

    regmatch_t match;
    if (regexec(&reg, s, 1, &match, 0) == 0) {
        int len = match.rm_eo - match.rm_so;
        char* result = (char*)malloc(len + 1);
        if (result) {
            memcpy(result, s + match.rm_so, len);
            result[len] = '\0';
            regfree(&reg);
            return result;
        }
    }
    regfree(&reg);
    return strdup("");
}

// Find all non-overlapping matches
DesiList* __re_findall(const char* pattern, const char* s) {
    DesiList* result = list_new(1, NULL); // type_tag 1 = str
    if (!pattern || !s) return result;

    regex_t reg;
    if (regcomp(&reg, pattern, REG_EXTENDED) != 0) return result;

    const char* cursor = s;
    regmatch_t match;
    while (regexec(&reg, cursor, 1, &match, 0) == 0) {
        int len = match.rm_eo - match.rm_so;
        if (len == 0) {
            cursor++;
            if (*cursor == '\0') break;
            continue;
        }
        char* part = (char*)malloc(len + 1);
        if (part) {
            memcpy(part, cursor + match.rm_so, len);
            part[len] = '\0';
            list_append(result, (void*)part, 1);
        }
        cursor += match.rm_eo;
    }
    regfree(&reg);
    return result;
}

// Replace all matches of pattern with replacement
char* __re_replace(const char* pattern, const char* s, const char* repl) {
    if (!pattern || !s || !repl) return strdup(s ? s : "");

    regex_t reg;
    if (regcomp(&reg, pattern, REG_EXTENDED) != 0) return strdup(s);

    // Build result by iterating through matches
    size_t repl_len = strlen(repl);
    size_t result_cap = strlen(s) * 2 + 64;
    char* result = (char*)malloc(result_cap);
    if (!result) { regfree(&reg); return strdup(s); }

    size_t result_len = 0;
    const char* cursor = s;
    regmatch_t match;

    while (regexec(&reg, cursor, 1, &match, 0) == 0) {
        // Append text before match
        size_t before_len = match.rm_so;
        while (result_len + before_len + repl_len + 1 > result_cap) {
            result_cap *= 2;
            result = (char*)realloc(result, result_cap);
            if (!result) { regfree(&reg); return strdup(s); }
        }
        memcpy(result + result_len, cursor, before_len);
        result_len += before_len;

        // Append replacement
        memcpy(result + result_len, repl, repl_len);
        result_len += repl_len;

        // Skip past match
        if (match.rm_eo == 0) {
            // Prevent infinite loop on zero-length match
            if (*cursor) {
                result[result_len++] = *cursor;
                cursor++;
            } else break;
        } else {
            cursor += match.rm_eo;
        }
    }

    // Append remaining text
    size_t remain = strlen(cursor);
    while (result_len + remain + 1 > result_cap) {
        result_cap *= 2;
        result = (char*)realloc(result, result_cap);
        if (!result) { regfree(&reg); return strdup(s); }
    }
    memcpy(result + result_len, cursor, remain);
    result_len += remain;
    result[result_len] = '\0';

    regfree(&reg);
    return result;
}

// Split string by regex pattern
DesiList* __re_split(const char* pattern, const char* s) {
    DesiList* result = list_new(1, NULL);
    if (!pattern || !s) return result;

    regex_t reg;
    if (regcomp(&reg, pattern, REG_EXTENDED) != 0) {
        list_append(result, (void*)strdup(s), 1);
        return result;
    }

    const char* cursor = s;
    regmatch_t match;

    while (regexec(&reg, cursor, 1, &match, 0) == 0) {
        if (match.rm_eo == 0) {
            cursor++;
            if (*cursor == '\0') break;
            continue;
        }
        // Add text before match
        int len = match.rm_so;
        char* part = (char*)malloc(len + 1);
        if (part) {
            memcpy(part, cursor, len);
            part[len] = '\0';
            list_append(result, (void*)part, 1);
        }
        cursor += match.rm_eo;
    }

    // Add remaining text
    list_append(result, (void*)strdup(cursor), 1);
    regfree(&reg);
    return result;
}
