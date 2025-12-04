#include "dict.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define INITIAL_BUCKET_COUNT 16
#define LOAD_FACTOR_THRESHOLD 0.75

// FNV-1a hash function for strings
uint64_t dict_hash(const char* key) {
    uint64_t hash = 14695981039346656037ULL;
    while (*key) {
        hash ^= (uint64_t)(unsigned char)(*key++);
        hash *= 1099511628211ULL;
    }
    return hash;
}

// Create a new dictionary
dict_t* dict_new(size_t value_size, int type_tag, ElemToStrFunc value_to_str_fn) {
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
    d->value_size = value_size;
    d->type_tag = type_tag;
    d->value_to_str_fn = value_to_str_fn;  // Store function pointer
    
    return d;
}

// Free all memory associated with dictionary
void dict_free(dict_t* d) {
    if (!d) return;
    
    for (size_t i = 0; i < d->bucket_count; i++) {
        dict_entry_t* entry = d->buckets[i];
        while (entry) {
            dict_entry_t* next = entry->next;
            free(entry->key);
            free(entry->value);
            free(entry);
            entry = next;
        }
    }
    
    free(d->buckets);
    free(d);
}

// Insert or update a key-value pair
void dict_insert(dict_t* d, const char* key, const void* value, int type_tag) {
    if (!d || !key) return;
    
    // Upgrade type tag if currently Int (0) and new tag is different
    // This handles empty dicts (initialized as 0) becoming String/Bool dicts
    if (d->type_tag == 0 && type_tag != 0) {
        d->type_tag = type_tag;
    }
    
    uint64_t hash = dict_hash(key);
    size_t index = hash % d->bucket_count;
    
    // Check if key already exists
    dict_entry_t* entry = d->buckets[index];
    while (entry) {
        if (strcmp(entry->key, key) == 0) {
            // Update existing value
            memcpy(entry->value, value, d->value_size);
            return;
        }
        entry = entry->next;
    }
    
    // Create new entry
    dict_entry_t* new_entry = malloc(sizeof(dict_entry_t));
    if (!new_entry) return;
    
    new_entry->key = strdup(key);
    new_entry->value = malloc(d->value_size);
    if (!new_entry->value) {
        free(new_entry->key);
        free(new_entry);
        return;
    }
    new_entry->value = malloc(d->value_size);
    if (!new_entry->value) {
        free(new_entry->key);
        free(new_entry);
        return;
    }
    memcpy(new_entry->value, value, d->value_size);
    
    // Insert at head of bucket chain
    new_entry->next = d->buckets[index];
    d->buckets[index] = new_entry;
    d->entry_count++;
}

// Get value for a key, or return default if not found
void* dict_get(dict_t* d, const char* key, const void* default_val) {
    if (!d || !key) return (void*)default_val;
    
    uint64_t hash = dict_hash(key);
    size_t index = hash % d->bucket_count;
    
    dict_entry_t* entry = d->buckets[index];
    while (entry) {
        if (strcmp(entry->key, key) == 0) {
            return entry->value;
        }
        entry = entry->next;
    }
    
    return (void*)default_val;
}

// Check if key exists in dictionary
bool dict_has_key(dict_t* d, const char* key) {
    if (!d || !key) return false;
    
    uint64_t hash = dict_hash(key);
    size_t index = hash % d->bucket_count;
    
    dict_entry_t* entry = d->buckets[index];
    while (entry) {
        if (strcmp(entry->key, key) == 0) {
            return true;
        }
        entry = entry->next;
    }
    
    return false;
}

// Remove and return value for a key (caller must free)
void* dict_pop(dict_t* d, const char* key) {
    if (!d || !key) return NULL;
    
    uint64_t hash = dict_hash(key);
    size_t index = hash % d->bucket_count;
    
    dict_entry_t* entry = d->buckets[index];
    dict_entry_t* prev = NULL;
    
    while (entry) {
        if (strcmp(entry->key, key) == 0) {
            // Found it - remove from chain
            if (prev) {
                prev->next = entry->next;
            } else {
                d->buckets[index] = entry->next;
            }
            
            void* value = entry->value;
            free(entry->key);
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
            free(entry->key);
            // Value is stored in entry->value buffer, freed with entry
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
            keys[k++] = entry->key;  // Note: shallow copy, don't free these!
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

// ========== String Representation ==========

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
            
            // Add key
            size_t key_len = strlen(entry->key);
            while (pos + key_len + 10 >= bufsize) {
                bufsize *= 2;
                buffer = (char*)realloc(buffer, bufsize);
            }
            
            memcpy(buffer + pos, entry->key, key_len);
            pos += key_len;
            buffer[pos++] = ':';
            buffer[pos++] = ' ';
            
            // Format key and value
            // Format value based on type
            char val_str[256];
            
            // Use function pointer if available (custom types)
            if (d->value_to_str_fn != NULL) {
                char* custom_str = d->value_to_str_fn(entry->value);
                if (custom_str) {
                    snprintf(val_str, sizeof(val_str), "%s", custom_str);
                } else {
                    snprintf(val_str, sizeof(val_str), "<null>");
                }
            } else if (d->type_tag == 1) { // String
                // Value is char* (pointer to string)
                char* str_val = *(char**)entry->value;
                snprintf(val_str, sizeof(val_str), "\"%s\"", str_val ? str_val : "null");
            } else if (d->type_tag == 2) { // Bool
                // Value is int64_t (0 or 1)
                int64_t bool_val = 0;
                if (d->value_size >= sizeof(int64_t)) {
                    memcpy(&bool_val, entry->value, sizeof(int64_t));
                }
                snprintf(val_str, sizeof(val_str), "%s", bool_val ? "true" : "false");
            } else if (d->type_tag == 3) { // Float
                // Value is double
                double float_val = 0.0;
                if (d->value_size >= sizeof(double)) {
                    memcpy(&float_val, entry->value, sizeof(double));
                }
                snprintf(val_str, sizeof(val_str), "%g", float_val);
            } else { // Int (0) or default
                // Value is int64_t
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
