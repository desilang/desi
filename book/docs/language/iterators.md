# Iterators

Desi provides a **lazy iterator protocol** that enables efficient, memory-safe iteration over collections. Instead of creating intermediate collections, iterators process elements on-demand, making them ideal for large datasets and chained operations.

## Basic Usage

### Creating an Iterator

Use the `.iter()` method to create an iterator from any list:

```python
let nums = [1, 2, 3, 4, 5]
let it = nums.iter()  # Creates a ListIter[int]
```

### Collecting Results

Use `.collect()` to materialize an iterator back into a list:

```python
let nums = [1, 2, 3, 4, 5]
let collected = nums.iter().collect()  # [1, 2, 3, 4, 5]
```

## Complete Example

```python
def main() -> int:
    # Create a list
    let numbers = [1, 2, 3, 4, 5]
    
    # Iterate and collect
    let copied = numbers.iter().collect()
    print(copied)  # [1, 2, 3, 4, 5]
    
    return 0
```

## Working with Different Types

### Primitive Types

Iterators work seamlessly with all primitive types:

```python
# Integers
let ints = [1, 2, 3]
let int_copy = ints.iter().collect()

# Strings
let strings = ["hello", "world"]
let str_copy = strings.iter().collect()
```

### Structs

Iterators preserve struct data integrity:

```python
struct Point:
    x: int
    y: int

def main() -> int:
    let p1 = Point(x=10, y=20)
    let p2 = Point(x=30, y=40)
    let points = [p1, p2]
    
    # Iterate and collect preserves field values
    let collected = points.iter().collect()
    let first = collected[0]
    print(first.x)  # 10
    print(first.y)  # 20
    
    return 0
```

### Classes

Iterators work with class instances:

```python
class Person:
    pub name: str
    pub age: int
    
    pub def __new__(name: str, age: int) -> Person:
        return Person(name=name, age=age)

def main() -> int:
    let alice = Person("Alice", 30)
    let bob = Person("Bob", 25)
    let people = [alice, bob]
    
    # Iterate and collect
    let collected = people.iter().collect()
    let first = collected[0]
    print(first.name)  # Alice
    
    return 0
```

## Edge Cases

### Empty Lists

Empty lists iterate correctly:

```python
let empty: list[int] = []
let result = empty.iter().collect()  # []
```

### Single Element

Single-element lists work as expected:

```python
let single = [42]
let result = single.iter().collect()  # [42]
```

### Multiple Iterations

You can create multiple iterators from the same source:

```python
let source = [1, 2, 3]
let iter1 = source.iter()
let iter2 = source.iter()

let result1 = iter1.collect()  # [1, 2, 3]
let result2 = iter2.collect()  # [1, 2, 3]
```

### Source Preservation

The original list is never modified by iteration:

```python
let original = [1, 2, 3]
let _ = original.iter().collect()
print(original)  # [1, 2, 3] - unchanged
```

## Performance Characteristics

- **Stack Allocation**: Iterator structs are allocated on the stack, avoiding heap allocation overhead
- **Zero-Copy**: Iteration doesn't copy elements until `.collect()` is called
- **Lazy Evaluation**: Elements are processed only when needed

## Available Methods

| Method | Description | Return Type |
|--------|-------------|-------------|
| `iter()` | Create iterator from list | `ListIter[T]` |
| `collect()` | Materialize iterator to list | `list[T]` |
| `first()` | Get first element (if any) | `Option[T]` |

## Coming Soon

The following iterator adaptors are planned for future releases:

- `map(fn)` - Transform each element
- `filter(predicate)` - Keep elements matching condition
- `take(n)` - Take first n elements
- `skip(n)` - Skip first n elements
- `enumerate()` - Add index to each element
- `zip(other)` - Combine two iterators
