# collections — Stack, Queue, Deque, Counter

Classic data structures for common programming tasks.

## Import

```desi
import collections
```

## Stack (LIFO)

| Function | Description |
|---|---|
| `collections.stack_new() -> cptr` | Create a new stack |
| `collections.stack_push(s, value) -> int` | Push a string value |
| `collections.stack_pop(s) -> str` | Pop the top value |
| `collections.stack_peek(s) -> str` | Peek at the top value |
| `collections.stack_size(s) -> int` | Number of elements |
| `collections.stack_is_empty(s) -> bool` | Check if empty |
| `collections.stack_free(s) -> int` | Free the stack |

## Queue (FIFO)

| Function | Description |
|---|---|
| `collections.queue_new() -> cptr` | Create a new queue |
| `collections.queue_enqueue(q, value) -> int` | Add to back |
| `collections.queue_dequeue(q) -> str` | Remove from front |
| `collections.queue_peek(q) -> str` | Peek at front |
| `collections.queue_size(q) -> int` | Number of elements |
| `collections.queue_is_empty(q) -> bool` | Check if empty |
| `collections.queue_free(q) -> int` | Free the queue |

## Deque (Double-ended Queue)

| Function | Description |
|---|---|
| `collections.deque_new() -> cptr` | Create a new deque |
| `collections.deque_push_front(dq, value) -> int` | Add to front |
| `collections.deque_push_back(dq, value) -> int` | Add to back |
| `collections.deque_pop_front(dq) -> str` | Remove from front |
| `collections.deque_pop_back(dq) -> str` | Remove from back |
| `collections.deque_peek_front(dq) -> str` | Peek at front |
| `collections.deque_peek_back(dq) -> str` | Peek at back |
| `collections.deque_size(dq) -> int` | Number of elements |
| `collections.deque_is_empty(dq) -> bool` | Check if empty |
| `collections.deque_free(dq) -> int` | Free the deque |

## Counter

| Function | Description |
|---|---|
| `collections.counter_new() -> cptr` | Create a new counter |
| `collections.counter_add(c, key) -> int` | Increment key by 1 |
| `collections.counter_add_n(c, key, n) -> int` | Increment key by n |
| `collections.counter_get(c, key) -> int` | Get count for key |
| `collections.counter_total(c) -> int` | Sum of all counts |
| `collections.counter_size(c) -> int` | Number of unique keys |
| `collections.counter_most_common(c, n) -> str` | Top n keys as string |
| `collections.counter_free(c) -> int` | Free the counter |

## Examples

### Stack

```desi
import collections

def main() -> int:
    let s = collections.stack_new()
    collections.stack_push(s, "a")
    collections.stack_push(s, "b")
    collections.stack_push(s, "c")

    print(collections.stack_size(s))   # 3
    print(collections.stack_pop(s))    # c
    print(collections.stack_peek(s))   # b
    collections.stack_free(s)
    0
```

### Queue

```desi
import collections

def main() -> int:
    let q = collections.queue_new()
    collections.queue_enqueue(q, "first")
    collections.queue_enqueue(q, "second")

    print(collections.queue_dequeue(q))  # first
    print(collections.queue_dequeue(q))  # second
    collections.queue_free(q)
    0
```

### Counter

```desi
import collections

def main() -> int:
    let c = collections.counter_new()
    collections.counter_add(c, "apple")
    collections.counter_add(c, "banana")
    collections.counter_add_n(c, "apple", 2)

    print(collections.counter_get(c, "apple"))   # 3
    print(collections.counter_get(c, "banana"))   # 1
    print(collections.counter_total(c))            # 4
    print(collections.counter_most_common(c, 2))   # apple:3,banana:1
    collections.counter_free(c)
    0
```
