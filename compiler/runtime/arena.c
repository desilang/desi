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

// How many nested marks an arena can record. Marks come from loop nesting, so
// this is a limit on how deeply loops that qualify for per-iteration rewind can
// nest within one function. Past it, marking still counts depth (so mark and
// rewind stay paired) but records nothing, and the matching rewind does nothing
// — the arena simply keeps growing to function exit, which is what it did
// before rewinding existed.
#define ARENA_MARK_STACK 32

typedef struct {
    ArenaChunk* chunk;  // chunk that was current when the mark was taken
    size_t offset;      // its bump position at that moment
} ArenaMark;

typedef struct {
    ArenaChunk* head;    // First chunk
    ArenaChunk* current; // Current chunk for allocations
    ArenaMark marks[ARENA_MARK_STACK];
    int depth;           // marks taken; may exceed ARENA_MARK_STACK
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
    arena->depth = 0;
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
    
    // A rewind (or a reset) leaves the chunks past the mark linked and empty,
    // so look for room in those before asking the allocator for more. This is
    // what makes a rewinding loop cost one round of growth rather than one per
    // iteration — and without it, the link step below would overwrite
    // chunk->next and strand every chunk after it.
    for (ArenaChunk* k = chunk->next; k; k = k->next) {
        if (k->offset + aligned_size <= k->capacity) {
            arena->current = k;
            void* ptr = k->buffer + k->offset;
            k->offset += aligned_size;
            return ptr;
        }
        chunk = k; // walk to the tail so the new chunk links onto the end
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
    arena->depth = 0;
}

// Record the current bump position so a later rewind can return to it.
//
// A loop body's allocations can be released at the end of each iteration, but
// only those: the arena also holds whatever the function allocated before the
// loop started, and a plain reset would take that with it. Marking at loop
// entry and rewinding at the latch releases exactly one iteration's worth.
//
// Marks nest, so the arena keeps a stack of them. Overflowing that stack is not
// an error: depth keeps counting so that every mark still has exactly one
// matching rewind, and the un-recorded levels simply do not reclaim anything.
void __arena_mark(DesiArena* arena) {
    if (!arena) return;
    if (arena->depth >= 0 && arena->depth < ARENA_MARK_STACK) {
        arena->marks[arena->depth].chunk = arena->current;
        arena->marks[arena->depth].offset = arena->current ? arena->current->offset : 0;
    }
    arena->depth++;
}

// Release everything allocated since the innermost mark, leaving that mark in
// place so it can be rewound to again.
//
// This is what a loop latch calls. One mark is taken before the loop and the
// latch returns to it on every pass, so the mark has to survive being used --
// pairing a push with each iteration instead would leave a stray mark behind
// every time the loop was left by `break`.
//
// Chunks past the mark keep their memory and stay linked, with their offsets
// zeroed; __arena_alloc walks them before allocating more. A loop therefore
// grows the arena at most once and reuses the same bytes on every later
// iteration, which is the whole point.
void __arena_rewind(DesiArena* arena) {
    if (!arena || arena->depth <= 0) return;
    int top = arena->depth - 1;
    if (top >= ARENA_MARK_STACK) return; // this level was never recorded

    ArenaChunk* marked = arena->marks[top].chunk;
    if (!marked) return;

    size_t back_to = arena->marks[top].offset;
#ifdef DESI_ARENA_POISON
    /* Build with -DDESI_ARENA_POISON to scribble over everything a rewind
     * releases. AddressSanitizer cannot help here: the arena is one large
     * malloc'd buffer, so handing the same bytes out twice is invisible to it.
     * Poisoning makes a read of released memory produce obvious garbage
     * instead of the value that happened to still be sitting there, which is
     * what turns a use-after-free into a failing test rather than a run that
     * happens to work. */
    if (marked->offset > back_to) {
        memset(marked->buffer + back_to, 0xDD, marked->offset - back_to);
    }
    for (ArenaChunk* k = marked->next; k; k = k->next) {
        if (k->offset) memset(k->buffer, 0xDD, k->offset);
    }
#endif

    marked->offset = back_to;
    for (ArenaChunk* k = marked->next; k; k = k->next) {
        k->offset = 0;
    }
    arena->current = marked;
}

// Rewind and drop the mark. This is what a loop's exit block calls, so the
// mark is retired however the loop was left -- running out of iterations or
// breaking out of the middle. Leaving by `return` skips it, but that path
// destroys the whole arena on the way out anyway.
void __arena_release(DesiArena* arena) {
    if (!arena || arena->depth <= 0) return;
    __arena_rewind(arena);
    arena->depth--;
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
