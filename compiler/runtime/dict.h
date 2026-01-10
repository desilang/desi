#ifndef DESI_DICT_H
#define DESI_DICT_H

#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>

// Function pointer type for value-to-string conversion
typedef char* (*ElemToStrFunc)(void*);

// Type tags for dict keys and values
// 0=int, 1=str, 2=bool, 3=float, 4=custom (class with __hash__/__eq__)
#define TYPE_TAG_INT    0
#define TYPE_TAG_STR    1
#define TYPE_TAG_BOOL   2
#define TYPE_TAG_FLOAT  3
#define TYPE_TAG_CUSTOM 4

// Function pointer types for custom key types (Phase 2)
typedef uint64_t (*KeyHashFunc)(void*);           // __hash__(self) -> u64
typedef bool (*KeyEqFunc)(void*, void*);          // __eq__(self, other) -> bool

// Simple hashmap implementation for Desi dict[K, V]
// Phase 1: primitive keys (int, str, bool, float)
// Phase 2: custom types with __hash__/__eq__ dunders

typedef struct dict_entry {
    int64_t key_int;            // Inline storage for int/bool keys
    char* key_str;              // String key (owned, strdup'd)
    double key_float;           // Float key
    void* key_ptr;              // Custom type key (owned, malloc'd copy)
    void* value;                // Generic value pointer
    struct dict_entry* next;    // Chaining for collisions
} dict_entry_t;

typedef struct dict {
    dict_entry_t** buckets;     // Array of bucket chains
    size_t bucket_count;        // Number of buckets
    size_t entry_count;         // Number of entries
    size_t key_size;            // Size of custom key in bytes (for TYPE_TAG_CUSTOM)
    size_t value_size;          // Size of each value in bytes
    int key_type_tag;           // Key type: 0=int, 1=str, 2=bool, 3=float, 4=custom
    int value_type_tag;         // Value type tag for printing
    KeyHashFunc key_hash_fn;    // Custom key hash function (for TYPE_TAG_CUSTOM)
    KeyEqFunc key_eq_fn;        // Custom key equality function (for TYPE_TAG_CUSTOM)
    ElemToStrFunc value_to_str_fn; // Function pointer for custom value types
} dict_t;

// Core operations - now accept generic keys
// For custom types, key_ptr is used instead of key_int/key_str/key_float
dict_t* dict_new(int key_type_tag, size_t key_size, size_t value_size, int value_type_tag, 
                 KeyHashFunc key_hash_fn, KeyEqFunc key_eq_fn, ElemToStrFunc value_to_str_fn);
void dict_free(dict_t* d);
void dict_insert(dict_t* d, int64_t key_int, const char* key_str, double key_float, 
                 void* key_ptr, const void* value, int value_type_tag);
void* dict_get(dict_t* d, int64_t key_int, const char* key_str, double key_float, 
               void* key_ptr, const void* default_val);
void* dict_setdefault(dict_t* d, int64_t key_int, const char* key_str, double key_float, 
                      void* key_ptr, const void* default_val, int value_type_tag);
bool dict_has_key(dict_t* d, int64_t key_int, const char* key_str, double key_float, void* key_ptr);
void* dict_pop(dict_t* d, int64_t key_int, const char* key_str, double key_float, void* key_ptr);
void dict_clear(dict_t* d);
int64_t dict_len(dict_t* d);

// Collection methods
char** dict_keys(dict_t* d, size_t* out_len);
void** dict_values(dict_t* d, size_t* out_len);

// Internal helpers
uint64_t dict_hash_int(int64_t key);
uint64_t dict_hash_str(const char* key);
uint64_t dict_hash_float(double key);

// String representation
char* dict_to_str(dict_t* d);

#endif // DESI_DICT_H
