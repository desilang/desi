#ifndef SET_H
#define SET_H

#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>

typedef struct set_entry {
    char* key;
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
void set_add(set_t* s, const char* key);
bool set_contains(set_t* s, const char* key);
void set_remove(set_t* s, const char* key);
void set_clear(set_t* s);

// Helpers
char** set_to_array(set_t* s, size_t* out_len);

#endif
