# Mutability and Assignment

Desi has a clear distinction between immutable and mutable variables, and between initial binding and mutation.

## Variable Declaration

```desi
# Immutable variable (cannot be changed)
let name = "Alice"
let count = 10

# Mutable variable (can be changed)
let mut counter = 0
let mut data = [1, 2, 3]
```

## Assignment Operators

| Operator | Usage | Description |
|----------|-------|-------------|
| `=` | Initial binding | Used with `let` to create new variables |
| `:=` | Mutation | Used to change existing values |

### Initial Binding with `=`

```desi
let x = 10              # Bind x to 10
let message = "Hello"   # Bind message to string
let nums = [1, 2, 3]    # Bind nums to list
```

### Mutation with `:=`

```desi
let mut x = 10
x := 20                 # Mutate x from 10 to 20

let mut data = [1, 2, 3]
data[0] := 99           # Mutate list element

# Also works in class methods
self.value := new_value
```

!!! important
    Always use `:=` for mutation, never `=`. Using `=` for mutation will cause a compiler error.

## Immutable vs Mutable

### Immutable Variables

```desi
let data = [1, 2, 3]
data[0] := 99  # ERROR: cannot assign to element of immutable list
```

### Mutable Variables

```desi
let mut data = [1, 2, 3]
data[0] := 99  # OK
print(data)    # [99, 2, 3]
```

## Class Fields

Fields can be mutable or immutable:

```desi
class Person:
    pub name: str           # Immutable field
    pub mut age: int        # Mutable field
    
    pub def __new__(self, name: str, age: int):
        self.name = name
        self.age = age
    
    pub def have_birthday(self):
        self.age := self.age + 1  # OK - age is mutable
        # self.name := "Bob"      # ERROR - name is immutable
```

## In Loops

```desi
let mut sum = 0
for x in [1, 2, 3, 4, 5]:
    sum := sum + x
print(sum)  # 15
```

Or with augmented assignment:

```desi
let mut count = 0
while count < 10:
    count += 1  # Augmented assignment works too
```
