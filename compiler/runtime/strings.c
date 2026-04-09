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

// ============================================================
// Case conversion (additional)
// ============================================================

// Swap case: uppercase ↔ lowercase
char* __strings_swapcase(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if (isupper(c))
            result[i] = (char)tolower(c);
        else if (islower(c))
            result[i] = (char)toupper(c);
        else
            result[i] = s[i];
    }
    result[len] = '\0';
    return result;
}

// ============================================================
// Search (additional)
// ============================================================

// Find last occurrence of sub in s, or -1 (Python rfind)
int __strings_last_index_of(const char* s, const char* sub) {
    if (!s || !sub) return -1;
    size_t slen = strlen(s);
    size_t sublen = strlen(sub);
    if (sublen > slen) return -1;
    if (sublen == 0) return (int)slen;
    for (int i = (int)(slen - sublen); i >= 0; i--) {
        if (strncmp(s + i, sub, sublen) == 0)
            return i;
    }
    return -1;
}

// ============================================================
// Transform (additional)
// ============================================================

// Remove prefix if present (Python 3.9+ removeprefix)
char* __strings_removeprefix(const char* s, const char* prefix) {
    if (!s || !prefix) return strdup(s ? s : "");
    size_t plen = strlen(prefix);
    if (plen > 0 && strncmp(s, prefix, plen) == 0)
        return strdup(s + plen);
    return strdup(s);
}

// Remove suffix if present (Python 3.9+ removesuffix)
char* __strings_removesuffix(const char* s, const char* suffix) {
    if (!s || !suffix) return strdup(s ? s : "");
    size_t slen = strlen(s);
    size_t xlen = strlen(suffix);
    if (xlen > 0 && xlen <= slen && strcmp(s + slen - xlen, suffix) == 0) {
        char* result = (char*)malloc(slen - xlen + 1);
        if (!result) return strdup(s);
        memcpy(result, s, slen - xlen);
        result[slen - xlen] = '\0';
        return result;
    }
    return strdup(s);
}

// Left-pad string to width with fill character (Python rjust)
char* __strings_pad_left(const char* s, int width, const char* fill) {
    if (!s) return strdup("");
    size_t slen = strlen(s);
    if ((int)slen >= width) return strdup(s);
    char fc = (fill && *fill) ? fill[0] : ' ';
    size_t pad = (size_t)(width) - slen;
    char* result = (char*)malloc(width + 1);
    if (!result) return strdup(s);
    memset(result, fc, pad);
    memcpy(result + pad, s, slen);
    result[width] = '\0';
    return result;
}

// Right-pad string to width with fill character (Python ljust)
char* __strings_pad_right(const char* s, int width, const char* fill) {
    if (!s) return strdup("");
    size_t slen = strlen(s);
    if ((int)slen >= width) return strdup(s);
    char fc = (fill && *fill) ? fill[0] : ' ';
    size_t pad = (size_t)(width) - slen;
    char* result = (char*)malloc(width + 1);
    if (!result) return strdup(s);
    memcpy(result, s, slen);
    memset(result + slen, fc, pad);
    result[width] = '\0';
    return result;
}

// Center string in field of given width (Python center)
char* __strings_center(const char* s, int width, const char* fill) {
    if (!s) return strdup("");
    size_t slen = strlen(s);
    if ((int)slen >= width) return strdup(s);
    char fc = (fill && *fill) ? fill[0] : ' ';
    size_t total_pad = (size_t)(width) - slen;
    size_t left_pad = total_pad / 2;
    size_t right_pad = total_pad - left_pad;
    char* result = (char*)malloc(width + 1);
    if (!result) return strdup(s);
    memset(result, fc, left_pad);
    memcpy(result + left_pad, s, slen);
    memset(result + left_pad + slen, fc, right_pad);
    result[width] = '\0';
    return result;
}

// Zero-fill: pad with zeros on the left, preserving sign (Python zfill)
char* __strings_zfill(const char* s, int width) {
    if (!s) return strdup("");
    size_t slen = strlen(s);
    if ((int)slen >= width) return strdup(s);
    size_t pad = (size_t)(width) - slen;
    char* result = (char*)malloc(width + 1);
    if (!result) return strdup(s);
    int sign_offset = 0;
    if (slen > 0 && (s[0] == '+' || s[0] == '-')) {
        result[0] = s[0];
        sign_offset = 1;
    }
    memset(result + sign_offset, '0', pad);
    memcpy(result + sign_offset + pad, s + sign_offset, slen - sign_offset);
    result[width] = '\0';
    return result;
}

// Get character at index as single-char string (supports negative indices)
char* __strings_char_at(const char* s, int idx) {
    if (!s) return strdup("");
    int len = (int)strlen(s);
    if (idx < 0) idx += len;
    if (idx < 0 || idx >= len) return strdup("");
    char* result = (char*)malloc(2);
    if (!result) return strdup("");
    result[0] = s[idx];
    result[1] = '\0';
    return result;
}

// ============================================================
// Character checks (additional)
// ============================================================

int __strings_is_ascii(const char* s) {
    if (!s || *s == '\0') return 1; // empty is ascii (Python behavior)
    while (*s) {
        if ((unsigned char)*s > 127) return 0;
        s++;
    }
    return 1;
}

int __strings_is_printable(const char* s) {
    if (!s || *s == '\0') return 1; // empty is printable (Python behavior)
    while (*s) {
        if (!isprint((unsigned char)*s)) return 0;
        s++;
    }
    return 1;
}

// ============================================================
// Beyond Python — Desi extras
// ============================================================

// Truncate with suffix: "Hello World" → "Hello..." (max n chars total)
char* __strings_truncate(const char* s, int max_len, const char* suffix) {
    if (!s) return strdup("");
    size_t slen = strlen(s);
    if ((int)slen <= max_len) return strdup(s);
    if (!suffix) suffix = "...";
    size_t sfx_len = strlen(suffix);
    int content_len = max_len - (int)sfx_len;
    if (content_len < 0) content_len = 0;
    char* result = (char*)malloc(max_len + 1);
    if (!result) return strdup(s);
    memcpy(result, s, content_len);
    memcpy(result + content_len, suffix, sfx_len);
    result[content_len + sfx_len] = '\0';
    return result;
}

// URL-friendly slug: "Hello World!" → "hello-world"
char* __strings_slugify(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    int j = 0;
    int prev_dash = 1; // start true to avoid leading dash
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if (isalnum(c)) {
            result[j++] = (char)tolower(c);
            prev_dash = 0;
        } else if (!prev_dash) {
            result[j++] = '-';
            prev_dash = 1;
        }
    }
    // Remove trailing dash
    if (j > 0 && result[j - 1] == '-') j--;
    result[j] = '\0';
    return result;
}

// ============================================================
// Split / Join (wrapping existing string.c functions)
// ============================================================

// Forward-declare existing string.c functions
typedef struct {
    void** data;
    size_t length;
    size_t capacity;
    int type_tag;
    char* (*to_str_fn)(void*);
} DesiList;

extern DesiList* list_new(int type_tag, char* (*to_str_fn)(void*));
extern void list_append(DesiList* list, void* item, int type_tag);
extern DesiList* string_split(const char* s, const char* delim);
extern char* string_join(DesiList* list, const char* delim);

// Split string by delimiter, returns list of strings
DesiList* __strings_split(const char* s, const char* delim) {
    return string_split(s, delim);
}

// Join list of strings with separator
char* __strings_join(const char* sep, DesiList* parts) {
    return string_join(parts, sep);
}

// Split by newlines (handles \n, \r\n, \r)
DesiList* __strings_splitlines(const char* s) {
    DesiList* result = list_new(1, NULL);
    if (!s) return result;

    const char* start = s;
    while (*start) {
        const char* end = start;
        while (*end && *end != '\n' && *end != '\r') end++;

        size_t len = (size_t)(end - start);
        char* line = (char*)malloc(len + 1);
        if (line) {
            memcpy(line, start, len);
            line[len] = '\0';
        }
        list_append(result, (void*)line, 1);

        if (*end == '\r' && *(end + 1) == '\n')
            end += 2; // \r\n
        else if (*end)
            end += 1; // \n or \r
        start = end;
    }
    return result;
}

// ============================================================
// Case Convention Converters (highly requested)
// ============================================================

// camelCase: "hello_world" or "hello-world" or "Hello World" -> "helloWorld"
char* __strings_camel_case(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    int j = 0;
    int capitalize_next = 0;
    int first = 1;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if (c == '_' || c == '-' || isspace(c)) {
            capitalize_next = 1;
        } else if (capitalize_next && !first) {
            result[j++] = (char)toupper(c);
            capitalize_next = 0;
        } else {
            result[j++] = first ? (char)tolower(c) : s[i];
            capitalize_next = 0;
            first = 0;
        }
    }
    result[j] = '\0';
    return result;
}

// PascalCase: "hello_world" -> "HelloWorld"
char* __strings_pascal_case(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");
    int j = 0;
    int capitalize_next = 1;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if (c == '_' || c == '-' || isspace(c)) {
            capitalize_next = 1;
        } else if (capitalize_next) {
            result[j++] = (char)toupper(c);
            capitalize_next = 0;
        } else {
            result[j++] = s[i];
        }
    }
    result[j] = '\0';
    return result;
}

// snake_case: "helloWorld" or "Hello World" or "hello-world" -> "hello_world"
char* __strings_snake_case(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    // Worst case: every char gets a _ prefix
    char* result = (char*)malloc(len * 2 + 1);
    if (!result) return strdup("");
    int j = 0;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if (c == '-' || isspace(c)) {
            if (j > 0 && result[j-1] != '_') result[j++] = '_';
        } else if (isupper(c)) {
            if (j > 0 && result[j-1] != '_') result[j++] = '_';
            result[j++] = (char)tolower(c);
        } else {
            result[j++] = c;
        }
    }
    // Remove leading underscore
    if (j > 0 && result[0] == '_') {
        memmove(result, result + 1, j);
        j--;
    }
    result[j] = '\0';
    return result;
}

// kebab-case: "helloWorld" or "hello_world" -> "hello-world"
char* __strings_kebab_case(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len * 2 + 1);
    if (!result) return strdup("");
    int j = 0;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if (c == '_' || isspace(c)) {
            if (j > 0 && result[j-1] != '-') result[j++] = '-';
        } else if (isupper(c)) {
            if (j > 0 && result[j-1] != '-') result[j++] = '-';
            result[j++] = (char)tolower(c);
        } else {
            result[j++] = c;
        }
    }
    if (j > 0 && result[0] == '-') {
        memmove(result, result + 1, j);
        j--;
    }
    result[j] = '\0';
    return result;
}

// SCREAMING_SNAKE_CASE: "helloWorld" -> "HELLO_WORLD"
char* __strings_screaming_snake(const char* s) {
    char* snake = __strings_snake_case(s);
    if (!snake) return strdup("");
    char* result = __strings_upper(snake);
    free(snake);
    return result;
}

// ============================================================
// Text Utilities (language gaps)
// ============================================================

// Word wrap: break long text at word boundaries
char* __strings_word_wrap(const char* s, int width) {
    if (!s || width <= 0) return strdup(s ? s : "");
    size_t len = strlen(s);
    // Worst case: newline every `width` chars
    char* result = (char*)malloc(len + len / width + 2);
    if (!result) return strdup(s);
    
    int col = 0;
    int j = 0;
    int last_space_src = -1;
    int last_space_dst = -1;
    
    for (size_t i = 0; i < len; i++) {
        if (s[i] == '\n') {
            result[j++] = '\n';
            col = 0;
            last_space_src = -1;
            last_space_dst = -1;
            continue;
        }
        if (s[i] == ' ') {
            last_space_src = (int)i;
            last_space_dst = j;
        }
        result[j++] = s[i];
        col++;
        if (col >= width && last_space_dst >= 0) {
            result[last_space_dst] = '\n';
            col = j - last_space_dst - 1;
            last_space_src = -1;
            last_space_dst = -1;
        }
    }
    result[j] = '\0';
    return result;
}

// Check if string is numeric (int or float)
int __strings_is_numeric(const char* s) {
    if (!s || *s == '\0') return 0;
    const char* p = s;
    if (*p == '+' || *p == '-') p++;
    if (*p == '\0') return 0;
    int has_dot = 0;
    int has_digit = 0;
    while (*p) {
        if (*p == '.') {
            if (has_dot) return 0;
            has_dot = 1;
        } else if (isdigit((unsigned char)*p)) {
            has_digit = 1;
        } else {
            return 0;
        }
        p++;
    }
    return has_digit;
}

// Dedent: remove common leading whitespace from multiline strings
char* __strings_dedent(const char* s) {
    if (!s) return strdup("");
    
    // Find minimum indentation (ignoring empty lines)
    int min_indent = 9999;
    const char* line = s;
    while (*line) {
        // Skip empty/whitespace-only lines for min calculation
        const char* end = line;
        while (*end && *end != '\n') end++;
        
        int indent = 0;
        const char* p = line;
        while (p < end && (*p == ' ' || *p == '\t')) {
            indent += (*p == '\t') ? 4 : 1;
            p++;
        }
        if (p < end) { // non-empty line
            if (indent < min_indent) min_indent = indent;
        }
        
        line = *end ? end + 1 : end;
    }
    
    if (min_indent <= 0 || min_indent == 9999) return strdup(s);
    
    // Build result with indentation removed
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup(s);
    
    int j = 0;
    line = s;
    while (*line) {
        int removed = 0;
        while (*line && *line != '\n' && removed < min_indent) {
            if (*line == '\t') removed += 4;
            else if (*line == ' ') removed += 1;
            else break;
            line++;
        }
        while (*line && *line != '\n') {
            result[j++] = *line++;
        }
        if (*line == '\n') result[j++] = *line++;
    }
    result[j] = '\0';
    return result;
}

// Abbreviate: "Hello World" with max=8 -> "Hello..."
// Unlike truncate, this breaks at word boundaries
char* __strings_abbreviate(const char* s, int max_len) {
    if (!s) return strdup("");
    size_t slen = strlen(s);
    if ((int)slen <= max_len) return strdup(s);
    if (max_len <= 3) {
        char* r = (char*)malloc(max_len + 1);
        if (!r) return strdup("");
        for (int i = 0; i < max_len; i++) r[i] = '.';
        r[max_len] = '\0';
        return r;
    }
    int limit = max_len - 3;
    // Find last space before limit
    int break_at = limit;
    for (int i = limit; i >= 0; i--) {
        if (s[i] == ' ') { break_at = i; break; }
    }
    char* result = (char*)malloc(break_at + 4);
    if (!result) return strdup("");
    memcpy(result, s, break_at);
    result[break_at] = '.';
    result[break_at+1] = '.';
    result[break_at+2] = '.';
    result[break_at+3] = '\0';
    return result;
}
