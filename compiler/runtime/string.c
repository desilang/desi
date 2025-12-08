// String operations runtime support
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

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

// Convert float to string (newly allocated)
char* float_to_str(double value) {
    // Reasonable buffer for float
    char* result = (char*)malloc(32);
    if (!result) {
        fprintf(stderr, "float_to_str: allocation failed\n");
        exit(1);
    }
    snprintf(result, 32, "%g", value);
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
char* string_substr(const char* s, int start, int len) {
    if (!s) return strdup("");
    
    int str_len = strlen(s);
    if (start < 0 || start >= str_len || len <= 0) {
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
