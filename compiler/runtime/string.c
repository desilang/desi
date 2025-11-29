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
