# Membership Operator

The `in` operator checks if a value exists in a collection.

## With Lists

```desi
let nums = [10, 20, 30, 40, 50]

if 20 in nums:
    print("Found 20!")

if 99 in nums:
    print("Won't print - 99 not in list")
```

## With Sets

```desi
let colors = #{"red", "green", "blue"}

if "red" in colors:
    print("Red is in the set")
```

## With Strings (Substring Check)

```desi
let message = "Hello, World!"

if "World" in message:
    print("Found 'World' in message")

if "Desi" in message:
    print("Won't print")
```

## With Tuples

```desi
let coords = (1, 2, 3, 4, 5)

if 3 in coords:
    print("Found 3 in tuple")
```

## Custom Classes

Classes can implement `__contains__` to support the `in` operator:

```desi
class MySet:
    pub data: list[int]
    
    pub def __new__(self, values: list[int]):
        self.data = values
    
    pub def __contains__(self, item: int) -> bool:
        # Custom logic to check membership
        for x in self.data:
            if x == item:
                return true
        return false

let s = MySet([10, 20, 30, 40, 50])

if 20 in s:
    print("20 is in MySet")  # This prints

if 99 in s:
    print("Won't print")
```

!!! tip
    The `__contains__` method decides how to compare elements. Use value equality (like `==`) for intuitive behavior.

## Negation

Negate the whole membership test with `not`:

```desi
let nums = [1, 2, 3]

if not (5 in nums):
    print("5 is not in the list")
```

!!! note "There is no `not in` operator"
    Python's `x not in y` is not Desi syntax. Write `not (x in y)` — the
    parentheses are required, because `not` binds tighter than `in`.
