#include "set.h"
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <stdbool.h>
#include <stdint.h>

#define INITIAL_BUCKET_COUNT 16
#define LOAD_FACTOR_THRESHOLD 0.75
#define SET_INITIAL_BUCKETS 16

// Simple integer hash function
static uint64_t set_hash_int(int64_t value) {
    // FNV-1a hash for integers
    uint64_t hash = 14695981039346656037ULL;
    hash ^= (uint64_t)value;
    hash *= 1099511628211ULL;
    return hash;
}

set_t* set_new(ElemToStrFunc elem_to_str_fn) {
    set_t* s = (set_t*)malloc(sizeof(set_t));
    if (!s) return NULL;
    
    s->bucket_count = SET_INITIAL_BUCKETS;
    s->entry_count = 0;
    s->elem_to_str_fn = elem_to_str_fn;  // Store function pointer
    
    s->buckets = calloc(s->bucket_count, sizeof(set_entry_t*));
    if (!s->buckets) {
        free(s);
        return NULL;
    }
    
    s->bucket_count = INITIAL_BUCKET_COUNT;
    s->entry_count = 0;
    return s;
}

void set_free(set_t* s) {
    if (!s) return;
    
    for (size_t i = 0; i < s->bucket_count; i++) {
        set_entry_t* entry = s->buckets[i];
        while (entry) {
            set_entry_t* next = entry->next;
            free(entry);
            entry = next;
        }
    }
    
    free(s->buckets);
    free(s);
}

void set_add(set_t* s, int64_t value) {
    if (!s) return;
    
    uint64_t hash = set_hash_int(value);
    size_t index = hash % s->bucket_count;
    
    // Check if value already exists
    set_entry_t* entry = s->buckets[index];
    while (entry) {
        if (entry->value == value) {
            return; // Already exists
        }
        entry = entry->next;
    }
    
    // Create new entry
    set_entry_t* new_entry = malloc(sizeof(set_entry_t));
    if (!new_entry) return;
    
    new_entry->value = value;
    
    // Insert at head
    new_entry->next = s->buckets[index];
    s->buckets[index] = new_entry;
    s->entry_count++;
}

bool set_contains(set_t* s, int64_t value) {
    if (!s) return false;
    
    uint64_t hash = set_hash_int(value);
    size_t index = hash % s->bucket_count;
    
    set_entry_t* entry = s->buckets[index];
    while (entry) {
        if (entry->value == value) {
            return true;
        }
        entry = entry->next;
    }
    
    return false;
}

void set_remove(set_t* s, int64_t value) {
    if (!s) return;
    
    uint64_t hash = set_hash_int(value);
    size_t index = hash % s->bucket_count;
    
    set_entry_t* entry = s->buckets[index];
    set_entry_t* prev = NULL;
    
    while (entry) {
        if (entry->value == value) {
            if (prev) {
                prev->next = entry->next;
            } else {
                s->buckets[index] = entry->next;
            }
            
            free(entry);
            s->entry_count--;
            return;
        }
        prev = entry;
        entry = entry->next;
    }
}

void set_clear(set_t* s) {
    if (!s) return;
    
    for (size_t i = 0; i < s->bucket_count; i++) {
        set_entry_t* entry = s->buckets[i];
        while (entry) {
            set_entry_t* next = entry->next;
            free(entry);
            entry = next;
        }
        s->buckets[i] = NULL;
    }
    s->entry_count = 0;
}

int64_t set_len(set_t* s) {
    return s ? (int64_t)s->entry_count : 0;
}

int64_t* set_to_array(set_t* s, size_t* out_len) {
    if (!s || !out_len) return NULL;
    
    *out_len = s->entry_count;
    if (s->entry_count == 0) return NULL;
    
    int64_t* values = malloc(sizeof(int64_t) * s->entry_count);
    if (!values) return NULL;
    
    size_t k = 0;
    for (size_t i = 0; i < s->bucket_count; i++) {
        set_entry_t* entry = s->buckets[i];
        while (entry) {
            values[k++] = entry->value;
            entry = entry->next;
        }
    }
    
    return values;
}

// Advanced set operations

// Union: Create new set with all elements from both sets
set_t* set_union(set_t* s1, set_t* s2) {
    if (!s1 || !s2) return NULL;
    
    set_t* result = set_new(NULL);  // No custom to_str for now
    if (!result) return NULL;
    
    // Add all elements from s1
    for (size_t i = 0; i < s1->bucket_count; i++) {
        set_entry_t* entry = s1->buckets[i];
        while (entry) {
            set_add(result, entry->value);
            entry = entry->next;
        }
    }
    
    // Add all elements from s2 (duplicates automatically handled)
    for (size_t i = 0; i < s2->bucket_count; i++) {
        set_entry_t* entry = s2->buckets[i];
        while (entry) {
            set_add(result, entry->value);
            entry = entry->next;
        }
    }
    
    return result;
}

// Intersection: Create new set with common elements
set_t* set_intersection(set_t* s1, set_t* s2) {
    if (!s1 || !s2) return NULL;
    
    set_t* result = set_new(NULL);  // No custom to_str for now
    if (!result) return NULL;
    
    // Iterate s1, add to result if exists in s2
    for (size_t i = 0; i < s1->bucket_count; i++) {
        set_entry_t* entry = s1->buckets[i];
        while (entry) {
            if (set_contains(s2, entry->value)) {
                set_add(result, entry->value);
            }
            entry = entry->next;
        }
    }
    
    return result;
}

// Difference: Create new set with elements in s1 but not in s2
set_t* set_difference(set_t* s1, set_t* s2) {
    if (!s1 || !s2) return NULL;
    
    set_t* result = set_new(NULL);  // No custom to_str for now
    if (!result) return NULL;
    
    // Iterate s1, add to result if NOT in s2
    for (size_t i = 0; i < s1->bucket_count; i++) {
        set_entry_t* entry = s1->buckets[i];
        while (entry) {
            if (!set_contains(s2, entry->value)) {
                set_add(result, entry->value);
            }
            entry = entry->next;
        }
    }
    
    return result;
}

// ========== String Representation ==========

// Convert set to string representation
char* set_to_str(set_t* s) {
    if (!s) {
        char* result = (char*)malloc(7);
        strcpy(result, "<null>");
        return result;
    }
    
    // Allocate initial buffer
    size_t bufsize = 256;
    char* buffer = (char*)malloc(bufsize);
    if (!buffer) {
        fprintf(stderr, "set_to_str: malloc failed\n");
        exit(1);
    }
    
    size_t pos = 0;
    buffer[pos++] = '{';
    
    bool first = true;
    for (size_t i = 0; i < s->bucket_count; i++) {
        set_entry_t* entry = s->buckets[i];
        while (entry) {
            if (!first) {
                if (pos + 2 >= bufsize) {
                    bufsize *= 2;
                    buffer = (char*)realloc(buffer, bufsize);
                }
                buffer[pos++] = ',';
                buffer[pos++] = ' ';
            }
            first = false;
            
            char temp[32];
            snprintf(temp, sizeof(temp), "%lld", (long long)entry->value);
            size_t len = strlen(temp);
            
            while (pos + len >= bufsize) {
                bufsize *= 2;
                buffer = (char*)realloc(buffer, bufsize);
            }
            
            memcpy(buffer + pos, temp, len);
            pos += len;
            
            entry = entry->next;
        }
    }
    
    if (pos + 2 >= bufsize) {
        buffer = (char*)realloc(buffer, pos + 2);
    }
    
    buffer[pos++] = '}';
    buffer[pos] = '\0';
    
    return buffer;
}
