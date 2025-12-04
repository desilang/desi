#ifndef SET_H
#define SET_H

#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>

typedef struct set_entry {
    int64_t value;  // Store as integer for now (Tier-0)
    struct set_entry* next;
} set_entry_t;

typedef struct {
    set_entry_t** buckets;
    size_t bucket_count;
    size_t entry_count;
} set_t;

// Core operations
set_t* set_new();
void set_free(set_t* s);
void set_add(set_t* s, int64_t value);
bool set_contains(set_t* s, int64_t value);
void set_remove(set_t* s, int64_t value);
void set_clear(set_t* s);

// Helpers
int64_t* set_to_array(set_t* s, size_t* out_len);

// Advanced operations
set_t* set_union(set_t* s1, set_t* s2);
set_t* set_intersection(set_t* s1, set_t* s2);
set_t* set_difference(set_t* s1, set_t* s2);

// String representation
char* set_to_str(set_t* s);

#endif
