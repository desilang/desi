#include "dict.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <math.h>

#define INITIAL_BUCKET_COUNT 16
#define LOAD_FACTOR_THRESHOLD 0.75

// ==================== Hash Functions ====================

// Hash for int/bool keys
uint64_t dict_hash_int(int64_t key) {
    return (uint64_t)key;
}

// FNV-1a hash for string keys
uint64_t dict_hash_str(const char* key) {
    if (!key) return 0;
    uint64_t hash = 14695981039346656037ULL;
    while (*key) {
        hash ^= (uint64_t)(unsigned char)(*key++);
        hash *= 1099511628211ULL;
    }
    return hash;
}

// Bit-exact hash for float keys
uint64_t dict_hash_float(double key) {
    uint64_t bits;
    memcpy(&bits, &key, sizeof(bits));
    return bits;
}

// Type-dispatched hash
static uint64_t hash_key(dict_t* d, int64_t key_int, const char* key_str, double key_float, void* key_ptr) {
    switch (d->key_type_tag) {
        case TYPE_TAG_INT:
        case TYPE_TAG_BOOL:
            return dict_hash_int(key_int);
        case TYPE_TAG_STR:
            return dict_hash_str(key_str);
        case TYPE_TAG_FLOAT:
            return dict_hash_float(key_float);
        case TYPE_TAG_CUSTOM:
            if (d->key_hash_fn && key_ptr) {
                // User-provided hash function
                return d->key_hash_fn(key_ptr);
            }
            // Default: pointer-based hash (identity)
            return (uint64_t)(uintptr_t)key_ptr;
        default:
            return dict_hash_int(key_int);
    }
}

// Type-dispatched key equality
static bool keys_equal(dict_t* d, dict_entry_t* entry, int64_t key_int, const char* key_str, double key_float, void* key_ptr) {
    switch (d->key_type_tag) {
        case TYPE_TAG_INT:
        case TYPE_TAG_BOOL:
            return entry->key_int == key_int;
        case TYPE_TAG_STR:
            return entry->key_str && key_str && strcmp(entry->key_str, key_str) == 0;
        case TYPE_TAG_FLOAT:
            return memcmp(&entry->key_float, &key_float, sizeof(double)) == 0;
        case TYPE_TAG_CUSTOM:
            if (d->key_eq_fn && entry->key_ptr && key_ptr) {
                // User-provided equality function
                return d->key_eq_fn(entry->key_ptr, key_ptr);
            }
            // Default: pointer equality (identity)
            return entry->key_ptr == key_ptr;
        default:
            return entry->key_int == key_int;
    }
}

// ==================== Core Operations ====================

// Create a new dictionary
dict_t* dict_new(int key_type_tag, size_t key_size, size_t value_size, int value_type_tag,
                 KeyHashFunc key_hash_fn, KeyEqFunc key_eq_fn, ElemToStrFunc value_to_str_fn) {
    if (value_size == 0) {
        fprintf(stderr, "dict_new: value_size cannot be 0\n");
        return NULL;
    }
    
    dict_t* d = (dict_t*)malloc(sizeof(dict_t));
    if (!d) return NULL;
    
    d->buckets = calloc(INITIAL_BUCKET_COUNT, sizeof(dict_entry_t*));
    if (!d->buckets) {
        free(d);
        return NULL;
    }
    
    d->bucket_count = INITIAL_BUCKET_COUNT;
    d->entry_count = 0;
    d->key_size = key_size;
    d->value_size = value_size;
    d->key_type_tag = key_type_tag;
    d->value_type_tag = value_type_tag;
    d->key_hash_fn = key_hash_fn;
    d->key_eq_fn = key_eq_fn;
    d->value_to_str_fn = value_to_str_fn;
    
    return d;
}

// Free all memory associated with dictionary
void dict_free(dict_t* d) {
    if (!d) return;
    
    for (size_t i = 0; i < d->bucket_count; i++) {
        dict_entry_t* entry = d->buckets[i];
        while (entry) {
            dict_entry_t* next = entry->next;
            if (d->key_type_tag == TYPE_TAG_STR) {
                free(entry->key_str);
            } else if (d->key_type_tag == TYPE_TAG_CUSTOM) {
                // Only free if we own the key (value-based with copy)
                if (d->key_hash_fn && d->key_eq_fn) {
                    free(entry->key_ptr);
                }
                // Pointer-identity keys are not owned, don't free
            }
            free(entry->value);
            free(entry);
            entry = next;
        }
    }
    
    free(d->buckets);
    free(d);
}

// Insert or update a key-value pair
void dict_insert(dict_t* d, int64_t key_int, const char* key_str, double key_float,
                 void* key_ptr, const void* value, int value_type_tag) {
    if (!d) return;
    
    // Upgrade value type tag if needed
    if (d->value_type_tag == TYPE_TAG_INT && value_type_tag != TYPE_TAG_INT) {
        d->value_type_tag = value_type_tag;
    }
    
    uint64_t hash = hash_key(d, key_int, key_str, key_float, key_ptr);
    size_t index = hash % d->bucket_count;
    
    // Check if key already exists
    dict_entry_t* entry = d->buckets[index];
    while (entry) {
        if (keys_equal(d, entry, key_int, key_str, key_float, key_ptr)) {
            // Update existing value
            memcpy(entry->value, value, d->value_size);
            entry->value_type_tag = value_type_tag;
            return;
        }
        entry = entry->next;
    }
    
    // Create new entry
    dict_entry_t* new_entry = malloc(sizeof(dict_entry_t));
    if (!new_entry) return;
    
    // Store key based on type
    new_entry->key_int = key_int;
    new_entry->key_float = key_float;
    new_entry->key_ptr = NULL;
    new_entry->key_str = NULL;
    
    if (d->key_type_tag == TYPE_TAG_STR && key_str) {
        new_entry->key_str = strdup(key_str);
    } else if (d->key_type_tag == TYPE_TAG_CUSTOM && key_ptr) {
        // For custom keys with hash/eq functions (value-based), copy the data
        // For default pointer identity (no functions), store pointer directly
        if (d->key_hash_fn && d->key_eq_fn && d->key_size > 0) {
            // Value-based: copy the key data
            new_entry->key_ptr = malloc(d->key_size);
            if (new_entry->key_ptr) {
                memcpy(new_entry->key_ptr, key_ptr, d->key_size);
            }
        } else {
            // Pointer identity: store the pointer directly (no copy)
            new_entry->key_ptr = key_ptr;
        }
    }
    
    new_entry->value = malloc(d->value_size);
    if (!new_entry->value) {
        if (new_entry->key_str) free(new_entry->key_str);
        if (new_entry->key_ptr) free(new_entry->key_ptr);
        free(new_entry);
        return;
    }
    memcpy(new_entry->value, value, d->value_size);
    new_entry->value_type_tag = value_type_tag;
    
    // Insert at head of bucket chain
    new_entry->next = d->buckets[index];
    d->buckets[index] = new_entry;
    d->entry_count++;
}

// Get value for a key, or return default if not found
void* dict_get(dict_t* d, int64_t key_int, const char* key_str, double key_float,
               void* key_ptr, const void* default_val) {
    if (!d) return (void*)default_val;
    
    uint64_t hash = hash_key(d, key_int, key_str, key_float, key_ptr);
    size_t index = hash % d->bucket_count;
    
    dict_entry_t* entry = d->buckets[index];
    while (entry) {
        if (keys_equal(d, entry, key_int, key_str, key_float, key_ptr)) {
            return entry->value;
        }
        entry = entry->next;
    }
    
    return (void*)default_val;
}

// Get value for a key, or insert default if not found
void* dict_setdefault(dict_t* d, int64_t key_int, const char* key_str, double key_float,
                      void* key_ptr, const void* default_val, int value_type_tag) {
    if (!d) return (void*)default_val; // Or maybe create a new dict? No, robust failure.

    uint64_t hash = hash_key(d, key_int, key_str, key_float, key_ptr);
    size_t index = hash % d->bucket_count;

    // 1. Check if key exists
    dict_entry_t* entry = d->buckets[index];
    while (entry) {
        if (keys_equal(d, entry, key_int, key_str, key_float, key_ptr)) {
            return entry->value;
        }
        entry = entry->next;
    }

    // 2. Key not found - insert default (using existing insert logic would re-hash, let's just manual insert or call insert)
    // Calling dict_insert is safer/cleaner code reuse, even if it re-hashes (optimization: pass hash?)
    // dict_insert accepts explicit key params.
    dict_insert(d, key_int, key_str, key_float, key_ptr, default_val, value_type_tag);
    
    // 3. Return the inserted value. Since we copied it, we need to find it again?
    // Optimization: dict_insert doesn't return the pointer.
    // Let's optimize: Inlined insert logic to return pointer.
    // Actually for MVP, let's just call dict_insert then dict_get (or manual lookup).
    // Manual lookup is fast since we have the index/hash (if we didn't recompute).
    // But dict_insert might resize (not yet implemented fully? Wait, insert doesn't resize in this snippet)
    
    // Re-lookup to return the *stored* pointer (important for ownership/lifetime)
    // Or return default_val (which is what we passed)?
    // The convention is usually to return the value in the dict.
    // If we return default_val (stack ptr), it might go out of scope if caller expects internal ptr?
    // But primitives are by value. Pointers are pointers.
    // If I insert a string, I insert a COPY. I should return the COPY in the dict basically.
    // So I should return the value *in the dict*.
    
    // Re-lookup is safest MVP.
    // We already computed hash/index, but dict_insert might have added it.
    
    // For now, re-use dict_insert and then re-search.
    // Optimization later.
    
    // (Wait, dict_insert recomputes hash).
    
    index = hash % d->bucket_count; // Recompute index just in case? No, insert might have resized?
    // This implementation of dict_insert DOES NOT RESIZE.
    
    entry = d->buckets[index];
    // It's at the HEAD (L199: new_entry->next = d->buckets[index]; d->buckets[index] = new_entry;)
    // So checking head is sufficient!
    if (entry && keys_equal(d, entry, key_int, key_str, key_float, key_ptr)) {
         return entry->value;
    }
    
    // Fallback if something weird happened (should be unreachable)
    return (void*)default_val;
}

// Check if key exists in dictionary
bool dict_has_key(dict_t* d, int64_t key_int, const char* key_str, double key_float, void* key_ptr) {
    if (!d) return false;
    
    uint64_t hash = hash_key(d, key_int, key_str, key_float, key_ptr);
    size_t index = hash % d->bucket_count;
    
    dict_entry_t* entry = d->buckets[index];
    while (entry) {
        if (keys_equal(d, entry, key_int, key_str, key_float, key_ptr)) {
            return true;
        }
        entry = entry->next;
    }
    
    return false;
}

// Remove and return value for a key (caller must free)
void* dict_pop(dict_t* d, int64_t key_int, const char* key_str, double key_float, void* key_ptr) {
    if (!d) return NULL;
    
    uint64_t hash = hash_key(d, key_int, key_str, key_float, key_ptr);
    size_t index = hash % d->bucket_count;
    
    dict_entry_t* entry = d->buckets[index];
    dict_entry_t* prev = NULL;
    
    while (entry) {
        if (keys_equal(d, entry, key_int, key_str, key_float, key_ptr)) {
            // Found it - remove from chain
            if (prev) {
                prev->next = entry->next;
            } else {
                d->buckets[index] = entry->next;
            }
            
            void* value = entry->value;
            if (d->key_type_tag == TYPE_TAG_STR) {
                free(entry->key_str);
            } else if (d->key_type_tag == TYPE_TAG_CUSTOM) {
                // Only free if we own the key (value-based with copy)
                if (d->key_hash_fn && d->key_eq_fn) {
                    free(entry->key_ptr);
                }
            }
            free(entry);
            d->entry_count--;
            return value;
        }
        prev = entry;
        entry = entry->next;
    }
    
    return NULL;
}

// Clear all entries
void dict_clear(dict_t* d) {
    if (!d) return;
    
    for (size_t i = 0; i < d->bucket_count; i++) {
        dict_entry_t* entry = d->buckets[i];
        while (entry) {
            dict_entry_t* next = entry->next;
            if (d->key_type_tag == TYPE_TAG_STR) {
                free(entry->key_str);
            } else if (d->key_type_tag == TYPE_TAG_CUSTOM) {
                // Only free if we own the key (value-based with copy)
                if (d->key_hash_fn && d->key_eq_fn) {
                    free(entry->key_ptr);
                }
            }
            free(entry->value);
            free(entry);
            entry = next;
        }
        d->buckets[i] = NULL;
    }
    
    d->entry_count = 0;
}

int64_t dict_len(dict_t* d) {
    return d ? (int64_t)d->entry_count : 0;
}

// ==================== Collection Methods ====================

// Get all keys (caller must free the array)
char** dict_keys(dict_t* d, size_t* out_len) {
    if (!d || !out_len) return NULL;
    
    *out_len = d->entry_count;
    if (d->entry_count == 0) return NULL;
    
    char** keys = malloc(sizeof(char*) * d->entry_count);
    if (!keys) return NULL;
    
    size_t k = 0;
    for (size_t i = 0; i < d->bucket_count; i++) {
        dict_entry_t* entry = d->buckets[i];
        while (entry) {
            if (d->key_type_tag == TYPE_TAG_STR) {
                keys[k++] = entry->key_str;  // Note: shallow copy
            } else if (d->key_type_tag == TYPE_TAG_CUSTOM) {
                // For custom keys, return address as string (for now)
                char* buf = malloc(32);
                snprintf(buf, 32, "<obj@%p>", entry->key_ptr);
                keys[k++] = buf;
            } else {
                // For non-string keys, convert to string representation
                char* buf = malloc(32);
                if (d->key_type_tag == TYPE_TAG_INT) {
                    snprintf(buf, 32, "%lld", (long long)entry->key_int);
                } else if (d->key_type_tag == TYPE_TAG_BOOL) {
                    snprintf(buf, 32, "%s", entry->key_int ? "true" : "false");
                } else if (d->key_type_tag == TYPE_TAG_FLOAT) {
                    snprintf(buf, 32, "%g", entry->key_float);
                } else {
                    snprintf(buf, 32, "%lld", (long long)entry->key_int);
                }
                keys[k++] = buf;
            }
            entry = entry->next;
        }
    }
    
    return keys;
}

// Get all values (caller must free the array)
void** dict_values(dict_t* d, size_t* out_len) {
    if (!d || !out_len) return NULL;
    
    *out_len = d->entry_count;
    if (d->entry_count == 0) return NULL;
    
    void** values = malloc(sizeof(void*) * d->entry_count);
    if (!values) return NULL;
    
    size_t k = 0;
    for (size_t i = 0; i < d->bucket_count; i++) {
        dict_entry_t* entry = d->buckets[i];
        while (entry) {
            values[k++] = entry->value;  // Note: shallow copy
            entry = entry->next;
        }
    }
    
    return values;
}

// ==================== String Representation ====================

// Convert dict to string representation
char* dict_to_str(dict_t* d) {
    if (!d) {
        char* result = (char*)malloc(7);
        strcpy(result, "<null>");
        return result;
    }
    
    // Allocate initial buffer
    size_t bufsize = 256;
    char* buffer = (char*)malloc(bufsize);
    if (!buffer) {
        fprintf(stderr, "dict_to_str: malloc failed\n");
        exit(1);
    }
    
    size_t pos = 0;
    buffer[pos++] = '{';
    
    bool first = true;
    for (size_t i = 0; i < d->bucket_count; i++) {
        dict_entry_t* entry = d->buckets[i];
        while (entry) {
            if (!first) {
                // Ensure space for ", "
                if (pos + 2 >= bufsize) {
                    bufsize *= 2;
                    buffer = (char*)realloc(buffer, bufsize);
                }
                buffer[pos++] = ',';
                buffer[pos++] = ' ';
            }
            first = false;
            
            // Format key based on type
            char key_str[64];
            switch (d->key_type_tag) {
                case TYPE_TAG_INT:
                    snprintf(key_str, sizeof(key_str), "%lld", (long long)entry->key_int);
                    break;
                case TYPE_TAG_STR:
                    snprintf(key_str, sizeof(key_str), "%s", entry->key_str ? entry->key_str : "null");
                    break;
                case TYPE_TAG_BOOL:
                    snprintf(key_str, sizeof(key_str), "%s", entry->key_int ? "true" : "false");
                    break;
                case TYPE_TAG_FLOAT:
                    snprintf(key_str, sizeof(key_str), "%g", entry->key_float);
                    break;
                case TYPE_TAG_CUSTOM:
                    snprintf(key_str, sizeof(key_str), "<obj@%p>", entry->key_ptr);
                    break;
                default:
                    snprintf(key_str, sizeof(key_str), "%lld", (long long)entry->key_int);
            }
            
            size_t key_len = strlen(key_str);
            while (pos + key_len + 10 >= bufsize) {
                bufsize *= 2;
                buffer = (char*)realloc(buffer, bufsize);
            }
            
            memcpy(buffer + pos, key_str, key_len);
            pos += key_len;
            buffer[pos++] = ':';
            buffer[pos++] = ' ';
            
            // Format value based on per-entry type tag (supports Any-typed dicts)
            char val_str[256];
            int vtt = entry->value_type_tag;
            
            // Use function pointer if available (custom types)
            if (d->value_to_str_fn != NULL) {
                char* custom_str = d->value_to_str_fn(entry->value);
                if (custom_str) {
                    snprintf(val_str, sizeof(val_str), "%s", custom_str);
                } else {
                    snprintf(val_str, sizeof(val_str), "<null>");
                }
            } else if (vtt == TYPE_TAG_STR) {
                char* str_val = *(char**)entry->value;
                snprintf(val_str, sizeof(val_str), "\"%s\"", str_val ? str_val : "null");
            } else if (vtt == TYPE_TAG_BOOL) {
                int64_t bool_val = 0;
                if (d->value_size >= sizeof(int64_t)) {
                    memcpy(&bool_val, entry->value, sizeof(int64_t));
                }
                snprintf(val_str, sizeof(val_str), "%s", bool_val ? "true" : "false");
            } else if (vtt == TYPE_TAG_FLOAT) {
                double float_val = 0.0;
                if (d->value_size >= sizeof(double)) {
                    memcpy(&float_val, entry->value, sizeof(double));
                }
                snprintf(val_str, sizeof(val_str), "%g", float_val);
            } else {
                int64_t int_val = 0;
                if (d->value_size >= sizeof(int64_t)) {
                    memcpy(&int_val, entry->value, sizeof(int64_t));
                }
                snprintf(val_str, sizeof(val_str), "%lld", (long long)int_val);
            }
            size_t val_len = strlen(val_str);
            
            while (pos + val_len >= bufsize) {
                bufsize *= 2;
                buffer = (char*)realloc(buffer, bufsize);
            }
            
            memcpy(buffer + pos, val_str, val_len);
            pos += val_len;
            
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
