// Iterator runtime for lazy iteration
// Stack-allocated iterator structs, no heap allocation for iterators themselves
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include "list.h"

// ========== Iterator Structs ==========
// These are small, stack-allocatable structs

// ListIter: iterates over a list
typedef struct {
    DesiList* list;
    int64_t index;
} ListIter;

// MapIter: transforms elements from source iterator
typedef struct {
    void* source;           // Source iterator (any type)
    void* (*next_fn)(void*); // Source's next function
    void* (*map_fn)(void*);  // Transform function
} MapIter;

// FilterIter: filters elements from source iterator
typedef struct {
    void* source;
    void* (*next_fn)(void*);
    bool (*pred_fn)(void*);  // Predicate function
} FilterIter;

// TakeIter: takes first N elements
typedef struct {
    void* source;
    void* (*next_fn)(void*);
    int64_t remaining;
} TakeIter;

// ========== ListIter Operations ==========

// Create iterator from list (returns stack-allocated struct by value)
ListIter list_iter_new(DesiList* list) {
    ListIter iter;
    iter.list = list;
    iter.index = 0;
    return iter;
}

// Initialize a pre-allocated iterator (for stack allocation from LLVM)
void list_iter_init(ListIter* iter, DesiList* list) {
    if (!iter) return;
    iter->list = list;
    iter->index = 0;
}

// Get next element, returns NULL when exhausted
// Returns boxed element pointer, or NULL
void* list_iter_next(ListIter* iter) {
    if (!iter || !iter->list) return NULL;
    
    if (iter->index >= (int64_t)iter->list->length) {
        return NULL;  // Exhausted
    }
    
    void* elem = iter->list->data[iter->index];
    iter->index++;
    return elem;
}

// Check if iterator has more elements
bool list_iter_has_next(ListIter* iter) {
    if (!iter || !iter->list) return false;
    return iter->index < (int64_t)iter->list->length;
}

// ========== MapIter Operations ==========

// Create map iterator (wraps any iterator + transform function)
MapIter map_iter_new(void* source, void* (*next_fn)(void*), void* (*map_fn)(void*)) {
    MapIter iter;
    iter.source = source;
    iter.next_fn = next_fn;
    iter.map_fn = map_fn;
    return iter;
}

// Get next transformed element
void* map_iter_next(MapIter* iter) {
    if (!iter || !iter->next_fn || !iter->map_fn) return NULL;
    
    void* elem = iter->next_fn(iter->source);
    if (!elem) return NULL;  // Source exhausted
    
    return iter->map_fn(elem);  // Apply transform
}

// ========== FilterIter Operations ==========

// Create filter iterator
FilterIter filter_iter_new(void* source, void* (*next_fn)(void*), bool (*pred_fn)(void*)) {
    FilterIter iter;
    iter.source = source;
    iter.next_fn = next_fn;
    iter.pred_fn = pred_fn;
    return iter;
}

// Get next matching element
void* filter_iter_next(FilterIter* iter) {
    if (!iter || !iter->next_fn || !iter->pred_fn) return NULL;
    
    while (true) {
        void* elem = iter->next_fn(iter->source);
        if (!elem) return NULL;  // Source exhausted
        
        if (iter->pred_fn(elem)) {
            return elem;  // Found match
        }
        // Keep searching
    }
}

// ========== TakeIter Operations ==========

TakeIter take_iter_new(void* source, void* (*next_fn)(void*), int64_t count) {
    TakeIter iter;
    iter.source = source;
    iter.next_fn = next_fn;
    iter.remaining = count;
    return iter;
}

void* take_iter_next(TakeIter* iter) {
    if (!iter || !iter->next_fn || iter->remaining <= 0) return NULL;
    
    void* elem = iter->next_fn(iter->source);
    if (elem) {
        iter->remaining--;
    }
    return elem;
}

// ========== Collecting ==========

// Collect iterator into new list
// Generic: works with any iterator that has a next function
DesiList* iter_collect(void* iter, void* (*next_fn)(void*), int type_tag) {
    DesiList* result = list_new(type_tag, NULL);

    while (true) {
        void* elem = next_fn(iter);
        if (!elem) break;
        // Owned elements (float boxes) must be cloned — the source
        // list still owns the originals and will free them.
        list_append(result, list_clone_elem(elem, type_tag), type_tag);
    }

    return result;
}

// Specialized collect for ListIter - simpler interface
DesiList* list_iter_collect(ListIter* iter, int type_tag) {
    DesiList* result = list_new(type_tag, NULL);

    while (true) {
        void* elem = list_iter_next(iter);
        if (!elem) break;
        list_append(result, list_clone_elem(elem, type_tag), type_tag);
    }

    return result;
}

// ========== Short-circuit Operations ==========

// Get first element that matches (stops iteration early)
void* iter_first(void* iter, void* (*next_fn)(void*)) {
    return next_fn(iter);  // Just get first element
}

// Check if any element is truthy (short-circuits on first true)
bool iter_any(void* iter, void* (*next_fn)(void*)) {
    while (true) {
        void* elem = next_fn(iter);
        if (!elem) return false;
        if ((intptr_t)elem) return true;
    }
}

// Check if all elements are truthy (short-circuits on first false)
bool iter_all(void* iter, void* (*next_fn)(void*)) {
    while (true) {
        void* elem = next_fn(iter);
        if (!elem) return true;  // Exhausted - all were truthy
        if (!(intptr_t)elem) return false;
    }
}
