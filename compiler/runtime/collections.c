/*
 * collections.c — Advanced data structures for Desi stdlib
 *
 * Provides: Deque (double-ended queue), Stack, Queue, Counter.
 * All operate on string values for simplicity (Desi runtime strings).
 *
 * Public API:
 *   Deque:
 *     __deque_new()                    → create deque
 *     __deque_push_front(dq, val)      → add to front
 *     __deque_push_back(dq, val)       → add to back
 *     __deque_pop_front(dq)            → remove from front
 *     __deque_pop_back(dq)             → remove from back
 *     __deque_peek_front(dq)           → view front
 *     __deque_peek_back(dq)            → view back
 *     __deque_len(dq)                  → size
 *     __deque_is_empty(dq)             → empty check
 *     __deque_free(dq)                 → destroy
 *
 *   Stack:
 *     __stack_new()                    → create stack (LIFO)
 *     __stack_push(s, val)             → push value
 *     __stack_pop(s)                   → pop value
 *     __stack_peek(s)                  → view top
 *     __stack_len(s)                   → size
 *     __stack_is_empty(s)              → empty check
 *     __stack_free(s)                  → destroy
 *
 *   Queue:
 *     __queue_new()                    → create queue (FIFO)
 *     __queue_enqueue(q, val)          → add to back
 *     __queue_dequeue(q)               → remove from front
 *     __queue_peek(q)                  → view front
 *     __queue_len(q)                   → size
 *     __queue_is_empty(q)              → empty check
 *     __queue_free(q)                  → destroy
 *
 *   Counter:
 *     __counter_new()                  → create counter
 *     __counter_add(c, val)            → increment count for val
 *     __counter_add_n(c, val, n)       → add n to count for val
 *     __counter_get(c, val)            → get count
 *     __counter_most_common(c, n)      → top n items as "key:count" pairs
 *     __counter_total(c)               → sum of all counts
 *     __counter_len(c)                 → number of unique items
 *     __counter_free(c)                → destroy
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

/* ============================================================
 * Deque — circular buffer implementation
 * ============================================================ */

typedef struct {
    char** items;
    int    head;
    int    tail;
    int    cap;
    int    len;
} DesiDeque;

DesiDeque* __deque_new(void) {
    DesiDeque* dq = (DesiDeque*)calloc(1, sizeof(DesiDeque));
    dq->cap = 16;
    dq->items = (char**)calloc(dq->cap, sizeof(char*));
    return dq;
}

static void deque_grow(DesiDeque* dq) {
    int new_cap = dq->cap * 2;
    char** new_items = (char**)calloc(new_cap, sizeof(char*));
    for (int i = 0; i < dq->len; i++) {
        new_items[i] = dq->items[(dq->head + i) % dq->cap];
    }
    free(dq->items);
    dq->items = new_items;
    dq->head = 0;
    dq->tail = dq->len;
    dq->cap = new_cap;
}

void __deque_push_front(DesiDeque* dq, const char* val) {
    if (dq->len >= dq->cap) deque_grow(dq);
    dq->head = (dq->head - 1 + dq->cap) % dq->cap;
    dq->items[dq->head] = strdup(val ? val : "");
    dq->len++;
}

void __deque_push_back(DesiDeque* dq, const char* val) {
    if (dq->len >= dq->cap) deque_grow(dq);
    dq->items[dq->tail] = strdup(val ? val : "");
    dq->tail = (dq->tail + 1) % dq->cap;
    dq->len++;
}

char* __deque_pop_front(DesiDeque* dq) {
    if (dq->len == 0) return strdup("");
    char* val = dq->items[dq->head];
    dq->items[dq->head] = NULL;
    dq->head = (dq->head + 1) % dq->cap;
    dq->len--;
    return val;
}

char* __deque_pop_back(DesiDeque* dq) {
    if (dq->len == 0) return strdup("");
    dq->tail = (dq->tail - 1 + dq->cap) % dq->cap;
    char* val = dq->items[dq->tail];
    dq->items[dq->tail] = NULL;
    dq->len--;
    return val;
}

const char* __deque_peek_front(DesiDeque* dq) {
    if (dq->len == 0) return "";
    return dq->items[dq->head];
}

const char* __deque_peek_back(DesiDeque* dq) {
    if (dq->len == 0) return "";
    int idx = (dq->tail - 1 + dq->cap) % dq->cap;
    return dq->items[idx];
}

int32_t __deque_len(DesiDeque* dq)      { return dq ? dq->len : 0; }
int32_t __deque_is_empty(DesiDeque* dq) { return dq ? (dq->len == 0) : 1; }

void __deque_free(DesiDeque* dq) {
    if (!dq) return;
    for (int i = 0; i < dq->len; i++) {
        free(dq->items[(dq->head + i) % dq->cap]);
    }
    free(dq->items);
    free(dq);
}

/* ============================================================
 * Stack — LIFO wrapper over dynamic array
 * ============================================================ */

typedef struct {
    char** items;
    int    len;
    int    cap;
} DesiStack;

DesiStack* __stack_new(void) {
    DesiStack* s = (DesiStack*)calloc(1, sizeof(DesiStack));
    s->cap = 16;
    s->items = (char**)calloc(s->cap, sizeof(char*));
    return s;
}

void __stack_push(DesiStack* s, const char* val) {
    if (s->len >= s->cap) {
        s->cap *= 2;
        s->items = (char**)realloc(s->items, sizeof(char*) * s->cap);
    }
    s->items[s->len++] = strdup(val ? val : "");
}

char* __stack_pop(DesiStack* s) {
    if (s->len == 0) return strdup("");
    return s->items[--s->len];
}

const char* __stack_peek(DesiStack* s) {
    if (s->len == 0) return "";
    return s->items[s->len - 1];
}

int32_t __stack_len(DesiStack* s)      { return s ? s->len : 0; }
int32_t __stack_is_empty(DesiStack* s) { return s ? (s->len == 0) : 1; }

void __stack_free(DesiStack* s) {
    if (!s) return;
    for (int i = 0; i < s->len; i++) free(s->items[i]);
    free(s->items);
    free(s);
}

/* ============================================================
 * Queue — FIFO wrapper (uses deque internally)
 * ============================================================ */

typedef DesiDeque DesiQueue;

DesiQueue* __queue_new(void)                            { return __deque_new(); }
void       __queue_enqueue(DesiQueue* q, const char* v) { __deque_push_back(q, v); }
char*      __queue_dequeue(DesiQueue* q)                { return __deque_pop_front(q); }
const char*__queue_peek(DesiQueue* q)                   { return __deque_peek_front(q); }
int32_t    __queue_len(DesiQueue* q)                    { return __deque_len(q); }
int32_t    __queue_is_empty(DesiQueue* q)               { return __deque_is_empty(q); }
void       __queue_free(DesiQueue* q)                   { __deque_free(q); }

/* ============================================================
 * Counter — hash map of string → count
 * ============================================================ */

typedef struct CounterEntry {
    char* key;
    int64_t count;
    struct CounterEntry* next;
} CounterEntry;

typedef struct {
    CounterEntry** buckets;
    int num_buckets;
    int len;
} DesiCounter;

static unsigned int counter_hash(const char* s, int nbuckets) {
    unsigned int h = 5381;
    while (*s) h = ((h << 5) + h) + (unsigned char)*s++;
    return h % nbuckets;
}

DesiCounter* __counter_new(void) {
    DesiCounter* c = (DesiCounter*)calloc(1, sizeof(DesiCounter));
    c->num_buckets = 64;
    c->buckets = (CounterEntry**)calloc(c->num_buckets, sizeof(CounterEntry*));
    return c;
}

static CounterEntry* counter_find(DesiCounter* c, const char* key) {
    unsigned int idx = counter_hash(key, c->num_buckets);
    CounterEntry* e = c->buckets[idx];
    while (e) {
        if (strcmp(e->key, key) == 0) return e;
        e = e->next;
    }
    return NULL;
}

void __counter_add_n(DesiCounter* c, const char* val, int64_t n) {
    if (!c || !val) return;
    CounterEntry* e = counter_find(c, val);
    if (e) {
        e->count += n;
        return;
    }
    /* New entry */
    unsigned int idx = counter_hash(val, c->num_buckets);
    e = (CounterEntry*)malloc(sizeof(CounterEntry));
    e->key = strdup(val);
    e->count = n;
    e->next = c->buckets[idx];
    c->buckets[idx] = e;
    c->len++;
}

void __counter_add(DesiCounter* c, const char* val) {
    __counter_add_n(c, val, 1);
}

int64_t __counter_get(DesiCounter* c, const char* val) {
    if (!c || !val) return 0;
    CounterEntry* e = counter_find(c, val);
    return e ? e->count : 0;
}

int64_t __counter_total(DesiCounter* c) {
    if (!c) return 0;
    int64_t total = 0;
    for (int i = 0; i < c->num_buckets; i++) {
        CounterEntry* e = c->buckets[i];
        while (e) { total += e->count; e = e->next; }
    }
    return total;
}

int32_t __counter_len(DesiCounter* c) { return c ? c->len : 0; }

/* most_common: returns "key1:count1\nkey2:count2\n..." for top n */
char* __counter_most_common(DesiCounter* c, int n) {
    if (!c || c->len == 0) return strdup("");

    /* Collect all entries into array */
    int total = c->len;
    CounterEntry** arr = (CounterEntry**)malloc(sizeof(CounterEntry*) * total);
    int idx = 0;
    for (int i = 0; i < c->num_buckets; i++) {
        CounterEntry* e = c->buckets[i];
        while (e) { arr[idx++] = e; e = e->next; }
    }

    /* Sort by count descending (simple insertion sort) */
    for (int i = 1; i < total; i++) {
        CounterEntry* tmp = arr[i];
        int j = i - 1;
        while (j >= 0 && arr[j]->count < tmp->count) {
            arr[j + 1] = arr[j]; j--;
        }
        arr[j + 1] = tmp;
    }

    /* Build result string */
    if (n > total) n = total;
    char buf[4096];
    int len = 0;
    for (int i = 0; i < n; i++) {
        len += snprintf(buf + len, sizeof(buf) - len, "%s:%lld\n",
                        arr[i]->key, (long long)arr[i]->count);
    }
    free(arr);
    return strdup(buf);
}

void __counter_free(DesiCounter* c) {
    if (!c) return;
    for (int i = 0; i < c->num_buckets; i++) {
        CounterEntry* e = c->buckets[i];
        while (e) {
            CounterEntry* next = e->next;
            free(e->key);
            free(e);
            e = next;
        }
    }
    free(c->buckets);
    free(c);
}
