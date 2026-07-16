# Reference Counting (rc/arc/weak)

Desi provides reference-counted smart pointers for shared ownership of heap-allocated values:
*   `rc[T]` - Single-threaded reference counting.
*   `arc[T]` - Thread-safe reference counting (atomic).
*   `weak[T]` - Non-owning weak reference to prevent cyclic references.

## Creating an Rc/Arc

Use the built-in `rc()` or `arc()` constructor functions:

```desi
let r = rc(42)         # rc[int]
let s = arc("hello")   # arc[str]
```

## Accessing the Inner Value

Call the `.get()` method to read the value:

```desi
let r = rc(42)
let val = r.get()      # Returns 42
```

## Sharing Ownership (Clone)

Call `.clone()` to create another owner. This increments the reference count without copying the underlying value:

```desi
let r = rc(42)
let r2 = r.clone()     # Both point to the same value
# Value is freed when BOTH r and r2 go out of scope
```

## Weak References (`weak[T]`)

Weak references do not keep the underlying object alive. They prevent reference cycles (which cause memory leaks).

### Creating a Weak Reference

Pass an `rc[T]` or `arc[T]` to the `weak()` constructor:

```desi
let r = rc(42)
let w = weak(r)        # weak[int]
```

### Upgrading a Weak Reference

To access the value, you must call `.upgrade()` to check if it's still alive. This returns an `Option[rc[T]]`:

```desi
let w = weak(r)

match w.upgrade():
    Some(strong):
        print("Value is alive: " + str(strong.get()))
    Nothing:
        print("Value has been deallocated!")
```

## Full Lifecycle Example

```desi
def create_weak() -> weak[int]:
	let r = rc(123)
	let w = weak(r)
	
	# Upgrade succeeds because r is still alive
	if w.upgrade() is Some(strong):
		print("Value: " + str(strong.get()))
	return w # r goes out of scope and is deallocated

def main() -> int:
	let w = create_weak()
	
	# Upgrade fails because r has been deallocated
	if w.upgrade() is Nothing:
		print("Weak pointer is dead!")
	return 0
```
