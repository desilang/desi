# Reference Counting (Rc)

`Rc[T]` provides shared ownership of values through reference counting.

## Creating an Rc

```python
let r := rc(42)         # Rc[int]
let s := rc("hello")    # Rc[str]
```

## Getting the Value

```python
let r := rc(42)
let val := r.get()      # Returns 42
print(val)              # 42
```

## Cloning (Sharing)

```python
let r := rc(42)
let r2 := r.clone()     # Both point to same value

# Both r and r2 now share ownership
# Value is freed when BOTH go out of scope
```

## How It Works

1. `rc(value)` allocates memory and stores the value with refcount=1
2. `.clone()` increments the refcount (doesn't copy the value)
3. `.get()` returns the inner value
4. When Rc goes out of scope, refcount decrements
5. When refcount reaches 0, memory is freed

## Example: Shared Ownership

```python
def main() -> int:
    let data := rc(100)
    
    let copy1 := data.clone()  # refcount=2
    let copy2 := data.clone()  # refcount=3
    
    print(data.get())   # 100
    print(copy1.get())  # 100
    print(copy2.get())  # 100
    
    # All three go out of scope → refcount drops to 0 → freed
    return 0
```

## Future: Arc (Atomic RC)

`Arc[T]` will provide thread-safe reference counting for concurrent code.
