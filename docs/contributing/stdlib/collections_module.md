# Collections Module Implementation

Internal documentation for the `collections` standard library module.

## Architecture

```
compiler/lib/collections.desi     → Desi API (stack, queue, deque, counter)
compiler/runtime/collections.c    → C runtime (linked lists, hash maps)
```

## C Runtime Symbols

### Stack
| C Symbol | Desi API |
|----------|----------|
| `__stack_new` | `collections.stack_new` |
| `__stack_push` | `collections.stack_push` |
| `__stack_pop` | `collections.stack_pop` |
| `__stack_peek` | `collections.stack_peek` |
| `__stack_len` | `collections.stack_size` |
| `__stack_is_empty` | `collections.stack_is_empty` |
| `__stack_free` | `collections.stack_free` |

### Queue
| C Symbol | Desi API |
|----------|----------|
| `__queue_new` | `collections.queue_new` |
| `__queue_enqueue` | `collections.queue_enqueue` |
| `__queue_dequeue` | `collections.queue_dequeue` |
| `__queue_peek` | `collections.queue_peek` |
| `__queue_len` | `collections.queue_size` |
| `__queue_is_empty` | `collections.queue_is_empty` |
| `__queue_free` | `collections.queue_free` |

### Deque
| C Symbol | Desi API |
|----------|----------|
| `__deque_new` | `collections.deque_new` |
| `__deque_push_front` | `collections.deque_push_front` |
| `__deque_push_back` | `collections.deque_push_back` |
| `__deque_pop_front` | `collections.deque_pop_front` |
| `__deque_pop_back` | `collections.deque_pop_back` |
| `__deque_peek_front` | `collections.deque_peek_front` |
| `__deque_peek_back` | `collections.deque_peek_back` |
| `__deque_len` | `collections.deque_size` |
| `__deque_is_empty` | `collections.deque_is_empty` |
| `__deque_free` | `collections.deque_free` |

### Counter
| C Symbol | Desi API |
|----------|----------|
| `__counter_new` | `collections.counter_new` |
| `__counter_add` | `collections.counter_add` |
| `__counter_add_n` | `collections.counter_add_n` |
| `__counter_get` | `collections.counter_get` |
| `__counter_total` | `collections.counter_total` |
| `__counter_len` | `collections.counter_size` |
| `__counter_most_common` | `collections.counter_most_common` |
| `__counter_free` | `collections.counter_free` |

## Implementation Notes

- **Naming convention**: The C runtime uses `__<type>_len` for size, but the Desi API exposes `<type>_size` for consistency with other languages.
- All handles are `cptr` (opaque pointers to heap-allocated C structs).
- The collections module is **flat** — no namespaced sub-types. All functions are prefixed with the collection type (e.g. `stack_`, `queue_`).

## Test Coverage

- `examples/502_collections_module.desi` — full coverage of all four collection types
