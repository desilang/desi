// Iterator types and function declarations
#ifndef DESI_ITERATOR_H
#define DESI_ITERATOR_H

#include <stdint.h>
#include <stdbool.h>
#include "list.h"

// Iterator structs (stack-allocated, no heap for iterators)
typedef struct {
    DesiList* list;
    int64_t index;
} ListIter;

typedef struct {
    void* source;
    void* (*next_fn)(void*);
    void* (*map_fn)(void*);
} MapIter;

typedef struct {
    void* source;
    void* (*next_fn)(void*);
    bool (*pred_fn)(void*);
} FilterIter;

typedef struct {
    void* source;
    void* (*next_fn)(void*);
    int64_t remaining;
} TakeIter;

// ListIter operations
ListIter list_iter_new(DesiList* list);
void list_iter_init(ListIter* iter, DesiList* list);
void* list_iter_next(ListIter* iter);
bool list_iter_has_next(ListIter* iter);

// MapIter operations  
MapIter map_iter_new(void* source, void* (*next_fn)(void*), void* (*map_fn)(void*));
void* map_iter_next(MapIter* iter);

// FilterIter operations
FilterIter filter_iter_new(void* source, void* (*next_fn)(void*), bool (*pred_fn)(void*));
void* filter_iter_next(FilterIter* iter);

// TakeIter operations
TakeIter take_iter_new(void* source, void* (*next_fn)(void*), int64_t count);
void* take_iter_next(TakeIter* iter);

// Collecting and short-circuit
DesiList* iter_collect(void* iter, void* (*next_fn)(void*), int type_tag);
DesiList* list_iter_collect(ListIter* iter, int type_tag);
void* iter_first(void* iter, void* (*next_fn)(void*));
bool iter_any(void* iter, void* (*next_fn)(void*));
bool iter_all(void* iter, void* (*next_fn)(void*));

#endif
