# Atomic

An **Atomic** provides lock-free, thread-safe integer operations. No locks needed - just pure atomic CPU instructions!

## Basic Usage

```desi
import sync

let a = sync.Atomic(0)  # Create atomic integer

let val = a.load()      # Read value atomically
a.store(42)             # Write value atomically
let new_val = a.inc()   # Increment and get new value
```

## When to Use Atomic

Use Atomic when:
- **Counters** - Thread-safe incrementing/decrementing
- **Flags** - Simple boolean-like state (0/1)
- **Lock-free algorithms** - Compare-and-swap patterns
- **Performance critical** - No lock overhead

## Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `load()` | `int` | Read the current value |
| `store(value)` | `none` | Write a new value |
| `add(delta)` | `int` | Add delta, return **new** value |
| `sub(delta)` | `int` | Subtract delta, return **new** value |
| `inc()` | `int` | Increment by 1, return **new** value |
| `dec()` | `int` | Decrement by 1, return **new** value |
| `compare_exchange(expected, desired)` | `bool` | If current == expected, set to desired |
| `exchange(new_value)` | `int` | Replace value, return **old** value |

## Examples

### Counter
```desi
import sync

let counter = sync.Atomic(0)

let v1 = counter.inc()  # 1
let v2 = counter.inc()  # 2
let v3 = counter.inc()  # 3
```

### Compare-and-Swap (CAS)
```desi
import sync

let a = sync.Atomic(50)

# Only update if current value is 50
if a.compare_exchange(50, 99):
    print("Updated to 99")
else:
    print("Value changed, didn't update")
```

### Exchange
```desi
import sync

let a = sync.Atomic(42)
let old = a.exchange(100)  # old = 42, a now = 100
```

## Edge Cases

### Zero and Negative Values
```desi
let a = sync.Atomic(0)
let v1 = a.dec()  # -1 (negatives work!)

let b = sync.Atomic(-10)
let v2 = b.add(20)  # 10
```

## Atomic vs Mutex

| Feature | Atomic | Mutex |
|---------|--------|-------|
| Lock-free | ✅ Yes | ❌ No |
| Speed | Faster | Slower |
| Data types | int only | Any type |
| Operations | Predefined methods | Custom code |
| Use case | Counters, flags | Complex data |

## Important Notes

1. **Use intermediate variables** when printing:
   ```desi
   # Correct
   let x = a.inc()
   print(x)
   
   # May cause double-evaluation (known issue)
   print(a.inc())
   ```

2. **Atomic is for integers only** - use `Mutex<T>` for other types
