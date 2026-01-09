#include "list.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define LIST_INITIAL_CAPACITY 8

// ========== Core Operations ==========

// Create a new empty list
DesiList* list_new(int type_tag, ElemToStrFunc to_str_fn) {
    DesiList* list = (DesiList*)malloc(sizeof(DesiList));
    if (!list) {
        return NULL;
    }
    
    list->capacity = LIST_INITIAL_CAPACITY;
    list->length = 0;
    list->type_tag = type_tag;
    list->to_str_fn = to_str_fn;  // Store function pointer
    list->data = (void**)malloc(list->capacity * sizeof(void*));
    
    if (!list->data) {
        free(list);
        return NULL;
    }
    
    return list;
}

// Free the list and its data array (does NOT free individual elements - compiler handles that)
void list_free(DesiList* list) {
    if (!list) return;
    
    if (list->data) {
        free(list->data);
    }
    free(list);
}

// Clear all elements from the list (does NOT free elements - compiler handles that)
void list_clear(DesiList* list) {
    if (!list) return;
    list->length = 0;
}

// Create a shallow copy of the list
DesiList* list_copy(DesiList* list) {
    if (!list) return NULL;
    
    DesiList* result = list_new(list->type_tag, list->to_str_fn);  // Copy function pointer too
    
    // Ensure capacity
    if (list->length > result->capacity) {
        result->data = (void**)realloc(result->data, list->length * sizeof(void*));
        if (!result->data) {
            fprintf(stderr, "list_copy: realloc failed\n");
            exit(1);
        }
        result->capacity = list->length;
    }
    
    // Copy elements (shallow copy - pointers only)
    memcpy(result->data, list->data, list->length * sizeof(void*));
    result->length = list->length;
    
    return result;
}

// ========== Element Access ==========

// Get item at index (returns NULL if out of bounds)
void* list_get(DesiList* list, int64_t index) {
    if (!list) {
        fprintf(stderr, "list_get: null list\n");
        return NULL;
    }
    
    // Handle negative indices (Python-style)
    if (index < 0) {
        index = (int64_t)list->length + index;
    }
    
    if (index < 0 || index >= (int64_t)list->length) {
        fprintf(stderr, "list_get: index %lld out of bounds (len=%zu)\n", 
                (long long)index, list->length);
        return NULL;
    }
    
    return list->data[index];
}

// Set item at index
void list_set(DesiList* list, int64_t index, void* item) {
    if (!list) {
        fprintf(stderr, "list_set: null list\n");
        return;
    }
    
    // Handle negative indices
    if (index < 0) {
        index = (int64_t)list->length + index;
    }
    
    if (index < 0 || index >= (int64_t)list->length) {
        fprintf(stderr, "list_set: index %lld out of bounds (len=%zu)\n",
                (long long)index, list->length);
        return;
    }
    
    list->data[index] = item;
}

// Get the length of the list
int64_t list_len(DesiList* list) {
    if (!list) return 0;
    return (int64_t)list->length;
}

// ========== Modification ==========

// Ensure the list has capacity for at least new_capacity elements
static void list_ensure_capacity(DesiList* list, size_t new_capacity) {
    if (list->capacity >= new_capacity) return;
    
    size_t target = list->capacity;
    while (target < new_capacity) {
        target *= 2;
    }
    
    void** new_data = (void**)realloc(list->data, target * sizeof(void*));
    if (!new_data) {
        fprintf(stderr, "list_ensure_capacity: realloc failed\n");
        exit(1);
    }
    list->data = new_data;
    list->capacity = target;
}

// Append an item to the list
void list_append(DesiList* list, void* item, int type_tag) {
    if (!list) {
        fprintf(stderr, "list_append: null list\n");
        return;
    }
    
    // Upgrade type tag if currently Int (0) and new tag is different
    if (list->type_tag == 0 && type_tag != 0) {
        list->type_tag = type_tag;
    }
    
    list_ensure_capacity(list, list->length + 1);
    list->data[list->length] = item;
    list->length++;
}

// Extend list with all items from another list
void list_extend(DesiList* list, DesiList* other) {
    if (!list || !other) return;
    
    list_ensure_capacity(list, list->length + other->length);
    memcpy(list->data + list->length, other->data, other->length * sizeof(void*));
    list->length += other->length;
}

// Insert item at index (shifts elements right)
void list_insert(DesiList* list, int64_t index, void* item) {
    if (!list) return;
    
    // Handle negative indices
    if (index < 0) {
        index = (int64_t)list->length + index;
    }
    
    // Clamp to valid range [0, length]
    if (index < 0) index = 0;
    if (index > (int64_t)list->length) index = (int64_t)list->length;
    
    list_ensure_capacity(list, list->length + 1);
    
    // Shift elements right
    if (index < (int64_t)list->length) {
        memmove(list->data + index + 1, list->data + index, 
                (list->length - index) * sizeof(void*));
    }
    
    list->data[index] = item;
    list->length++;
}

// Remove and return item at index (-1 removes last)
void* list_pop(DesiList* list, int64_t index) {
    if (!list || list->length == 0) {
        fprintf(stderr, "list_pop: list is empty\n");
        return NULL;
    }
    
    // Default to last element
    if (index == -1) {
        index = (int64_t)list->length - 1;
    }
    
    // Handle negative indices
    if (index < 0) {
        index = (int64_t)list->length + index;
    }
    
    if (index < 0 || index >= (int64_t)list->length) {
        fprintf(stderr, "list_pop: index %lld out of bounds\n", (long long)index);
        return NULL;
    }
    
    void* item = list->data[index];
    
    // Shift elements left
    if (index < (int64_t)list->length - 1) {
        memmove(list->data + index, list->data + index + 1,
                (list->length - index - 1) * sizeof(void*));
    }
    
    list->length--;
    return item;
}

// Remove first occurrence of item (uses pointer equality)
void list_remove(DesiList* list, void* item) {
    if (!list) return;
    
    for (size_t i = 0; i < list->length; i++) {
        if (list->data[i] == item) {
            list_pop(list, (int64_t)i);
            return;
        }
    }
    
    fprintf(stderr, "list_remove: item not found in list\n");
}

// Reverse the list in-place
void list_reverse(DesiList* list) {
    if (!list || list->length <= 1) return;
    
    size_t left = 0;
    size_t right = list->length - 1;
    
    while (left < right) {
        void* temp = list->data[left];
        list->data[left] = list->data[right];
        list->data[right] = temp;
        left++;
        right--;
    }
}

// ========== Sorting ==========

// Comparison function for integers (stored as intptr_t)
static int cmp_int_asc(const void* a, const void* b) {
    intptr_t ia = (intptr_t)(*(void**)a);
    intptr_t ib = (intptr_t)(*(void**)b);
    return (ia > ib) - (ia < ib);
}

static int cmp_int_desc(const void* a, const void* b) {
    return -cmp_int_asc(a, b);
}

// Comparison function for strings
static int cmp_str_asc(const void* a, const void* b) {
    const char* sa = (const char*)(*(void**)a);
    const char* sb = (const char*)(*(void**)b);
    if (!sa && !sb) return 0;
    if (!sa) return -1;
    if (!sb) return 1;
    return strcmp(sa, sb);
}

static int cmp_str_desc(const void* a, const void* b) {
    return -cmp_str_asc(a, b);
}

// Comparison function for floats (stored as double bits in intptr_t)
static int cmp_float_asc(const void* a, const void* b) {
    double fa, fb;
    intptr_t ia = (intptr_t)(*(void**)a);
    intptr_t ib = (intptr_t)(*(void**)b);
    memcpy(&fa, &ia, sizeof(double));
    memcpy(&fb, &ib, sizeof(double));
    return (fa > fb) - (fa < fb);
}

static int cmp_float_desc(const void* a, const void* b) {
    return -cmp_float_asc(a, b);
}

// Sort list in-place
// reverse: 0 = ascending, 1 = descending
void list_sort(DesiList* list, int reverse) {
    if (!list || list->length <= 1) return;
    
    int (*cmp_func)(const void*, const void*) = NULL;
    
    switch (list->type_tag) {
        case 0: // int
            cmp_func = reverse ? cmp_int_desc : cmp_int_asc;
            break;
        case 1: // str
            cmp_func = reverse ? cmp_str_desc : cmp_str_asc;
            break;
        case 4: // float (type_tag 4 for float)
            cmp_func = reverse ? cmp_float_desc : cmp_float_asc;
            break;
        default:
            // For other types, sort by pointer value (stable but arbitrary)
            cmp_func = reverse ? cmp_int_desc : cmp_int_asc;
            break;
    }
    
    qsort(list->data, list->length, sizeof(void*), cmp_func);
}

// Create a slice from start (inclusive) to end (exclusive)
DesiList* list_slice(DesiList* list, int64_t start, int64_t end) {
    if (!list) {
        fprintf(stderr, "list_slice: null list\n");
        return NULL;
    }
    
    // Handle negative indices
    if (start < 0) start = (int64_t)list->length + start;
    if (end < 0) end = (int64_t)list->length + end;
    
    // Clamp to bounds
    if (start < 0) start = 0;
    if (end > (int64_t)list->length) end = (int64_t)list->length;
    if (start > end) start = end;
    
    DesiList* result = list_new(list->type_tag, list->to_str_fn);
    for (int64_t i = start; i < end; i++) {
        list_append(result, list->data[i], list->type_tag);
    }
    
    return result;
}

// Create a slice with step (Python-style)
// step > 0: forward iteration
// step < 0: reverse iteration (e.g., [::-1] reverses)
// Sentinel values: INT64_MAX for start means "use default", INT64_MIN for end means "use default"
#define SENTINEL_START 9223372036854775807LL
#define SENTINEL_END   (-9223372036854775807LL - 1)

DesiList* list_slice_step(DesiList* list, int64_t start, int64_t end, int64_t step) {
    if (!list) {
        fprintf(stderr, "list_slice_step: null list\n");
        return NULL;
    }
    
    if (step == 0) {
        fprintf(stderr, "list_slice_step: step cannot be zero\n");
        return NULL;
    }
    
    int64_t len = (int64_t)list->length;
    
    // Handle sentinel values (omitted start/end)
    if (step > 0) {
        // Forward: start defaults to 0, end defaults to len
        if (start == SENTINEL_START) start = 0;
        if (end == SENTINEL_END) end = len;
        // Handle negative indices
        if (start < 0) start = len + start;
        if (end < 0) end = len + end;
        if (start < 0) start = 0;
        if (end > len) end = len;
    } else {
        // Reverse: start defaults to len-1, end defaults to -1 (before first)
        if (start == SENTINEL_START) start = len - 1;
        if (end == SENTINEL_END) end = -1;
        // Handle negative indices
        if (start < 0) start = len + start;
        if (end < -1 && end != SENTINEL_END) end = len + end;
        if (start >= len) start = len - 1;
    }
    
    DesiList* result = list_new(list->type_tag, list->to_str_fn);
    
    if (step > 0) {
        for (int64_t i = start; i < end; i += step) {
            if (i >= 0 && i < len) {
                list_append(result, list->data[i], list->type_tag);
            }
        }
    } else {
        for (int64_t i = start; i > end; i += step) {
            if (i >= 0 && i < len) {
                list_append(result, list->data[i], list->type_tag);
            }
        }
    }
    
    return result;
}

// Find index of first occurrence of item in range [start, end)
int64_t list_index(DesiList* list, void* item, int64_t start, int64_t end) {
    if (!list) return -1;
    
    // Handle negative indices
    if (start < 0) start = (int64_t)list->length + start;
    if (end < 0) end = (int64_t)list->length + end;
    
    // Clamp to bounds
    if (start < 0) start = 0;
    if (end > (int64_t)list->length) end = (int64_t)list->length;
    
    for (int64_t i = start; i < end; i++) {
        if (list->data[i] == item) {
            return i;
        }
    }
    
    return -1; // Not found
}

// Count occurrences of item
int64_t list_count(DesiList* list, void* item) {
    if (!list) return 0;
    
    int64_t count = 0;
    for (size_t i = 0; i < list->length; i++) {
        if (list->data[i] == item) {
            count++;
        }
    }
    
    return count;
}

// Check if list contains item
bool list_contains(DesiList* list, void* item) {
    return list_index(list, item, 0, list ? (int64_t)list->length : 0) != -1;
}

// ========== Functional Operations ==========

// Map a function over all elements
DesiList* list_map(DesiList* list, MapFunc func) {
    if (!list || !func) return NULL;
    
    DesiList* result = list_new(list->type_tag, list->to_str_fn); // Preserve type tag? Or unknown?
    // Map can change type. For Tier-0, let's assume it preserves or becomes unknown (3).
    // But we don't know the return type of func.
    // Let's use 3 (Unknown) if we can't be sure, or propagate.
    // If we propagate, we might print wrong.
    // But map is generic.
    // Let's propagate for now, assuming map(int->int) or map(str->str).
    if (!result) return NULL;
    
    for (size_t i = 0; i < list->length; i++) {
        list_append(result, func(list->data[i]), list->type_tag);
    }
    
    return result;
}

// Filter elements that match predicate
DesiList* list_filter(DesiList* list, FilterFunc func) {
    if (!list || !func) return NULL;
    
    DesiList* result = list_new(list->type_tag, list->to_str_fn);
    if (!result) return NULL;
    
    for (size_t i = 0; i < list->length; i++) {
        if (func(list->data[i])) {
            list_append(result, list->data[i], list->type_tag);
        }
    }
    
    return result;
}

// Reduce list to single value using accumulator
void* list_reduce(DesiList* list, ReduceFunc func, void* initial) {
    if (!list || !func) return initial;
    
    void* accumulator = initial;
    for (size_t i = 0; i < list->length; i++) {
        accumulator = func(accumulator, list->data[i]);
    }
    
    return accumulator;
}

// Check if any element satisfies predicate
bool list_any(DesiList* list, FilterFunc predicate) {
    if (!list || !predicate) return false;
    
    for (size_t i = 0; i < list->length; i++) {
        if (predicate(list->data[i])) {
            return true;
        }
    }
    
    return false;
}

// Check if all elements satisfy predicate
bool list_all(DesiList* list, FilterFunc predicate) {
    if (!list || !predicate) return true; // Empty list = all true
    
    for (size_t i = 0; i < list->length; i++) {
        if (!predicate(list->data[i])) {
            return false;
        }
    }
    
    return true;
}

// ========== String Representation ==========

// Convert list to string representation
// Note: This is a simple implementation that treats pointers as integers
// for small values (< 1000000), suitable for demo purposes
char* list_to_str(DesiList* list) {
    if (!list) {
        char* result = (char*)malloc(7);
        strcpy(result, "<null>");
        return result;
    }
    
    // Allocate initial buffer
    size_t bufsize = 256;
    char* buffer = (char*)malloc(bufsize);
    if (!buffer) {
        fprintf(stderr, "list_to_str: malloc failed\n");
        exit(1);
    }
    
    size_t pos = 0;
    buffer[pos++] = '[';
    
    for (size_t i = 0; i < list->length; i++) {
        if (i > 0) {
            // Ensure space for ", "
            if (pos + 2 >= bufsize) {
                bufsize *= 2;
                buffer = (char*)realloc(buffer, bufsize);
                if (!buffer) {
                    fprintf(stderr, "list_to_str: realloc failed\n");
                    exit(1);
                }
            }
            buffer[pos++] = ',';
            buffer[pos++] = ' ';
        }
        
        // Convert element to string
        char temp[256];
        void* item = list->data[i];
        
        // Use function pointer if available (custom types)
        if (list->to_str_fn != NULL) {
            char* custom_str = list->to_str_fn(item);
            if (custom_str) {
                snprintf(temp, sizeof(temp), "%s", custom_str);
                // Note: Assuming to_str_fn returns malloc'd string, don't free it here
            } else {
                snprintf(temp, sizeof(temp), "<null>");
            }
        } else if (list->type_tag == 1) { // String
            // Item is char*
            snprintf(temp, sizeof(temp), "\"%s\"", (char*)item ? (char*)item : "null");
        } else if (list->type_tag == 2) { // Bool
            // Item is intptr_t (0 or 1)
            snprintf(temp, sizeof(temp), "%s", (intptr_t)item ? "true" : "false");
        } else { // Int (0) or default
            // Item is intptr_t
            snprintf(temp, sizeof(temp), "%lld", (long long)(intptr_t)item);
        }
        size_t len = strlen(temp);
        
        // Ensure space
        while (pos + len >= bufsize) {
            bufsize *= 2;
            buffer = (char*)realloc(buffer, bufsize);
            if (!buffer) {
                fprintf(stderr, "list_to_str: realloc failed\n");
                exit(1);
            }
        }
        
        memcpy(buffer + pos, temp, len);
        pos += len;
    }
    
    // Ensure space for ]
    if (pos + 2 >= bufsize) {
        bufsize = pos + 2;
        buffer = (char*)realloc(buffer, bufsize);
        if (!buffer) {
            fprintf(stderr, "list_to_str: realloc failed\n");
            exit(1);
        }
    }
    
    buffer[pos++] = ']';
    buffer[pos] = '\0';
    
    return buffer;
}
