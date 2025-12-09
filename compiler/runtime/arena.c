// Arena allocator for region-based memory management
// All allocations from an arena are freed at once when the arena is destroyed
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#define ARENA_DEFAULT_CAPACITY (64 * 1024)  // 64KB default
#define ARENA_ALIGNMENT 8  // 8-byte alignment for all allocations

typedef struct ArenaChunk {
    char* buffer;
    size_t capacity;
    size_t offset;
    struct ArenaChunk* next;  // For grow-on-demand
} ArenaChunk;

typedef struct {
    ArenaChunk* head;    // First chunk
    ArenaChunk* current; // Current chunk for allocations
} DesiArena;

// Align size up to ARENA_ALIGNMENT boundary
static inline size_t align_up(size_t size) {
    return (size + ARENA_ALIGNMENT - 1) & ~(ARENA_ALIGNMENT - 1);
}

// Create a new chunk with given capacity
static ArenaChunk* arena_chunk_new(size_t capacity) {
    ArenaChunk* chunk = (ArenaChunk*)malloc(sizeof(ArenaChunk));
    if (!chunk) return NULL;
    
    chunk->buffer = (char*)malloc(capacity);
    if (!chunk->buffer) {
        free(chunk);
        return NULL;
    }
    
    chunk->capacity = capacity;
    chunk->offset = 0;
    chunk->next = NULL;
    return chunk;
}

// Create a new arena with default capacity
DesiArena* __arena_new(size_t initial_capacity) {
    if (initial_capacity == 0) {
        initial_capacity = ARENA_DEFAULT_CAPACITY;
    }
    
    DesiArena* arena = (DesiArena*)malloc(sizeof(DesiArena));
    if (!arena) return NULL;
    
    arena->head = arena_chunk_new(initial_capacity);
    if (!arena->head) {
        free(arena);
        return NULL;
    }
    
    arena->current = arena->head;
    return arena;
}

// Allocate memory from the arena (bump allocator)
void* __arena_alloc(DesiArena* arena, size_t size) {
    if (!arena || !arena->current) return NULL;
    
    size_t aligned_size = align_up(size);
    
    // Try current chunk
    ArenaChunk* chunk = arena->current;
    if (chunk->offset + aligned_size <= chunk->capacity) {
        void* ptr = chunk->buffer + chunk->offset;
        chunk->offset += aligned_size;
        return ptr;
    }
    
    // Need a new chunk - make it at least as big as requested
    size_t new_capacity = chunk->capacity * 2;
    if (new_capacity < aligned_size) {
        new_capacity = aligned_size + ARENA_DEFAULT_CAPACITY;
    }
    
    ArenaChunk* new_chunk = arena_chunk_new(new_capacity);
    if (!new_chunk) return NULL;
    
    // Link and use new chunk
    chunk->next = new_chunk;
    arena->current = new_chunk;
    
    void* ptr = new_chunk->buffer + new_chunk->offset;
    new_chunk->offset += aligned_size;
    return ptr;
}

// Allocate and zero memory from the arena
void* __arena_calloc(DesiArena* arena, size_t count, size_t size) {
    size_t total = count * size;
    void* ptr = __arena_alloc(arena, total);
    if (ptr) {
        memset(ptr, 0, total);
    }
    return ptr;
}

// Reset arena for reuse (keeps allocated chunks but resets offsets)
void __arena_reset(DesiArena* arena) {
    if (!arena) return;
    
    ArenaChunk* chunk = arena->head;
    while (chunk) {
        chunk->offset = 0;
        chunk = chunk->next;
    }
    arena->current = arena->head;
}

// Destroy the arena and free all memory at once
void __arena_destroy(DesiArena* arena) {
    if (!arena) return;
    
    ArenaChunk* chunk = arena->head;
    while (chunk) {
        ArenaChunk* next = chunk->next;
        free(chunk->buffer);
        free(chunk);
        chunk = next;
    }
    
    free(arena);
}

// Get current usage statistics
size_t __arena_used(DesiArena* arena) {
    if (!arena) return 0;
    
    size_t total = 0;
    ArenaChunk* chunk = arena->head;
    while (chunk) {
        total += chunk->offset;
        chunk = chunk->next;
    }
    return total;
}

// Get total capacity (all chunks)
size_t __arena_capacity(DesiArena* arena) {
    if (!arena) return 0;
    
    size_t total = 0;
    ArenaChunk* chunk = arena->head;
    while (chunk) {
        total += chunk->capacity;
        chunk = chunk->next;
    }
    return total;
}
