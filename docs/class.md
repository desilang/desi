# Classes in Desi - Complete Guide

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [Tutorial: Your First Class](#tutorial-your-first-class)
3. [Fields and Visibility](#fields-and-visibility)
4. [Methods and Self](#methods-and-self)
5. [Constructors](#constructors)
6. [Decorators](#decorators)
7. [Inheritance](#inheritance)
8. [Generic Classes](#generic-classes)
9. [RAII and Resource Management](#raii-and-resource-management)
10. [Dunder Methods Reference](#dunder-methods-reference)
11. [Common Patterns](#common-patterns)
12. [Performance Characteristics](#performance-characteristics)
13. [Design Philosophy](#design-philosophy)

---

## Quick Start

```desi
class Point:
    pub x: int
    pub y: int

def main() -> int:
    let p = Point()  # Zero-arg constructor
    p.x = 10
    p.y = 20
    return 0
```

Key features:
- **Zero-cost abstractions**: Methods compile to static function calls
- **C-compatible layout**: Direct memory access, FFI-friendly
- **Move semantics by default**: Memory safe, no hidden copies
- **Explicit visibility**: `pub` keyword for public fields/methods

---

## Tutorial: Your First Class

### Step 1: Define a Basic Class

```desi
class Counter:
    pub value: int
```

This creates a class with a single public field. By default, Desi generates a zero-arg constructor.

### Step 2: Create an Instance

```desi
def main() -> int:
    let c = Counter()
    c.value = 0
    return 0
```

**Note**: Fields are uninitialized by default. You must assign values explicitly.

### Step 3: Add Methods

```desi
class Counter:
    pub value: int
    
    pub def increment(self) -> none:
        self.value = self.value + 1
        return
    
    pub def get(self) -> int:
        return self.value

def main() -> int:
    let mut c = Counter()
    c.value = 0
    c.increment()
    c.increment()
    let result = c.get()  # result = 2
    return 0
```

**Key Concepts:**
- `self` parameter: Automatically added to instance methods
- `pub` keyword: Required for methods/fields accessible outside the class
- Return values: Always explicit (use `none` for void)

### Step 4: Add a Custom Constructor

```desi
class Counter:
    pub value: int
    
    pub def __new__(initial: int) -> Counter:
        let c = Counter()  # Call zero-arg constructor
        c.value = initial
        return c
    
    pub def increment(self) -> none:
        self.value = self.value + 1
        return

def main() -> int:
    let c = Counter(10)  # Uses custom constructor
    c.increment()
    return 0
```

**Constructor Rules:**
- Name: Always `__new__`
- Must be `pub`
- Return type: The class itself
- Can be overloaded (different parameter counts/types)

---

## Fields and Visibility

### Public Fields

```desi
class Person:
    pub name: str
    pub age: int
```

**Access:** Anyone can read/write

```desi
let p = Person()
p.name = "Alice"
p.age = 30
```

### Private Fields

```desi
class BankAccount:
    pub id: int
    balance: int  # Private - no `pub` keyword
    
    pub def deposit(self, amount: int) -> none:
        self.balance = self.balance + amount
        return
    
    pub def get_balance(self) -> int:
        return self.balance
```

**Access Rules:**
- Private fields accessible only within the class
- Attempting external access: compile error `DTE0010`

```desi
def main() -> int:
    let acc = BankAccount()
    acc.id = 100  # ✅ OK - public
    acc.balance = 1000  # ❌ ERROR - private
    acc.deposit(1000)  # ✅ OK - use public method
    return 0
```

### Field Assignment

```desi
class Point:
    pub x: int
    pub y: int

def main() -> int:
    let mut p = Point()
    p.x = 10  # Direct assignment (zero-cost)
    p.y = 20
    return 0
```

**Assignment Checks:**
- Field exists
- Field is visible (public or same-class)
- Type matches

---

## Methods and Self

### Instance Methods

```desi
class Rectangle:
    pub width: int
    pub height: int
    
    pub def area(self) -> int:
        return self.width * self.height
    
    pub def resize(self, w: int, h: int) -> none:
        self.width = w
        self.height = h
        return
```

**The `self` Parameter:**
- **Explicit in signature**: `def method(self, ...)`
- **Implicit injection**: Checker adds it if omitted (future feature)
- **Type**: Always a pointer to the class instance

### Method Calls

```desi
let r = Rectangle()
r.width = 10
r.height = 20
let a = r.area()  # Compiles to: Rectangle_area(&r)
```

**Performance**: Static dispatch (same cost as C function call)

### Private Methods

```desi
class Calculator:
    pub def add(self, a: int, b: int) -> int:
        return self._validate(a) + self._validate(b)
    
    def _validate(self, n: int) -> int:  # Private - no `pub`
        # Validation logic
        return n

def main() -> int:
    let calc = Calculator()
    let result = calc.add(5, 10)  # ✅ OK
    let x = calc._validate(5)     # ❌ ERROR - private method
    return 0
```

---

## Constructors

### Zero-Arg Default Constructor

If no `__new__` is defined, Desi provides a default:

```desi
class Empty:
    pass

# Equivalent to:
class Empty:
    pub def __new__() -> Empty:
        # Allocates memory, returns uninitialized instance
```

### Custom Constructors

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __new__(px: int, py: int) -> Point:
        let p = Point()  # Call default constructor
        p.x = px
        p.y = py
        return p
```

### Constructor Overloading

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __new__() -> Point:
        # Origin point
        let p = Point()
        p.x = 0
        p.y = 0
        return p
    
    pub def __new__(px: int, py: int) -> Point:
        let p = Point()
        p.x = px
        p.y = py
        return p

def main() -> int:
    let origin = Point()      # Calls first constructor
    let custom = Point(5, 10)  # Calls second constructor
    return 0
```

**Resolution:** Based on argument count and types

### Constructor Best Practices

**✅ DO:**
```desi
pub def __new__(id: int) -> Account:
    let acc = Account()
    acc.id = id
    acc.balance = 0  # Initialize all fields
    return acc
```

**❌ DON'T:**
```desi
pub def __new__(id: int) -> Account:
    let acc = Account()
    acc.id = id
    # balance remains uninitialized - dangerous!
    return acc
```

---

## Decorators

Desi provides three built-in decorators for methods: `@staticmethod`, `@classmethod`, and `@property`.

### @staticmethod

**Purpose**: Define methods that don't need instance state

```desi
class Math:
    @staticmethod
    pub def add(a: int, b: int) -> int:
        return a + b
    
    @staticmethod
    pub def max(a: int, b: int) -> int:
        if a > b:
            return a
        return b

def main() -> int:
    let sum = Math.add(5, 10)  # Call without instance
    let maximum = Math.max(3, 7)
    return 0
```

**Key Points:**
- No `self` parameter
- Called on the class, not an instance
- Zero overhead (compiles to regular function)

**Use Cases:**
- Utility functions related to the class
- Factory methods (though `@classmethod` is more idiomatic)
- Pure functions that logically belong to the class

### @classmethod

**Purpose**: Define methods that operate on the class itself (typically factory methods)

```desi
class Counter:
    pub value: int
    
    @classmethod
    pub def zero() -> Counter:
        let c = Counter()
        c.value = 0
        return c
    
    @classmethod
    pub def with_value(n: int) -> Counter:
        let c = Counter()
        c.value = n
        return c

def main() -> int:
    let c1 = Counter.zero()
    let c2 = Counter.with_value(100)
    return 0
```

**Key Points:**
- No `cls` parameter (unlike Python) - use class name directly
- Called on the class, not an instance
- Typically used for alternative constructors

**Why no `cls` parameter?**
- Desi classes are static types (no runtime class objects)
- No class-level state (yet)
- Simpler than Python's approach
- **Future-proof**: If class variables are added, `cls` can be introduced without breaking code

### @property

**Purpose**: Define methods accessed like fields (getters)

```desi
class Circle:
    pub radius: float
    
    @property
    pub def area(self) -> float:
        return 3.14159 * self.radius * self.radius
    
    @property
    pub def diameter(self) -> float:
        return 2.0 * self.radius

def main() -> int:
    let c = Circle()
    c.radius = 5.0
    let a = c.area      # No parentheses! Calls getter
    let d = c.diameter  # No parentheses!
    return 0
```

**Key Points:**
- Signature: `(self) -> T` (only self parameter)
- Access: `obj.property` (no `()`)
- Cost: One function call (can be inlined by LLVM)

**Performance Considerations:**
```desi
# ❌ BAD: Repeated property access in loop
for i in range(1000):
    let x = heavy_computation.result  # Called 1000 times!

# ✅ GOOD: Cache property value
let result = heavy_computation.result
for i in range(1000):
    let x = result  # Uses cached value
```

**Use Cases:**
- Computed values (area from radius)
- Getters for private fields
- Validation or transformation of field access

---

## Inheritance

### Basic Inheritance

```desi
class Animal:
    pub name: str
    
    pub def speak(self) -> str:
        return "Some sound"

class Dog(Animal):  # Dog inherits from Animal
    pub breed: str
    
    pub def speak(self) -> str:  # Override
        return "Woof!"

def main() -> int:
    let d = Dog()
    d.name = "Buddy"  # Inherited field
    d.breed = "Golden Retriever"
    let sound = d.speak()  # Calls Dog.speak, not Animal.speak
    return 0
```

**Inheritance Rules:**
- Single inheritance only (no multiple inheritance)
- Base class fields come first in memory layout
- Methods can be overridden
- No virtual dispatch - method resolution at compile time

### Method Overriding

```desi
class Base:
    pub def greet(self) -> str:
        return "Hello"

class Derived(Base):
    pub def greet(self) -> str:  # Override
        return "Hi there!"
```

**No `override` keyword** - just define same method name

### Accessing Base Methods

Currently, you cannot call base class methods directly. This is a limitation:

```desi
class Derived(Base):
    pub def greet(self) -> str:
        # Cannot do: super().greet() or Base.greet(self)
        return "Hi"
```

**Workaround:** Rename methods or use composition

### Memory Layout

```desi
class Animal:
    pub name: str  # 8 bytes (ptr)

class Dog(Animal):
    pub breed: str  # 8 bytes (ptr)
```

**Memory:**
```
┌────────────┬─────────────┐
│ name (8B)  │ breed (8B)  │
│ (from Base)│ (Dog field) │
└────────────┴─────────────┘
Total: 16 bytes
```

**Benefits:**
- Predictable layout
- Can cast Dog* to Animal*
- Field access at known offsets

---

## Generic Classes

### Basic Generic Class

```desi
class Box<T>:
    pub value: T

def main() -> int:
    let b1: Box<int> = Box()
    let b2: Box<str> = Box()
    return 0
```

**Type Inference:**
```desi
let b: Box<int> = Box()  # Type inferred from annotation
```

### Generic Methods

```desi
class Container<T>:
    pub data: T
    
    pub def get(self) -> T:
        return self.data
    
    pub def set(self, value: T) -> none:
        self.data = value
        return
```

###Generic Constructors

```desi
class Pair<T, U>:
    pub first: T
    pub second: U
    
    pub def __new__(f: T, s: U) -> Pair<T, U>:
        let p = Pair()
        p.first = f
        p.second = s
        return p

def main() -> int:
    let p: Pair<int, str> = Pair(42, "hello")
    return 0
```

### Constraints (Future)

Currently, no trait bounds. All generic parameters accept any type.

**Planned:**
```desi
class SortedList<T: Ordered>:  # T must implement Ordered
    # ...
```

---

## RAII and Resource Management

### Using Blocks and `__close__`

```desi
class File:
    pub path: str
    
    pub def __close__(self) -> none:
        # Cleanup code - close file descriptor, etc.
        return

def main() -> int:
    using f = File():
        # Use file here
        pass
    # f.__close__() called automatically here
    return 0
```

**Cleanup Order:**
1. Execute body of `using` block
2. Call `__close__()` (even if error occurred)
3. Deallocate instance

### Nested Resources

```desi
using outer = Resource():
    using inner = Resource():
        # Both resources active
        pass
    # inner.__close__() called here
# outer.__close__() called here
```

**Cleanup:** LIFO order (last-in, first-out)

### RAII Best Practices

**✅ DO:**
```desi
class Database:
    pub def __close__(self) -> none:
        # Close connection
        # Flush buffers
        # Release locks
        return
```

**❌ DON'T:**
```desi
class Database:
    pub def __close__(self) -> none:
        # DON'T throw errors here
        # DON'T do heavy computation
        # Keep it simple and fast
        return
```

**When to use RAII:**
- File handles
- Network connections
- Database connections
- Locks and mutexes
- GPU resources
- Temporary buffers

---

## Dunder Methods Reference

### Constructor Methods

#### `__new__`
**Signature:** `pub def __new__(...) -> ClassName`

**Purpose:** Create and initialize class instances

**Example:**
```desi
class User:
    pub id: int
    pub name: str
    
    pub def __new__(uid: int, uname: str) -> User:
        let u = User()
        u.id = uid
        u.name = uname
        return u
```

**Rules:**
- Must be `pub`
- Can be overloaded
- Must return instance of the class

### Display Methods

#### `__repr__` ✅ Implemented
**Signature:** `pub def __repr__(self) -> str`

**Purpose:** String representation for debugging

**Example:**
```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __repr__(self) -> str:
        return "Point(" + str(self.x) + ", " + str(self.y) + ")"

def main() -> int:
    let p = Point()
    p.x = 5
    p.y = 10
    print(p.__repr__())  # Output: Point(5, 10)
    return 0
```

**Rules:**
- Must be `pub`
- Must return `str`
- Takes only `self` parameter

#### `to_str` (Auto-generated)
**Current:** Every class gets a default `to_str` that returns classname + newline

**Future:** Can be customized

### Resource Management Methods

#### `__close__`
**Signature:** `pub def __close__(self) -> none`

**Purpose:** Cleanup when instance goes out of scope (RAII)

**Example:**
```desi
class Connection:
    pub def __close__(self) -> none:
        # Close socket, release resources
        return
```

**Called:** Automatically at end of `using` block

#### `__copy__` ✅ Implemented
**Signature:** `pub def __copy__(self) -> ClassName`

**Purpose:** Create a deep copy of the instance

**Example:**
```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __copy__(self) -> Point:
        let p = Point()
        p.x = self.x
        p.y = self.y
        return p

def main() -> int:
    let p1 = Point()
    p1.x = 5
    p1.y = 10
    let p2 = p1.__copy__()  # Create independent copy
    p2.x = 20  # Modifying p2 doesn't affect p1
    return 0
```

**Rules:**
- Must be `pub`
- Must return the same class type
- Takes only `self` parameter

### Comparison Methods ✅ Implemented

#### `__eq__`
**Signature:** `pub def __eq__(self, other: ClassName) -> bool`

**Purpose:** Equality comparison (`a == b`)

**Example:**
```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __eq__(self, other: Point) -> bool:
        return self.x == other.x and self.y == other.y

def main() -> int:
    let p1 = Point()
    p1.x = 5
    p1.y = 10
    
    let p2 = Point()
    p2.x = 5
    p2.y = 10
    
    let equal = p1 == p2  # Calls p1.__eq__(p2), returns true
    return 0
```

#### `__hash__`
**Signature:** `pub def __hash__(self) -> u64`

**Purpose:** Hash value for use in dicts/sets

**Note:** If you implement `__eq__`, you should implement `__hash__`

**Example:**
```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __hash__(self) -> u64:
        # Simple hash combining x and y
        return (self.x as u64) * 31 + (self.y as u64)
```

### Operator Overloading ✅ Implemented

Desi supports full operator overloading for classes via dunder methods. When you define these methods, operators are automatically desugared to method calls during compilation.

#### Arithmetic Operators

**Supported Operators:**
- `+` → `__add__`
- `-` → `__sub__`
- `*` → `__mul__`
- `/` → `__div__`
- `%` → `__mod__` (planned)
- `**` → `__pow__` (planned)

**Signature:** `pub def __add__(self, other: ClassName) -> ReturnType`

**Example:**
```desi
class Vector:
    pub x: float
    pub y: float
    
    pub def __add__(self, other: Vector) -> Vector:
        let v = Vector()
        v.x = self.x + other.x
        v.y = self.y + other.y
        return v
    
    pub def __sub__(self, other: Vector) -> Vector:
        let v = Vector()
        v.x = self.x - other.x
        v.y = self.y - other.y
        return v
    
    pub def __mul__(self, scale: float) -> Vector:
        let v = Vector()
        v.x = self.x * scale
        v.y = self.y * scale
        return v

def main() -> int:
    let v1 = Vector()
    v1.x = 1.0
    v1.y = 2.0
    
    let v2 = Vector()
    v2.x = 3.0
    v2.y = 4.0
    
    let v3 = v1 + v2      # Calls v1.__add__(v2)
    let v4 = v1 - v2      # Calls v1.__sub__(v2)
    let v5 = v1 * 2.0     # Calls v1.__mul__(2.0)
    return 0
```

#### Comparison Operators

**Supported Operators:**
- `==` → `__eq__`
- `!=` → `__ne__` (or `!__eq__` if `__ne__` is missing)
- `<` → `__lt__` (planned)
- `<=` → `__le__` (planned)
- `>` → `__gt__` (planned)
- `>=` → `__ge__` (planned)

**Signature:** `pub def __eq__(self, other: ClassName) -> bool`

**Example:**
```desi
class Vector:
    pub x: float
    pub y: float
    
    pub def __eq__(self, other: Vector) -> bool:
        return self.x == other.x and self.y == other.y

def main() -> int:
    let v1 = Vector()
    v1.x = 1.0
    v1.y = 2.0
    
    let v2 = Vector()
    v2.x = 1.0
    v2.y = 2.0
    
    let same = v1 == v2   # Calls v1.__eq__(v2), returns true
    let diff = v1 != v2   # Calls v1.__eq__(v2) and negates result, returns false
    return 0
```

**Rules for Operator Dunders:**
- Must be `pub`
- Arithmetic: Must take 2 params (`self`, `other`)
- Comparison: Must take 2 params and return `bool`
- Return type can vary for arithmetic (e.g., `Vector * float -> Vector`)

**Performance:**
- Operators are desugared at compile time to static method calls
- Zero overhead compared to explicit method calls
- Equivalent LLVM IR: `v1 + v2` → `Vector___add__(v1, v2)`

**Note on `!=` Fallback:**
If you don't define `__ne__`, Desi automatically uses `!__eq__` for the `!=` operator. This follows Python's convention.

---

## Common Patterns

### Builder Pattern

```desi
class ConfigBuilder:
    pub host: str
    pub port: int
    pub timeout: int
    
    pub def with_host(self, h: str) -> ConfigBuilder:
        self.host = h
        return self
    
    pub def with_port(self, p: int) -> ConfigBuilder:
        self.port = p
        return self
    
    pub def build(self) -> Config:
        let c = Config()
        c.host = self.host
        c.port = self.port
        return c

# Usage:
let config = ConfigBuilder()
    .with_host("localhost")
    .with_port(8080)
    .build()
```

### Immutable Objects

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def moved(self, dx: int, dy: int) -> Point:
        let p = Point()
        p.x = self.x + dx
        p.y = self.y + dy
        return p  # Return new instance, don't modify self

# Usage - original remains unchanged:
let p1 = Point()
let p2 = p1.moved(5, 10)  # p1 is unchanged
```

### Factory Methods

```desi
class Color:
    pub r: u8
    pub g: u8
    pub b: u8
    
    @classmethod
    pub def red() -> Color:
        let c = Color()
        c.r = 255
        c.g = 0
        c.b = 0
        return c
    
    @classmethod
    pub def from_hex(hex: str) -> Color:
        # Parse hex string
        let c = Color()
        # ... parsing logic
        return c

# Usage:
let c1 = Color.red()
let c2 = Color.from_hex("#FF5733")
```

### Value Objects

```desi
class Money:
    pub amount: int  # in cents
    pub currency: str
    
    pub def __eq__(self, other: Money) -> bool:
        return self.amount == other.amount and 
               self.currency == other.currency
    
    pub def add(self, other: Money) -> Money:
        # Check currency matches
        let m = Money()
        m.amount = self.amount + other.amount
        m.currency = self.currency
        return m
```

---

## Performance Characteristics

### Method Calls

**Cost:** Static function call (same as C)

```desi
obj.method()  // Compiles to: ClassName_method(&obj)
```

**LLVM can inline:** Zero overhead for small methods

### Field Access

**Cost:** Direct memory load/store at known offset

```desi
obj.field = value  // store at offset X
let x = obj.field  // load from offset X
```

**Same as C struct field access**

### Memory Layout

Classes use C-compatible struct layout:
- No hidden fields
- Predictable alignment
- FFI-friendly
- Cache-friendly sequential access

### Constructor Overhead

**Default constructor:**
- Cost: 1 malloc call
- Can be stack-allocated by LLVM if escape analysis proves safe

**Custom constructor:**
- Cost: malloc + initialization code
- Inlining possible

---

## Design Philosophy

### Zero-Cost Abstractions

**Principle:** Classes should have no overhead compared to hand-written C code

**How:**
- Static dispatch (no vtables)
- C-compatible layout (no hidden fields)
- Direct field access (no getters/setters unless explicit)
- Inline-friendly methods

### Explicit Costs

**Principle:** Performance costs should be visible in code

**Examples:**
- `.clone()` instead of implicit copy
- `async def` instead of hidden async
- `@property` signals function call cost

### Memory Safety

**Principle:** Prevent undefined behavior at compile time

**How:**
- Move semantics by default
- Borrow checker prevents use-after-move
- No null pointers (use Option<T>)
- RAII for resource cleanup

### Predictability

**Principle:** Code behavior should be obvious

**Examples:**
- No hidden allocations
- No automatic conversions
- Explicit error handling (`?` operator)
- Static dispatch (no runtime polymorphism)

---

## Common Pitfalls and Solutions

### Pitfall 1: Uninitialized Fields

**❌ Problem:**
```desi
class Point:
    pub x: int
    pub y: int

let p = Point()
let sum = p.x + p.y  // Undefined behavior - fields uninitialized!
```

**✅ Solution:**
```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __new__() -> Point:
        let p = Point()
        p.x = 0
        p.y = 0
        return p
```

### Pitfall 2: Forgetting `pub`

**❌ Problem:**
```desi
class Counter:
    value: int  // Private!
    
    def increment(self) -> none:  // Private!
        self.value = self.value + 1

// In another file:
let c = Counter()
c.increment()  // ERROR: private method
```

**✅ Solution:** Add `pub` to public API

```desi
class Counter:
    pub value: int
    
    pub def increment(self) -> none:
        self.value = self.value + 1
```

### Pitfall 3: Move After Use

**❌ Problem:**
```desi
let c1 = Counter()
let c2 = c1  // c1 is moved
c1.increment()  // ERROR: c1 is moved
```

**✅ Solution:** Use `__copy__` for explicit copying

```desi
let c1 = Counter()
let c2 = c1.__copy__()  // Explicit copy
c1.increment()  // OK
```

### Pitfall 4: Property Performance

**❌ Problem:**
```desi
for i in range(1000000):
    let x = obj.expensive_property  // Called 1M times!
```

**✅ Solution:** Cache the value

```desi
let cached = obj.expensive_property
for i in range(1000000):
    let x = cached  // No repeated calls
```

---

## Implementation Status

### ✅ Fully Implemented (Production Ready)

- **Basic Classes:** Fields, methods, constructors
- **Visibility:** `pub` keyword for fields and methods
- **Inheritance:** Single inheritance with method override
- **Generic Classes:** Type parameters with inference
- **Decorators:** `@staticmethod`, `@classmethod`, `@property`
- **RAII:** `__close__` with `using` blocks
- **Field Assignment:** `obj.field = value` syntax
- **Move Semantics:** Borrow checker integration
- **Name Mangling:** Static dispatch for all methods
- **Memory Layout:** C-compatible struct layout

### 🚧 Designed (Implementation Pending)

- **Copy Semantics:** `__copy__` dunder method
- **Operator Overloading:** `__add__`, `__eq__`, etc.
- **Abstract Methods:** `@abstract` decorator
- **Multiple Inheritance:** Multiple base classes with MRO

### 🔮 Future Considerations

- **Trait Objects:** Dynamic dispatch when needed
- **Metaclasses:** Runtime class manipulation
- **Dataclasses:** Auto-generate dunders with `@dataclass`
- **Custom Allocators:** Per-class allocation strategies

---

## Best Practices Summary

**DO:**
- ✅ Initialize all fields in constructors
- ✅ Use `pub` for public API
- ✅ Implement `__close__` for resources
- ✅ Use `@property` for computed values
- ✅ Use `@classmethod` for factory methods
- ✅ Keep methods small and focused
- ✅ Use explicit copies (`__copy__`)

**DON'T:**
- ❌ Leave fields uninitialized
- ❌ Forget `pub` on public members
- ❌ Use properties in tight loops without caching
- ❌ Rely on implicit copies
- ❌ Make `__close__` throw errors
- ❌ Put heavy logic in `__close__`

---

## Learning Resources

**Examples in Repository:**
- `examples/90_class_basic.desi` - Basic class usage
- `examples/91_class_new.desi` - Custom constructors
- `examples/92_class_methods.desi` - Method definitions
- `examples/94_class_raii.desi` - RAII with `__close__`
- `examples/95_class_inheritance.desi` - Inheritance
- `examples/96_class_generic.desi` - Generic classes
- `examples/100_class_staticmethod.desi` - `@staticmethod` decorator
- `examples/101_class_classmethod.desi` - `@classmethod` decorator
- `examples/102_class_property.desi` - `@property` decorator
- `examples/103_class_raii_test.desi` - Nested RAII
- `examples/104_class_visibility.desi` - Visibility examples
- `examples/106_class_field_assign.desi` - Field assignment

**Next Steps:**
1. Try the examples above
2. Build a simple data structure (Stack, Queue)
3. Implement a resource manager with RAII
4. Create a generic container class
5. Build a small application using classes

---

**This document is the definitive reference for classes in Desi.** For compiler internals and implementation details, see the technical sections or compiler source code.
