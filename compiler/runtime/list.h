#ifndef DESI_LIST_H
#define DESI_LIST_H

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

// Function pointer type for element-to-string conversion
typedef char* (*ElemToStrFunc)(void*);

// DesiList - Dynamic growable list (type-erased via void*)
typedef struct {
    void** data;       // Array of pointers to elements
    size_t length;     // Current number of elements
    size_t capacity;   // Total allocated capacity
    int type_tag;      // 0=int, 1=str, 2=bool, 3=other
    ElemToStrFunc to_str_fn;   // Function pointer for custom types
} DesiList;

// === Core Operations ===
DesiList* list_new(int type_tag, ElemToStrFunc to_str_fn);
void list_free(DesiList* list);
void list_clear(DesiList* list);
DesiList* list_copy(DesiList* list);
// Clone an element if the tag marks it as list-owned (float boxes, tag 3);
// other tags pass through unchanged. Use when elements flow between lists.
void* list_clone_elem(void* item, int type_tag);

// === Element Access ===
void* list_get(DesiList* list, int64_t index);
void list_set(DesiList* list, int64_t index, void* item);
int64_t list_len(DesiList* list);

// === Modification ===
void list_append(DesiList* list, void* item, int type_tag);
void list_extend(DesiList* list, DesiList* other);
void list_insert(DesiList* list, int64_t index, void* item);
void* list_pop(DesiList* list, int64_t index);
void list_remove(DesiList* list, void* item);  // Remove first occurrence
void list_reverse(DesiList* list);
void list_sort(DesiList* list, int reverse);  // reverse: 0=asc, 1=desc

// === Slicing & Search ===
DesiList* list_slice(DesiList* list, int64_t start, int64_t end);
int64_t list_index(DesiList* list, void* item, int64_t start, int64_t end);
int64_t list_count(DesiList* list, void* item);
int list_contains(DesiList* list, void* item);

// === Functional Operations ===
// Note: These require function pointers for callbacks
typedef void* (*MapFunc)(void*);
typedef bool (*FilterFunc)(void*);
typedef void* (*ReduceFunc)(void*, void*);

DesiList* list_map(DesiList* list, MapFunc func);
DesiList* list_filter(DesiList* list, FilterFunc func);
void* list_reduce(DesiList* list, ReduceFunc func, void* initial);
int list_any(DesiList* list, FilterFunc predicate);
int list_all(DesiList* list, FilterFunc predicate);

// === String Representation ===
char* list_to_str(DesiList* list);

#endif // DESI_LIST_H
