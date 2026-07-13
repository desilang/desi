// re.c - Regular expression module
// Uses POSIX <regex.h> on macOS/Linux (no external deps).
// Windows has no <regex.h>: a minimal API-compatible shim below implements
// the ERE subset this module needs — literals, '.', [classes] with ranges
// and negation, '*'/'+'/'?' quantifiers, '^'/'$' anchors, and '\' escapes.
// Groups and alternation are not supported by the shim (regcomp fails,
// and the module's error paths handle that gracefully).
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
/* ---- minimal POSIX regex API shim (Windows only) ---- */
#define REG_EXTENDED 1
#define REG_NOSUB    2

typedef struct { long long rm_so, rm_eo; } regmatch_t;

typedef enum { RE_CHAR, RE_ANY, RE_CLASS, RE_BOL, RE_EOL } ReType;
typedef enum { RE_Q_ONE, RE_Q_STAR, RE_Q_PLUS, RE_Q_OPT } ReQuant;

typedef struct {
    ReType        type;
    ReQuant       quant;
    char          ch;
    unsigned char cls[32]; /* 256-bit membership bitmap */
    int           negate;
} ReTok;

typedef struct {
    ReTok* toks;
    int    n;
} regex_t;

static void re_cls_set(unsigned char* cls, unsigned char c) { cls[c >> 3] |= (unsigned char)(1 << (c & 7)); }
static int  re_cls_has(const unsigned char* cls, unsigned char c) { return cls[c >> 3] & (1 << (c & 7)); }

static int regcomp(regex_t* rx, const char* pat, int flags) {
    (void)flags;
    size_t plen = strlen(pat);
    rx->toks = (ReTok*)calloc(plen + 1, sizeof(ReTok));
    rx->n = 0;
    if (!rx->toks) return 1;

    const char* p = pat;
    while (*p) {
        ReTok* t = &rx->toks[rx->n];
        t->quant = RE_Q_ONE;
        if (*p == '^') {
            t->type = RE_BOL; p++;
        } else if (*p == '$') {
            t->type = RE_EOL; p++;
        } else if (*p == '.') {
            t->type = RE_ANY; p++;
        } else if (*p == '[') {
            t->type = RE_CLASS;
            p++;
            if (*p == '^') { t->negate = 1; p++; }
            if (*p == ']') { re_cls_set(t->cls, ']'); p++; } /* leading ] is literal */
            while (*p && *p != ']') {
                unsigned char lo = (unsigned char)*p;
                if (*p == '\\' && p[1]) { p++; lo = (unsigned char)*p; }
                if (p[1] == '-' && p[2] && p[2] != ']') {
                    unsigned char hi = (unsigned char)p[2];
                    for (unsigned c = lo; c <= hi; c++) re_cls_set(t->cls, (unsigned char)c);
                    p += 3;
                } else {
                    re_cls_set(t->cls, lo);
                    p++;
                }
            }
            if (*p != ']') { free(rx->toks); rx->toks = NULL; return 1; }
            p++;
        } else if (*p == '\\' && p[1]) {
            t->type = RE_CHAR;
            p++;
            switch (*p) {
                case 'n': t->ch = '\n'; break;
                case 't': t->ch = '\t'; break;
                case 'r': t->ch = '\r'; break;
                default:  t->ch = *p;   break;
            }
            p++;
        } else if (*p == '(' || *p == ')' || *p == '|') {
            /* groups/alternation unsupported */
            free(rx->toks); rx->toks = NULL; return 1;
        } else {
            t->type = RE_CHAR;
            t->ch = *p;
            p++;
        }

        if (*p == '*')      { t->quant = RE_Q_STAR; p++; }
        else if (*p == '+') { t->quant = RE_Q_PLUS; p++; }
        else if (*p == '?') { t->quant = RE_Q_OPT;  p++; }
        rx->n++;
    }
    return 0;
}

static void regfree(regex_t* rx) {
    if (rx) { free(rx->toks); rx->toks = NULL; rx->n = 0; }
}

static int re_single(const ReTok* t, const char* s) {
    unsigned char c = (unsigned char)*s;
    if (c == '\0') return 0;
    switch (t->type) {
        case RE_CHAR:  return c == (unsigned char)t->ch;
        case RE_ANY:   return c != '\n';
        case RE_CLASS: { int in = re_cls_has(t->cls, c); return t->negate ? !in : in; }
        default:       return 0;
    }
}

/* Match tokens t[0..n) starting at s (str = string start); set *end on success. */
static int re_here(const ReTok* t, int n, const char* s, const char* str, const char** end) {
    if (n == 0) { *end = s; return 1; }
    if (t->type == RE_BOL) {
        if (s != str) return 0;
        return re_here(t + 1, n - 1, s, str, end);
    }
    if (t->type == RE_EOL) {
        if (*s != '\0') return 0;
        return re_here(t + 1, n - 1, s, str, end);
    }
    switch (t->quant) {
        case RE_Q_ONE:
            if (!re_single(t, s)) return 0;
            return re_here(t + 1, n - 1, s + 1, str, end);
        case RE_Q_OPT:
            if (re_single(t, s) && re_here(t + 1, n - 1, s + 1, str, end)) return 1;
            return re_here(t + 1, n - 1, s, str, end);
        case RE_Q_STAR:
        case RE_Q_PLUS: {
            const char* p = s;
            while (re_single(t, p)) p++;
            const char* min = (t->quant == RE_Q_PLUS) ? s + 1 : s;
            /* greedy: longest repetition first, backtrack toward min */
            while (p >= min) {
                if (re_here(t + 1, n - 1, p, str, end)) return 1;
                if (p == min) break;
                p--;
            }
            return 0;
        }
    }
    return 0;
}

static int regexec(const regex_t* rx, const char* s, size_t nmatch, regmatch_t* pmatch, int eflags) {
    (void)eflags;
    if (!rx->toks && rx->n > 0) return 1;
    for (const char* start = s; ; start++) {
        const char* end;
        if (re_here(rx->toks, rx->n, start, s, &end)) {
            if (nmatch > 0 && pmatch) {
                pmatch[0].rm_so = start - s;
                pmatch[0].rm_eo = end - s;
            }
            return 0;
        }
        if (*start == '\0') break;
    }
    return 1; /* no match */
}
/* ---- end shim ---- */
#else
#include <regex.h>
#endif

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
