# Dunder Methods

Dunder (double underscore) methods let you customize how your classes behave with Desi's built-in operators and functions.

## Constructor: `__new__`

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __new__(self, x: int, y: int):
        self.x = x
        self.y = y

let p = Point(10, 20)
```

## Indexing: `__getitem__` and `__setitem__`

```desi
class MyList:
    pub mut data: list[int]
    
    pub def __new__(self, values: list[int]):
        self.data = values
    
    pub def __getitem__(self, index: int) -> int:
        return self.data[index]
    
    pub def __setitem__(self, index: int, value: int):
        self.data[index] := value

let arr = MyList([10, 20, 30])
print(arr[0])    # 10 (uses __getitem__)
arr[0] := 999    # Uses __setitem__
print(arr[0])    # 999
```

!!! important
    Use `:=` for mutation, not `=`. This is Desi's syntax for reassignment.

## Length: `__len__`

```desi
class MyList:
    pub data: list[int]
    
    pub def __len__(self) -> int:
        return len(self.data)

let arr = MyList([10, 20, 30])
print(len(arr))  # 3
```

## Membership: `__contains__`

```desi
class MySet:
    pub data: list[int]
    
    pub def __new__(self, values: list[int]):
        self.data = values
    
    pub def __contains__(self, item: int) -> bool:
        for x in self.data:
            if x == item:
                return true
        return false

let s = MySet([10, 20, 30])

if 20 in s:
    print("Found!")  # This runs

if 99 in s:
    print("Won't run")
```

## String Representation: `__repr__`

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __repr__(self) -> str:
        return "Point(" + str(self.x) + ", " + str(self.y) + ")"

let p = Point(10, 20)
print(p)  # Point(10, 20)
```

## Comparison: `__eq__`, `__ne__`, `__lt__`, etc.

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __eq__(self, other: Point) -> bool:
        return self.x == other.x and self.y == other.y

let p1 = Point(10, 20)
let p2 = Point(10, 20)
let p3 = Point(5, 5)

print(p1 == p2)  # true
print(p1 == p3)  # false
```

## Truthiness: `__bool__`

Define custom truthiness for use in `if` conditions:

```desi
class Container:
    pub mut count: int
    
    pub def __new__(self, n: int):
        self.count := n
    
    pub def __bool__(self) -> bool:
        return self.count > 0

let empty = Container(0)
let filled = Container(5)

if empty:
    print("has items")  # Won't run
else:
    print("empty!")     # Runs

if filled:
    print("has items")  # Runs
```

This is useful for container-like classes, optional wrappers, or any object where "truthy/falsy" makes semantic sense.

## Arithmetic: `__add__`, `__sub__`, etc.

```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __add__(self, other: Point) -> Point:
        return Point(self.x + other.x, self.y + other.y)

let p1 = Point(10, 20)
let p2 = Point(5, 10)
let p3 = p1 + p2  # Point(15, 30)
```

## Slicing: `__getslice__`

```desi
class MyList:
    pub data: list[int]
    
    pub def __new__(self, values: list[int]):
        self.data = values
    
    pub def __getslice__(self, start: int, end: int) -> list[int]:
        return self.data[start:end]

let arr = MyList([10, 20, 30, 40, 50])
let slice = arr[1:4]  # Uses __getslice__ -> [20, 30, 40]
```

## Iteration: `__iter__` and `__next__`

Define custom iterators for use in `for` loops:

```desi
class Range:
    pub start: int
    pub end: int
    pub mut current: int
    
    pub def __new__(self, s: int, e: int):
        self.start := s
        self.end := e
        self.current := s
    
    pub def __iter__(self) -> Range:
        return self
    
    pub def __next__(self) -> Option<int>:
        if self.current < self.end:
            let val = self.current
            self.current := self.current + 1
            let result: Option<int> = Option.Some(val)
            return result
        let nothing: Option<int> = Option.Nothing
        return nothing

let range = Range(0, 5)
for i in range:
    print(i)  # 0, 1, 2, 3, 4
```

The iterator protocol:
- `__iter__(self)` returns an iterator object (often `self`)
- `__next__(self)` returns `Option.Some(value)` for each element, or `Option.Nothing` when exhausted

## Summary

| Dunder | Syntax | Description |
|--------|--------|-------------|
| `__new__` | `Class()` | Constructor |
| `__getitem__` | `obj[i]` | Index access |
| `__setitem__` | `obj[i] := v` | Index assignment |
| `__getslice__` | `obj[i:j]` | Slice access |
| `__len__` | `len(obj)` | Custom length |
| `__contains__` | `x in obj` | Membership |
| `__bool__` | `if obj:` | Custom truthiness |
| `__iter__` | `for x in obj:` | Return iterator |
| `__next__` | `for x in obj:` | Yield next element |
| `__repr__` | `print(obj)` | String representation |
| `__eq__` | `a == b` | Equality |
| `__add__` | `a + b` | Addition |
