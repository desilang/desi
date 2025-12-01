#include "list.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define LIST_INITIAL_CAPACITY 8

// Create a new empty list
DesiList* list_new(void) {
    DesiList* list = (DesiList*)malloc(sizeof(DesiList));
    if (!list) {
        fprintf(stderr, "list_new: malloc failed\n");
        exit(1);
    }
    
    list->data = (void**)malloc(LIST_INITIAL_CAPACITY * sizeof(void*));
    if (!list->data) {
        fprintf(stderr, "list_new: malloc failed for data\n");
        exit(1);
    }
    
    list->length = 0;
    list->capacity = LIST_INITIAL_CAPACITY;
    return list;
}

// Append an item to the end of the list
void list_append(DesiList* list, void* item) {
    if (!list) {
        fprintf(stderr, "list_append: null list\n");
        return;
    }
    
    // Grow if necessary
    if (list->length >= list->capacity) {
        size_t new_capacity = list->capacity * 2;
        void** new_data = (void**)realloc(list->data, new_capacity * sizeof(void*));
        if (!new_data) {
            fprintf(stderr, "list_append: realloc failed\n");
            exit(1);
        }
        list->data = new_data;
        list->capacity = new_capacity;
    }
    
    list->data[list->length] = item;
    list->length++;
}

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
    if (!list) {
        return 0;
    }
    return (int64_t)list->length;
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
    
    DesiList* result = list_new();
    for (int64_t i = start; i < end; i++) {
        list_append(result, list->data[i]);
    }
    
    return result;
}

// Free the list and its data array (does NOT free individual elements)
void list_free(DesiList* list) {
    if (!list) {
        return;
    }
    
    if (list->data) {
        free(list->data);
    }
    free(list);
}
