#include "set.h"
#include <stdlib.h>
#include <string.h>

#define INITIAL_BUCKET_COUNT 16
#define LOAD_FACTOR_THRESHOLD 0.75

// Simple integer hash function
static uint64_t set_hash_int(int64_t value) {
    // FNV-1a hash for integers
    uint64_t hash = 14695981039346656037ULL;
    hash ^= (uint64_t)value;
    hash *= 1099511628211ULL;
    return hash;
}

set_t* set_new() {
    set_t* s = malloc(sizeof(set_t));
    if (!s) return NULL;
    
    s->buckets = calloc(INITIAL_BUCKET_COUNT, sizeof(set_entry_t*));
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
