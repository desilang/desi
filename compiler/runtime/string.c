// String operations runtime support
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>  // int64_t (int_to_str) — MSVC needs this explicitly

// Concatenate two strings, returns newly allocated string
// Caller is responsible for freeing the result
char* string_concat(const char* a, const char* b) {
    if (!a) a = "";
    if (!b) b = "";
    
    size_t len_a = strlen(a);
    size_t len_b = strlen(b);
    size_t total = len_a + len_b + 1;
    
    char* result = (char*)malloc(total);
    if (!result) {
        fprintf(stderr, "string_concat: allocation failed\n");
        exit(1);
    }
    
    memcpy(result, a, len_a);
    memcpy(result + len_a, b, len_b);
    result[len_a + len_b] = '\0';
    
    return result;
}

// Convert int to string (newly allocated)
char* int_to_str(int value) {
    // Max int is ~10 digits + sign + null
    char* result = (char*)malloc(12);
    if (!result) {
        fprintf(stderr, "int_to_str: allocation failed\n");
        exit(1);
    }
    snprintf(result, 12, "%d", value);
    return result;
}

// Convert float to string (Python-style: trims trailing zeros, preserves .0)
// 3.14    -> "3.14"
// 3.10    -> "3.1"
// 42.0    -> "42.0"  (not "42")
// 100.0   -> "100.0" (not "100")
// 1e15    -> "1e+15" (large numbers use scientific notation)
char* float_to_str(double value) {
    char* result = (char*)malloc(64);
    if (!result) {
        fprintf(stderr, "float_to_str: allocation failed\n");
        exit(1);
    }
    // Use %.15g — matches Python's effective precision, avoids IEEE 754 noise
    snprintf(result, 64, "%.15g", value);

    // If the result has no decimal point and no 'e'/'E',
    // it's a whole number — append ".0" to distinguish from int
    if (!strchr(result, '.') && !strchr(result, 'e') && !strchr(result, 'E')
        && !strchr(result, 'n') && !strchr(result, 'i')) {  // skip nan/inf
        strcat(result, ".0");
    }

    // Trim unnecessary trailing zeros after decimal point, but keep at least one
    char* dot = strchr(result, '.');
    if (dot && !strchr(result, 'e') && !strchr(result, 'E')) {
        char* end = result + strlen(result) - 1;
        while (end > dot + 1 && *end == '0') {
            *end = '\0';
            end--;
        }
    }

    return result;
}

// ==================== Accumulator support ====================
// The lowerer rewrites eligible mutable string accumulators
// (let mut s = "lit"; s := s + x in a loop) to these owned-string
// helpers, so the loop stops leaking every intermediate value and
// stops copying the whole prefix each iteration.

// Fresh owned copy of a literal (accumulator initialization/reset).
char* __desi_str_new(const char* s) {
    return strdup(s ? s : "");
}

// Append suffix to an OWNED heap string, freeing/reusing the old
// allocation. realloc typically extends in place, so building a string
// of length N by appending costs ~O(N) bytes copied instead of the
// O(N^2) that repeated string_concat incurs — and nothing leaks.
// `old` must be heap-owned (the lowerer guarantees it via __desi_str_new)
// and `suffix` must not alias `old` (the lowerer rejects s := s + s).
char* __desi_str_append_free(char* old, const char* suffix) {
    size_t old_len = old ? strlen(old) : 0;
    size_t suf_len = suffix ? strlen(suffix) : 0;
    char* result = (char*)realloc(old, old_len + suf_len + 1);
    if (!result) {
        return old; // OOM: keep the old value, drop the suffix (never crash)
    }
    if (suf_len > 0) {
        memcpy(result + old_len, suffix, suf_len);
    }
    result[old_len + suf_len] = '\0';
    return result;
}

// Convert bool to string (newly allocated)
char* bool_to_str(int value) {
    if (value) {
        char* result = (char*)malloc(5);
        strcpy(result, "true");
        return result;
    } else {
        char* result = (char*)malloc(6);
        strcpy(result, "false");
        return result;
    }
}

// Get string length
int string_len(const char* s) {
    return s ? (int)strlen(s) : 0;
}

// Get substring (start is 0-indexed, len is character count)
// Returns newly allocated string
// Supports negative start (Python-style: -1 = last char)
char* string_substr(const char* s, int start, int len) {
    if (!s) return strdup("");
    
    int str_len = strlen(s);
    
    // Handle negative start (Python-style)
    if (start < 0) {
        start = str_len + start;
    }
    
    // Clamp to bounds
    if (start < 0) start = 0;
    if (start >= str_len || len <= 0) {
        return strdup("");
    }
    
    if (start + len > str_len) {
        len = str_len - start;
    }
    
    char* result = (char*)malloc(len + 1);
    if (!result) {
        fprintf(stderr, "string_substr: allocation failed\n");
        exit(1);
    }
    
    memcpy(result, s + start, len);
    result[len] = '\0';
    
    return result;
}

// Slice string from start (inclusive) to end (exclusive)
// Supports negative indices like Python
char* string_slice(const char* s, int start, int end) {
    if (!s) return strdup("");
    
    int str_len = strlen(s);
    
    // Handle negative indices (Python-style)
    if (start < 0) start = str_len + start;
    if (end < 0) end = str_len + end;
    
    // Clamp to bounds
    if (start < 0) start = 0;
    if (end > str_len) end = str_len;
    if (start >= end) return strdup("");
    
    int len = end - start;
    char* result = (char*)malloc(len + 1);
    if (!result) {
        fprintf(stderr, "string_slice: allocation failed\n");
        exit(1);
    }
    
    memcpy(result, s + start, len);
    result[len] = '\0';
    
    return result;
}

// Slice string with step (Python-style)
// step > 0: forward iteration
// step < 0: reverse iteration (e.g., [::-1] reverses)
// Sentinel values: INT32_MAX for start means "use default", INT32_MIN for end means "use default"
#define STR_SENTINEL_START 2147483647
#define STR_SENTINEL_END   (-2147483647 - 1)

char* string_slice_step(const char* s, int start, int end, int step) {
    if (!s) return strdup("");
    
    if (step == 0) {
        fprintf(stderr, "string_slice_step: step cannot be zero\n");
        return strdup("");
    }
    
    int str_len = strlen(s);
    
    // Handle sentinel values (omitted start/end)
    if (step > 0) {
        // Forward: start defaults to 0, end defaults to len
        if (start == STR_SENTINEL_START) start = 0;
        if (end == STR_SENTINEL_END) end = str_len;
        // Handle negative indices
        if (start < 0) start = str_len + start;
        if (end < 0) end = str_len + end;
        if (start < 0) start = 0;
        if (end > str_len) end = str_len;
    } else {
        // Reverse: start defaults to len-1, end defaults to -1 (before first)
        if (start == STR_SENTINEL_START) start = str_len - 1;
        if (end == STR_SENTINEL_END) end = -1;
        // Handle negative indices
        if (start < 0) start = str_len + start;
        if (end < -1 && end != STR_SENTINEL_END) end = str_len + end;
        if (start >= str_len) start = str_len - 1;
    }
    
    // Calculate result length
    int count = 0;
    if (step > 0) {
        for (int i = start; i < end; i += step) {
            if (i >= 0 && i < str_len) count++;
        }
    } else {
        for (int i = start; i > end; i += step) {
            if (i >= 0 && i < str_len) count++;
        }
    }
    
    char* result = (char*)malloc(count + 1);
    if (!result) {
        fprintf(stderr, "string_slice_step: allocation failed\n");
        exit(1);
    }
    
    int idx = 0;
    if (step > 0) {
        for (int i = start; i < end && idx < count; i += step) {
            if (i >= 0 && i < str_len) {
                result[idx++] = s[i];
            }
        }
    } else {
        for (int i = start; i > end && idx < count; i += step) {
            if (i >= 0 && i < str_len) {
                result[idx++] = s[i];
            }
        }
    }
    result[idx] = '\0';
    
    return result;
}

// Forward declare DesiList from list.h
typedef struct {
    void** data;
    size_t length;
    size_t capacity;
    int type_tag;
    char* (*to_str_fn)(void*);
} DesiList;

// External list functions
extern DesiList* list_new(int type_tag, char* (*to_str_fn)(void*));
extern void list_append(DesiList* list, void* item, int type_tag);

// Split string by delimiter, returns DesiList of strings
// type_tag 1 = str
DesiList* string_split(const char* s, const char* delim) {
    DesiList* result = list_new(1, NULL);  // type_tag 1 = str
    
    if (!s || !delim || strlen(delim) == 0) {
        // Return list with original string
        if (s) {
            list_append(result, (void*)strdup(s), 1);
        }
        return result;
    }
    
    size_t delim_len = strlen(delim);
    const char* start = s;
    const char* found;
    
    while ((found = strstr(start, delim)) != NULL) {
        size_t part_len = found - start;
        char* part = (char*)malloc(part_len + 1);
        memcpy(part, start, part_len);
        part[part_len] = '\0';
        list_append(result, (void*)part, 1);
        start = found + delim_len;
    }
    
    // Add the remaining part after last delimiter
    if (*start || start == s) {
        list_append(result, (void*)strdup(start), 1);
    }
    
    return result;
}

// Join list of strings with delimiter
// Expects list with type_tag 1 (str)
char* string_join(DesiList* list, const char* delim) {
    if (!list || list->length == 0) {
        return strdup("");
    }
    
    if (!delim) delim = "";
    size_t delim_len = strlen(delim);
    
    // Calculate total length
    size_t total_len = 0;
    for (size_t i = 0; i < list->length; i++) {
        const char* s = (const char*)list->data[i];
        if (s) total_len += strlen(s);
        if (i < list->length - 1) total_len += delim_len;
    }
    
    char* result = (char*)malloc(total_len + 1);
    if (!result) {
        fprintf(stderr, "string_join: allocation failed\n");
        exit(1);
    }
    
    char* ptr = result;
    for (size_t i = 0; i < list->length; i++) {
        const char* s = (const char*)list->data[i];
        if (s) {
            size_t len = strlen(s);
            memcpy(ptr, s, len);
            ptr += len;
        }
        if (i < list->length - 1 && delim_len > 0) {
            memcpy(ptr, delim, delim_len);
            ptr += delim_len;
        }
    }
    *ptr = '\0';
    
    return result;
}

// Replace all occurrences of old_str with new_str
char* string_replace(const char* s, const char* old_str, const char* new_str) {
    if (!s) return strdup("");
    if (!old_str || strlen(old_str) == 0) return strdup(s);
    if (!new_str) new_str = "";
    
    size_t old_len = strlen(old_str);
    size_t new_len = strlen(new_str);
    
    // Count occurrences
    size_t count = 0;
    const char* tmp = s;
    while ((tmp = strstr(tmp, old_str)) != NULL) {
        count++;
        tmp += old_len;
    }
    
    if (count == 0) return strdup(s);
    
    // Calculate new length
    size_t s_len = strlen(s);
    size_t result_len = s_len + count * ((long)new_len - (long)old_len);
    
    char* result = (char*)malloc(result_len + 1);
    if (!result) {
        fprintf(stderr, "string_replace: allocation failed\n");
        exit(1);
    }
    
    char* ptr = result;
    const char* src = s;
    const char* found;
    
    while ((found = strstr(src, old_str)) != NULL) {
        size_t before_len = found - src;
        memcpy(ptr, src, before_len);
        ptr += before_len;
        memcpy(ptr, new_str, new_len);
        ptr += new_len;
        src = found + old_len;
    }
    
    // Copy remaining
    strcpy(ptr, src);
    
    return result;
}

// Check if needle is contained in haystack
// Returns 1 (true) if needle is found in haystack, 0 (false) otherwise
// Used for 'in' operator: "a" in "abc" -> true
int string_contains(const char* haystack, const char* needle) {
    if (!haystack || !needle) return 0;
    if (*needle == '\0') return 1;  // Empty needle is always found
    return strstr(haystack, needle) != NULL ? 1 : 0;
}
