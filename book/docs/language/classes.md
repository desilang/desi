# Classes

Classes in Desi provide object-oriented programming with Python-like syntax and Rust-inspired safety guarantees.

## Basic Syntax

```desi
class ClassName:
    # Fields
    pub field_name: Type
    
    # Methods
    pub def method_name(self) -> ReturnType:
        # method body
```

### Simple Class

```desi
class Counter:
    pub value: int
    
    pub def increment(self) -> none:
        self.value = self.value + 1
    
    pub def get(self) -> int:
        return self.value

def main():
    let c = Counter()
    c.increment()
    print(c.get())  # 1
```

## Fields

### Declaration

Fields are declared with visibility, optionally mutable, name, and type:

```desi
class Point:
    pub x: int           # Public, immutable after construction
    pub mut y: int       # Public, mutable
    _private: str        # Private (underscore prefix)
```

### Mutability

=== "Immutable Field"
    ```desi
    pub x: int
    # Can only be set during construction
    ```

=== "Mutable Field"
    ```desi
    pub mut x: int
    # Can be modified after construction
    ```

### Visibility

| Prefix | Visibility |
|--------|------------|
| `pub` | Public - accessible outside class |
| (none) | Private - only accessible within class |
| `_name` | Private by convention |

```desi
class User:
    pub name: str          # Public
    _password_hash: str    # Private
```

## Constructors

### Default Constructor

Classes automatically get a default constructor:

```desi
class Point:
    pub x: int
    pub y: int

def main():
    let p = Point()  # Default constructor
    # Fields initialized to default values (0 for int)
```

### Custom Constructor (__new__)

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __new__(x: int, y: int) -> Point:
        let p = Point()
        p.x = x
        p.y = y
        return p

def main():
    let p = Point.__new__(10, 20)
    # Or: let p = Point(x=10, y=20) if named params supported
```

## Methods

### Instance Methods

Methods receive `self` implicitly:

```desi
class Counter:
    pub mut value: int
    
    pub def increment(self) -> none:
        self.value = self.value + 1
    
    pub def decrement(self) -> none:
        self.value = self.value - 1
    
    pub def reset(self) -> none:
        self.value = 0
```

!!! tip "Implicit `self`"
    Instance methods automatically have access to `self`. You can omit `self` from the parameter list in the declaration - it's added implicitly.

### Static Methods

Use `@staticmethod` for methods that don't need instance access:

```desi
class Point:
    pub x: int
    pub y: int
    
    @staticmethod
    pub def origin() -> Point:
        return Point()
    
    @staticmethod
    pub def from_coords(x: int, y: int) -> Point:
        let p = Point()
        p.x = x
        p.y = y
        return p

def main():
    let p1 = Point.origin()
    let p2 = Point.from_coords(10, 20)
```

### Class Methods

Use `@classmethod` for methods that receive the class:

```desi
class Animal:
    pub name: str
    
    @classmethod
    pub def create(cls, name: str) -> Animal:
        let a = cls()
        a.name = name
        return a
```

## Special Methods (Dunders)

### String Representation

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __str__(self) -> str:
        return f"Point({self.x}, {self.y})"
    
    pub def __repr__(self) -> str:
        return f"Point(x={self.x}, y={self.y})"

def main():
    let p = Point()
    p.x = 10
    p.y = 20
    print(p)  # Uses __str__: Point(10, 20)
```

### Operator Overloading

```desi
class Vector:
    pub mut x: float
    pub mut y: float
    
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
    
    pub def __eq__(self, other: Vector) -> bool:
        return self.x == other.x and self.y == other.y

def main():
    let v1 = Vector()
    v1.x = 1.0
    v1.y = 2.0
    
    let v2 = Vector()
    v2.x = 3.0
    v2.y = 4.0
    
    let v3 = v1 + v2  # Calls __add__
    let v4 = v1 - v2  # Calls __sub__
    let eq = v1 == v2 # Calls __eq__
```

### Available Operators

| Method | Operator | Description |
|--------|----------|-------------|
| `__add__` | `+` | Addition |
| `__sub__` | `-` | Subtraction |
| `__mul__` | `*` | Multiplication |
| `__div__` | `/` | Division |
| `__eq__` | `==` | Equality |
| `__ne__` | `!=` | Not equal |
| `__lt__` | `<` | Less than |
| `__le__` | `<=` | Less or equal |
| `__gt__` | `>` | Greater than |
| `__ge__` | `>=` | Greater or equal |

### Length and Container

```desi
class Stack:
    _items: list[int]
    
    pub def __len__(self) -> int:
        return len(self._items)
    
    pub def __contains__(self, item: int) -> bool:
        return item in self._items
```

### Copy

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __copy__(self) -> Point:
        let p = Point()
        p.x = self.x
        p.y = self.y
        return p
```

## RAII and Cleanup

### __close__ Method

Called automatically when using `using` statement:

```desi
class Connection:
    pub path: str
    
    pub def __close__(self) -> none:
        print("Closing connection")
        # Cleanup logic

def main():
    using conn = Connection():
        # Use connection
        print("Connected")
    # __close__ called automatically here
```

### __del__ Destructor

Called when object is destroyed:

```desi
class Resource:
    pub def __del__(self) -> none:
        print("Resource freed")
```

## Inheritance

### Basic Inheritance

```desi
class Animal:
    pub name: str
    
    pub def speak(self) -> str:
        return "..."

class Dog(Animal):
    pub breed: str
    
    pub def speak(self) -> str:
        return "Woof!"

class Cat(Animal):
    pub def speak(self) -> str:
        return "Meow!"

def main():
    let dog = Dog()
    dog.name = "Rex"
    dog.breed = "Labrador"
    print(dog.speak())  # "Woof!"
```

### Parent Methods

Access parent class methods with `super`:

```desi
class Parent:
    pub def greet(self) -> str:
        return "Hello from parent"

class Child(Parent):
    pub def greet(self) -> str:
        let parent_msg = super.greet()
        return f"{parent_msg}, and hello from child!"
```

## Properties

Use `@property` for computed fields:

```desi
class Rectangle:
    pub width: int
    pub height: int
    
    @property
    pub def area(self) -> int:
        return self.width * self.height
    
    @property
    pub def perimeter(self) -> int:
        return 2 * (self.width + self.height)

def main():
    let r = Rectangle()
    r.width = 10
    r.height = 5
    print(r.area)       # 50 (accessed like a field)
    print(r.perimeter)  # 30
```

## Generic Classes

```desi
class Box<T>:
    pub value: T
    
    pub def get(self) -> T:
        return self.value
    
    pub def set(self, v: T) -> none:
        self.value = v

def main():
    let int_box = Box<int>()
    int_box.set(42)
    print(int_box.get())  # 42
    
    let str_box = Box<str>()
    str_box.set("hello")
    print(str_box.get())  # hello
```

See [Generics](generics.md) for more details.

## Abstract Classes

```desi
class Shape:
    @abstract
    pub def area(self) -> float:
        pass
    
    @abstract
    pub def perimeter(self) -> float:
        pass

class Circle(Shape):
    pub radius: float
    
    pub def area(self) -> float:
        return 3.14159 * self.radius * self.radius
    
    pub def perimeter(self) -> float:
        return 2 * 3.14159 * self.radius
```

## Best Practices

### ✅ Do

- **Use `pub` for public API**: Explicit visibility
- **Keep classes focused**: Single responsibility
- **Use properties for computed values**: Clean interface
- **Implement `__str__` for debugging**: Easy to print
- **Use inheritance sparingly**: Prefer composition

### ❌ Don't

- **Don't expose internal state**: Keep fields private when possible
- **Avoid deep inheritance hierarchies**: Hard to maintain
- **Don't overload too many operators**: Can be confusing
- **Don't forget cleanup**: Implement `__close__` for resources

## Common Patterns

### Factory Pattern

```desi
class User:
    pub name: str
    pub email: str
    
    @staticmethod
    pub def create(name: str, email: str) -> User:
        let u = User()
        u.name = name
        u.email = email
        return u
    
    @staticmethod
    pub def guest() -> User:
        return User.create("Guest", "guest@example.com")
```

### Builder Pattern

```desi
class QueryBuilder:
    pub mut table: str
    pub mut conditions: list[str]
    
    pub def from_table(self, name: str) -> QueryBuilder:
        self.table = name
        return self
    
    pub def where(self, condition: str) -> QueryBuilder:
        self.conditions.append(condition)
        return self
    
    pub def build(self) -> str:
        return f"SELECT * FROM {self.table}"
```

### Data Class

```desi
class Person:
    pub name: str
    pub age: int
    pub email: str
    
    pub def __eq__(self, other: Person) -> bool:
        return self.name == other.name and 
               self.age == other.age and
               self.email == other.email
    
    pub def __repr__(self) -> str:
        return f"Person(name={self.name}, age={self.age})"
```

## See Also

- [Generics](generics.md) - Generic classes and methods
- [Functions](functions.md) - Method definitions
- [Error Handling](error-handling.md) - Custom error types with classes
