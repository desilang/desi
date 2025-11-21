#ifndef DESI_DICT_H
#define DESI_DICT_H

#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>

// Simple hashmap implementation for Desi dict[K, V]
// For Tier-0: supports string keys with generic value types

typedef struct dict_entry {
    char* key;                  // Owned string key
    void* value;                // Generic value pointer
    struct dict_entry* next;    // Chaining for collisions
} dict_entry_t;

typedef struct dict {
    dict_entry_t** buckets;     // Array of bucket chains
    size_t bucket_count;        // Number of buckets
    size_t entry_count;         // Number of entries
    size_t value_size;          // Size of each value in bytes
} dict_t;

// Core operations
dict_t* dict_new(size_t value_size);
void dict_free(dict_t* d);
void dict_insert(dict_t* d, const char* key, const void* value);
void* dict_get(dict_t* d, const char* key, const void* default_val);
bool dict_has_key(dict_t* d, const char* key);
void* dict_pop(dict_t* d, const char* key);
void dict_clear(dict_t* d);

// Collection methods
char** dict_keys(dict_t* d, size_t* out_len);
void** dict_values(dict_t* d, size_t* out_len);

// Internal helper
uint64_t dict_hash(const char* key);

#endif // DESI_DICT_H
