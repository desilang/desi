# Builtin Functions

This is a reference for all built-in functions available in Desi.

---

## Output

### `print(value)`

Print a value to standard output.

```desi
print("Hello, World!")
print(42)
print(3.14)
```

---

## Collections

### `len(collection) -> int`

Returns the length of a collection.

```desi
let nums = [1, 2, 3, 4, 5]
print(len(nums))  # 5

let name = "Desi"
print(len(name))  # 4
```

### `range(stop) -> list<int>`

Generate a list of numbers from 0 to stop-1.

```desi
for i in range(5):
    print(i)  # 0, 1, 2, 3, 4
```

### `range(start, stop) -> list<int>`

Generate a list from start to stop-1.

```desi
for i in range(2, 6):
    print(i)  # 2, 3, 4, 5
```

---

## Aggregation

### `sum(collection) -> int|float`

Sum all elements.

```desi
let nums = [1, 2, 3, 4, 5]
print(sum(nums))  # 15
```

### `min(collection) -> T`

Find the minimum value.

```desi
let nums = [5, 2, 8, 1, 9]
print(min(nums))  # 1
```

### `max(collection) -> T`

Find the maximum value.

```desi
let nums = [5, 2, 8, 1, 9]
print(max(nums))  # 9
```

---

## Boolean Checks

### `any(collection) -> bool`

Returns `true` if any element is truthy.

```desi
let vals = [false, false, true]
print(any(vals))  # true
```

### `all(collection) -> bool`

Returns `true` if all elements are truthy.

```desi
let vals = [true, true, true]
print(all(vals))  # true
```

---

## Transformation

### `map(fn, collection) -> list`

Apply a function to each element.

```desi
let nums = [1, 2, 3]
let doubled = map(|x| x * 2, nums)
print(doubled)  # [2, 4, 6]
```

### `filter(fn, collection) -> list`

Keep elements that satisfy a predicate.

```desi
let nums = [1, 2, 3, 4, 5]
let evens = filter(|x| x % 2 == 0, nums)
print(evens)  # [2, 4]
```

### `sorted(collection) -> list`

Return a sorted copy.

```desi
let nums = [3, 1, 4, 1, 5]
let ordered = sorted(nums)
print(ordered)  # [1, 1, 3, 4, 5]
```

### `reversed(collection) -> list`

Return a reversed copy.

```desi
let nums = [1, 2, 3]
let rev = reversed(nums)
print(rev)  # [3, 2, 1]
```

---

## Combining

### `zip(a, b) -> list<tuple>`

Combine two lists element-wise.

```desi
let names = ["Alice", "Bob"]
let ages = [30, 25]
let pairs = zip(names, ages)
# [("Alice", 30), ("Bob", 25)]
```

### `enumerate(collection) -> list<tuple>`

Pair each element with its index.

```desi
let names = ["a", "b", "c"]
for i, name in enumerate(names):
    print(i)
    print(name)
```

---

## Reduction

### `reduce(fn, collection, initial) -> T`

Reduce a collection to a single value.

```desi
let nums = [1, 2, 3, 4, 5]
let total = reduce(|acc, x| acc + x, nums, 0)
print(total)  # 15
```

### `foldl(fn, collection, initial) -> T`

Left fold (same as reduce).

### `foldr(fn, collection, initial) -> T`

Right fold.

---

## Type Conversion

### `int(value) -> int`

Convert to integer.

```desi
let x = int(3.7)  # 3
let y = int("42")  # 42
```

### `float(value) -> float`

Convert to float.

### `str(value) -> str`

Convert to string.

### `bool(value) -> bool`

Convert to boolean.
