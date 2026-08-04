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
    void* arena;       // Optional arena handle (Task 4)
} DesiList;

// The compiler reads these fields directly.
//
// The backend emits the bounds check and the load for `xs[i]` inline, using
// these offsets as literals, and calls into this file only when the check
// fails. That makes the layout an ABI between the runtime and the code
// generator rather than a private detail: reordering these fields, or changing
// one's width, silently changes what generated programs load.
//
// These assertions fail the build if that happens. The matching constants live
// in compiler/internal/backend/llvm/list_layout.go, and a test compares the two
// so a change on either side is caught rather than mis-compiled.
_Static_assert(offsetof(DesiList, data) == 0,
    "backend emits a bare load for DesiList.data; it must stay first");
_Static_assert(offsetof(DesiList, length) == 8,
    "backend emits getelementptr i8 +8 for DesiList.length");
_Static_assert(offsetof(DesiList, capacity) == 16,
    "backend emits getelementptr i8 +16 for DesiList.capacity");
_Static_assert(sizeof(((DesiList*)0)->length) == 8,
    "backend loads DesiList.length as i64");
_Static_assert(sizeof(((DesiList*)0)->capacity) == 8,
    "backend loads DesiList.capacity as i64");

// === Core Operations ===
DesiList* list_new(int type_tag, ElemToStrFunc to_str_fn);
DesiList* list_new_in(void* arena, int type_tag, ElemToStrFunc to_str_fn);
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
