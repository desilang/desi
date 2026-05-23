/*
 * cache.c — In-memory LRU and TTL caching for Desi stdlib
 *
 * Thread-safe LRU cache with optional TTL expiration.
 *
 * Public API:
 *   __cache_lru_new(max_size)            → create LRU cache
 *   __cache_lru_set(c, key, value)       → set key-value
 *   __cache_lru_set_ttl(c, key, value, ttl_sec) → set with TTL
 *   __cache_lru_get(c, key)              → get value (or "")
 *   __cache_lru_has(c, key)              → check if key exists
 *   __cache_lru_remove(c, key)           → remove key
 *   __cache_lru_clear(c)                 → clear all entries
 *   __cache_lru_len(c)                   → current size
 *   __cache_lru_free(c)                  → destroy
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <time.h>
#include "platform.h"

/* ---- Doubly-linked list node ---- */

typedef struct CacheNode {
    char* key;
    char* value;
    time_t expires;     /* 0 = no expiry */
    struct CacheNode* prev;
    struct CacheNode* next;
} CacheNode;

/* ---- Hash map bucket ---- */

typedef struct CacheBucket {
    CacheNode* node;
    struct CacheBucket* next;
} CacheBucket;

/* ---- LRU Cache ---- */

typedef struct {
    CacheBucket** buckets;
    int           num_buckets;
    CacheNode*    head;       /* most recently used */
    CacheNode*    tail;       /* least recently used */
    int           len;
    int           max_size;
    DesiPlatformMutex lock;
} LRUCache;

/* ---- Hash ---- */
static unsigned int cache_hash(const char* s, int nbuckets) {
    unsigned int h = 5381;
    while (*s) h = ((h << 5) + h) + (unsigned char)*s++;
    return h % nbuckets;
}

/* ---- DLL helpers ---- */

static void dll_remove(LRUCache* c, CacheNode* n) {
    if (n->prev) n->prev->next = n->next;
    else c->head = n->next;
    if (n->next) n->next->prev = n->prev;
    else c->tail = n->prev;
    n->prev = n->next = NULL;
}

static void dll_push_front(LRUCache* c, CacheNode* n) {
    n->next = c->head;
    n->prev = NULL;
    if (c->head) c->head->prev = n;
    c->head = n;
    if (!c->tail) c->tail = n;
}

static void bucket_remove(LRUCache* c, const char* key) {
    unsigned int idx = cache_hash(key, c->num_buckets);
    CacheBucket** pp = &c->buckets[idx];
    while (*pp) {
        if (strcmp((*pp)->node->key, key) == 0) {
            CacheBucket* tmp = *pp;
            *pp = tmp->next;
            free(tmp);
            return;
        }
        pp = &(*pp)->next;
    }
}

static CacheNode* bucket_find(LRUCache* c, const char* key) {
    unsigned int idx = cache_hash(key, c->num_buckets);
    CacheBucket* b = c->buckets[idx];
    while (b) {
        if (strcmp(b->node->key, key) == 0) return b->node;
        b = b->next;
    }
    return NULL;
}

static void bucket_insert(LRUCache* c, CacheNode* n) {
    unsigned int idx = cache_hash(n->key, c->num_buckets);
    CacheBucket* b = (CacheBucket*)malloc(sizeof(CacheBucket));
    b->node = n;
    b->next = c->buckets[idx];
    c->buckets[idx] = b;
}

static void evict_tail(LRUCache* c) {
    if (!c->tail) return;
    CacheNode* victim = c->tail;
    dll_remove(c, victim);
    bucket_remove(c, victim->key);
    free(victim->key);
    free(victim->value);
    free(victim);
    c->len--;
}

static void free_node(CacheNode* n) {
    free(n->key);
    free(n->value);
    free(n);
}

/* ============================================================
 * Public API
 * ============================================================ */

LRUCache* __cache_lru_new(int max_size) {
    LRUCache* c = (LRUCache*)calloc(1, sizeof(LRUCache));
    c->max_size = max_size > 0 ? max_size : 1000;
    c->num_buckets = c->max_size * 2;
    c->buckets = (CacheBucket**)calloc(c->num_buckets, sizeof(CacheBucket*));
    DESI_MUTEX_INIT(c->lock);
    return c;
}

void __cache_lru_set_ttl(LRUCache* c, const char* key, const char* value, int ttl_sec) {
    if (!c || !key) return;
    DESI_MUTEX_LOCK(c->lock);

    CacheNode* existing = bucket_find(c, key);
    if (existing) {
        free(existing->value);
        existing->value = strdup(value ? value : "");
        existing->expires = ttl_sec > 0 ? time(NULL) + ttl_sec : 0;
        dll_remove(c, existing);
        dll_push_front(c, existing);
        DESI_MUTEX_UNLOCK(c->lock);
        return;
    }

    /* Evict if at capacity */
    while (c->len >= c->max_size) evict_tail(c);

    CacheNode* n = (CacheNode*)calloc(1, sizeof(CacheNode));
    n->key = strdup(key);
    n->value = strdup(value ? value : "");
    n->expires = ttl_sec > 0 ? time(NULL) + ttl_sec : 0;

    dll_push_front(c, n);
    bucket_insert(c, n);
    c->len++;

    DESI_MUTEX_UNLOCK(c->lock);
}

void __cache_lru_set(LRUCache* c, const char* key, const char* value) {
    __cache_lru_set_ttl(c, key, value, 0);
}

const char* __cache_lru_get(LRUCache* c, const char* key) {
    if (!c || !key) return "";
    DESI_MUTEX_LOCK(c->lock);

    CacheNode* n = bucket_find(c, key);
    if (!n) {
        DESI_MUTEX_UNLOCK(c->lock);
        return "";
    }

    /* Check TTL */
    if (n->expires > 0 && time(NULL) > n->expires) {
        dll_remove(c, n);
        bucket_remove(c, key);
        free_node(n);
        c->len--;
        DESI_MUTEX_UNLOCK(c->lock);
        return "";
    }

    /* Move to front (most recently used) */
    dll_remove(c, n);
    dll_push_front(c, n);

    DESI_MUTEX_UNLOCK(c->lock);
    return n->value;
}

int32_t __cache_lru_has(LRUCache* c, const char* key) {
    const char* v = __cache_lru_get(c, key);
    return (v && v[0]) ? 1 : 0;
}

void __cache_lru_remove(LRUCache* c, const char* key) {
    if (!c || !key) return;
    DESI_MUTEX_LOCK(c->lock);
    CacheNode* n = bucket_find(c, key);
    if (n) {
        dll_remove(c, n);
        bucket_remove(c, key);
        free_node(n);
        c->len--;
    }
    DESI_MUTEX_UNLOCK(c->lock);
}

void __cache_lru_clear(LRUCache* c) {
    if (!c) return;
    DESI_MUTEX_LOCK(c->lock);
    while (c->head) evict_tail(c);
    DESI_MUTEX_UNLOCK(c->lock);
}

int32_t __cache_lru_len(LRUCache* c)  { return c ? c->len : 0; }

void __cache_lru_free(LRUCache* c) {
    if (!c) return;
    while (c->head) evict_tail(c);
    free(c->buckets);
    DESI_MUTEX_DESTROY(c->lock);
    free(c);
}
