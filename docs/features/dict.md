# Dict (Dictionary)

Desi's `dict[K, V]` is a hash map that maps keys of type `K` to values of type `V`.

## Key Types

Desi dictionaries support the following key types:

| Key Type | Example |
|----------|---------|
| `str` | `{"name": "Alice", "city": "NYC"}` |
| `int` | `{1: "one", 2: "two", 3: "three"}` |
| `bool` | `{true: "yes", false: "no"}` |
| `float` | `{1.5: "half", 3.14: "pi"}` |

## Creating Dicts

```desi
# String keys (most common)
let prices: dict[str, int] = {"apple": 100, "banana": 50}

# Integer keys
let names: dict[int, str] = {1: "Alice", 2: "Bob"}

# Boolean keys
let flags: dict[bool, str] = {true: "enabled", false: "disabled"}

# Float keys
let rates: dict[float, str] = {0.5: "half", 1.0: "full"}

# Empty dict
let empty: dict[str, int] = {}
```

## Membership Test (`in` operator)

Check if a key exists in a dict:

```desi
let prices: dict[str, int] = {"apple": 100, "banana": 50}

if "apple" in prices:
    print("Apple is available")

if "orange" in prices:
    print("Orange is available")
else:
    print("Orange not found")
```

## Iterating Over Dicts

Use `dict.items()` to iterate over key-value pairs:

```desi
let scores: dict[str, int] = {"Alice": 95, "Bob": 87, "Carol": 92}

for name: str, score: int in scores.items():
    print(name)
    print(score)
```

## Dict Methods

| Method | Description |
|--------|-------------|
| `has_key(key)` | Returns `true` if key exists |
| `get(key, default)` | Get value or default if not found |
| `insert(key, value)` | Insert or update key-value pair |
| `clear()` | Remove all entries |
| `to_str()` | Convert to string representation |

## Examples

### Simple Key-Value Store

```desi
let config: dict[str, str] = {
    "host": "localhost",
    "port": "8080",
    "debug": "true"
}

if "host" in config:
    print("Host is configured")
```

### Counter with Integer Keys

```desi
let counts: dict[int, int] = {1: 0, 2: 0, 3: 0}

# Increment counter for key 2
counts.insert(2, 1)
```

### Multiple Dicts

```desi
let english: dict[str, str] = {"hello": "Hi", "bye": "Goodbye"}
let spanish: dict[str, str] = {"hello": "Hola", "bye": "Adiós"}

if "hello" in english:
    print("English greeting available")
```

## Float Keys Note

> [!NOTE]
> Float keys use bit-exact comparison. This means:
> - `0.0` and `-0.0` are different keys
> - `NaN` values won't match themselves
>
> Using floats as keys is valid but uncommon. Prefer `int` or `str` keys when possible.

## Future: Custom Type Keys

In a future version, you'll be able to use custom types as keys by implementing `__hash__` and `__eq__`:

```desi
# Future syntax
class Point:
    x: int
    y: int
    
    pub def __hash__(self) -> int:
        return self.x * 31 + self.y
    
    pub def __eq__(self, other: Point) -> bool:
        return self.x == other.x and self.y == other.y

# Then use as key:
let cache: dict[Point, str] = {Point(0, 0): "origin"}
```
