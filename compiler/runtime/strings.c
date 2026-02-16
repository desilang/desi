// strings.c - Strings module C runtime
// Provides string utility functions for the Desi strings module.
// Uses __strings_ prefix to avoid collisions.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

// ============================================================
// Case conversion
// ============================================================

char* __strings_upper(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    for (size_t i = 0; i < len; i++)
        result[i] = (char)toupper((unsigned char)s[i]);
    result[len] = '\0';
    return result;
}

char* __strings_lower(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    for (size_t i = 0; i < len; i++)
        result[i] = (char)tolower((unsigned char)s[i]);
    result[len] = '\0';
    return result;
}

// Capitalize first letter, lowercase the rest
char* __strings_capitalize(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    for (size_t i = 0; i < len; i++)
        result[i] = (char)tolower((unsigned char)s[i]);
    if (len > 0)
        result[0] = (char)toupper((unsigned char)result[0]);
    result[len] = '\0';
    return result;
}

// Title case: capitalize first letter of each word
char* __strings_title(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    int capitalize_next = 1;
    for (size_t i = 0; i < len; i++) {
        if (isspace((unsigned char)s[i])) {
            result[i] = s[i];
            capitalize_next = 1;
        } else if (capitalize_next) {
            result[i] = (char)toupper((unsigned char)s[i]);
            capitalize_next = 0;
        } else {
            result[i] = (char)tolower((unsigned char)s[i]);
        }
    }
    result[len] = '\0';
    return result;
}

// ============================================================
// Search
// ============================================================

int __strings_starts_with(const char* s, const char* prefix) {
    if (!s || !prefix) return 0;
    size_t plen = strlen(prefix);
    if (plen > strlen(s)) return 0;
    return strncmp(s, prefix, plen) == 0;
}

int __strings_ends_with(const char* s, const char* suffix) {
    if (!s || !suffix) return 0;
    size_t slen = strlen(s);
    size_t xlen = strlen(suffix);
    if (xlen > slen) return 0;
    return strcmp(s + slen - xlen, suffix) == 0;
}

// Returns the index of the first occurrence of sub in s, or -1
int __strings_index_of(const char* s, const char* sub) {
    if (!s || !sub) return -1;
    const char* found = strstr(s, sub);
    if (!found) return -1;
    return (int)(found - s);
}

int __strings_contains(const char* s, const char* sub) {
    if (!s || !sub) return 0;
    return strstr(s, sub) != NULL;
}

int __strings_count(const char* s, const char* sub) {
    if (!s || !sub || !*sub) return 0;
    int count = 0;
    size_t sublen = strlen(sub);
    const char* p = s;
    while ((p = strstr(p, sub)) != NULL) {
        count++;
        p += sublen;
    }
    return count;
}

// ============================================================
// Transform
// ============================================================

// Trim whitespace from both ends
char* __strings_trim(const char* s) {
    if (!s) return strdup("");
    const char* start = s;
    while (*start && isspace((unsigned char)*start)) start++;
    const char* end = s + strlen(s) - 1;
    while (end > start && isspace((unsigned char)*end)) end--;
    size_t len = (size_t)(end - start + 1);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    memcpy(result, start, len);
    result[len] = '\0';
    return result;
}

char* __strings_trim_left(const char* s) {
    if (!s) return strdup("");
    const char* start = s;
    while (*start && isspace((unsigned char)*start)) start++;
    return strdup(start);
}

char* __strings_trim_right(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    while (len > 0 && isspace((unsigned char)s[len - 1])) len--;
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    memcpy(result, s, len);
    result[len] = '\0';
    return result;
}

// Replace all occurrences of old with new
char* __strings_replace(const char* s, const char* old_str, const char* new_str) {
    if (!s || !old_str || !new_str) return strdup(s ? s : "");
    size_t old_len = strlen(old_str);
    if (old_len == 0) return strdup(s);
    size_t new_len = strlen(new_str);

    // Count occurrences
    int count = 0;
    const char* p = s;
    while ((p = strstr(p, old_str)) != NULL) { count++; p += old_len; }

    size_t result_len = strlen(s) + count * (new_len - old_len);
    char* result = (char*)malloc(result_len + 1);
    if (!result) return strdup(s);

    char* dst = result;
    p = s;
    while (*p) {
        if (strncmp(p, old_str, old_len) == 0) {
            memcpy(dst, new_str, new_len);
            dst += new_len;
            p += old_len;
        } else {
            *dst++ = *p++;
        }
    }
    *dst = '\0';
    return result;
}

// Repeat string n times
char* __strings_repeat(const char* s, int n) {
    if (!s || n <= 0) return strdup("");
    size_t len = strlen(s);
    size_t total = len * (size_t)n;
    char* result = (char*)malloc(total + 1);
    if (!result) return strdup("");
    for (int i = 0; i < n; i++)
        memcpy(result + i * len, s, len);
    result[total] = '\0';
    return result;
}

// Reverse a string
char* __strings_reverse(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    for (size_t i = 0; i < len; i++)
        result[i] = s[len - 1 - i];
    result[len] = '\0';
    return result;
}

// ============================================================
// Character checks
// ============================================================

int __strings_is_empty(const char* s) {
    return !s || *s == '\0';
}

int __strings_is_digit(const char* s) {
    if (!s || *s == '\0') return 0;
    while (*s) {
        if (!isdigit((unsigned char)*s)) return 0;
        s++;
    }
    return 1;
}

int __strings_is_alpha(const char* s) {
    if (!s || *s == '\0') return 0;
    while (*s) {
        if (!isalpha((unsigned char)*s)) return 0;
        s++;
    }
    return 1;
}

int __strings_is_alnum(const char* s) {
    if (!s || *s == '\0') return 0;
    while (*s) {
        if (!isalnum((unsigned char)*s)) return 0;
        s++;
    }
    return 1;
}

int __strings_is_space(const char* s) {
    if (!s || *s == '\0') return 0;
    while (*s) {
        if (!isspace((unsigned char)*s)) return 0;
        s++;
    }
    return 1;
}

int __strings_is_upper(const char* s) {
    if (!s || *s == '\0') return 0;
    while (*s) {
        if (isalpha((unsigned char)*s) && !isupper((unsigned char)*s)) return 0;
        s++;
    }
    return 1;
}

int __strings_is_lower(const char* s) {
    if (!s || *s == '\0') return 0;
    while (*s) {
        if (isalpha((unsigned char)*s) && !islower((unsigned char)*s)) return 0;
        s++;
    }
    return 1;
}
