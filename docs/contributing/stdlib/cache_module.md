# Cache Module Implementation

Internal documentation for the `cache` standard library module.

## Architecture

```
compiler/lib/cache.desi     → Desi API (LRU cache)
compiler/runtime/cache.c    → C runtime (doubly-linked list + hash map)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__cache_lru_new` | `cache.lru_new` | `(int32 max_size) → LRUCache*` |
| `__cache_lru_set` | `cache.set` | `(LRUCache*, char*, char*) → int32` |
| `__cache_lru_set_ttl` | `cache.set_ttl` | `(LRUCache*, char*, char*, int32) → int32` |
| `__cache_lru_get` | `cache.get` | `(LRUCache*, char*) → char*` |
| `__cache_lru_has` | `cache.has` | `(LRUCache*, char*) → int32` (→ bool) |
| `__cache_lru_remove` | `cache.remove` | `(LRUCache*, char*) → int32` |
| `__cache_lru_clear` | `cache.clear` | `(LRUCache*) → int32` |
| `__cache_lru_len` | `cache.size` | `(LRUCache*) → int32` |
| `__cache_lru_free` | `cache.free` | `(LRUCache*) → int32` |

## Implementation Notes

- Uses a hash map for O(1) lookups and a doubly-linked list for LRU eviction ordering.
- TTL is checked on `get()` — expired entries return empty string and are lazily removed.
- When the cache exceeds `max_size`, the least-recently-used entry is evicted.
- All keys and values are `strdup()`'d on insertion and `free()`'d on eviction.

## Test Coverage

- `examples/505_cache_module.desi`
