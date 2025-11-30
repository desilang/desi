#ifndef DESI_LIST_H
#define DESI_LIST_H

#include <stddef.h>
#include <stdint.h>

// DesiList - Dynamic growable list (type-erased via void*)
typedef struct {
    void** data;       // Array of pointers to elements
    size_t length;     // Current number of elements
    size_t capacity;   // Total allocated capacity
} DesiList;

// Core list operations
DesiList* list_new(void);
void list_append(DesiList* list, void* item);
void* list_get(DesiList* list, int64_t index);
void list_set(DesiList* list, int64_t index, void* item);
int64_t list_len(DesiList* list);
DesiList* list_slice(DesiList* list, int64_t start, int64_t end);
void list_free(DesiList* list);

#endif // DESI_LIST_H
