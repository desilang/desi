# cache — LRU Cache

In-memory least-recently-used (LRU) cache with optional TTL expiration.

## Import

```desi
import cache
```

## API Reference

| Function | Description |
|---|---|
| `cache.lru_new(max_size) -> cptr` | Create a new LRU cache |
| `cache.set(h, key, value) -> int` | Set a key-value pair |
| `cache.set_ttl(h, key, value, ttl_sec) -> int` | Set with TTL in seconds |
| `cache.get(h, key) -> str` | Get value (empty if missing/expired) |
| `cache.has(h, key) -> bool` | Check if key exists |
| `cache.remove(h, key) -> int` | Remove a key |
| `cache.clear(h) -> int` | Clear all entries |
| `cache.size(h) -> int` | Number of entries |
| `cache.free(h) -> int` | Free the cache |

## Examples

### Basic Caching

```desi
import cache

def main() -> int:
    let c = cache.lru_new(100)

    cache.set(c, "user:1", "Alice")
    cache.set(c, "user:2", "Bob")

    print(cache.get(c, "user:1"))  # Alice
    print(cache.has(c, "user:2"))  # true
    print(cache.size(c))           # 2

    cache.remove(c, "user:1")
    print(cache.has(c, "user:1"))  # false

    cache.free(c)
    0
```

### TTL Expiration

```desi
import cache

def main() -> int:
    let c = cache.lru_new(50)

    # Entry expires after 5 seconds
    cache.set_ttl(c, "session", "abc123", 5)

    print(cache.get(c, "session"))  # abc123

    # After 5 seconds, cache.get would return ""
    cache.free(c)
    0
```
