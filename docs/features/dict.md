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
| `setdefault(key, default)` | Get value if exists, else insert default and return it |
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

### Nested Dicts

Dicts can contain other dicts as values:

```desi
# Config with environment-specific settings
let config: dict[str, dict[str, int]] = {
    "dev": {"port": 3000, "debug": 1},
    "prod": {"port": 80, "debug": 0}
}

# Check outer key
if "dev" in config:
    print("dev config exists")

# Extract inner dict and access values
let dev = config.get("dev", {})
let port = dev.get("port", 0)
print(port)  # Outputs: 3000

# Check key in inner dict
if "port" in dev:
    print("port is configured")
```

You can also use integer keys for nested dicts:

```desi
let matrix: dict[int, dict[int, str]] = {
    1: {10: "a", 20: "b"},
    2: {30: "c", 40: "d"}
}

if 1 in matrix:
    let row = matrix.get(1, {})
    if 10 in row:
        print("cell (1, 10) exists")
```

## Float Keys Note

> [!NOTE]
> Float keys use bit-exact comparison. This means:
> - `0.0` and `-0.0` are different keys
> - `NaN` values won't match themselves
>
> Using floats as keys is valid but uncommon. Prefer `int` or `str` keys when possible.

## Custom Type Keys

Any custom type (class, struct, enum) can be used as a dict key. By default, keys use **pointer identity**:

```desi
class SimpleClass:
    pub x: int

let cache: dict[SimpleClass, str] = {}
let obj = SimpleClass(1)
cache.insert(obj, "stored")

if obj in cache:        # Found - same pointer
    print("OK")

let copy = SimpleClass(1)
if copy in cache:       # NOT found - different pointer!
    print("found")
else:
    print("not found")  # This prints
```

### Value-Based Keys (with `__hash__`/`__eq__`)

For value-based equality, implement both `__hash__` and `__eq__`:

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __hash__(self) -> u64:
        return self.x * 31 + self.y
    
    pub def __eq__(self, other: Point) -> bool:
        return self.x == other.x and self.y == other.y

let cache: dict[Point, str] = {}
cache.insert(Point(1, 2), "origin")

# Different object, same values - FOUND!
if Point(1, 2) in cache:
    print("found via value equality")
```

### Enum Keys

> [!IMPORTANT]
> Enums use pointer identity. Each access to `Color.Red` creates a **new allocation**.
> Store enum values in variables for reliable lookups:

```desi
enum Color:
    Red: none
    Green: none

let red = Color.Red          # Store in variable
let cache: dict[Color, str] = {}
cache.insert(red, "color")

if red in cache:             # Same variable - found!
    print("OK")

if Color.Red in cache:       # NEW allocation - NOT found!
    print("found")
```

