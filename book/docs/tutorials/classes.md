# Classes

!!! info "Coming Soon"
    This tutorial is under development for v0.1.0.

## Overview

Desi has full object-oriented support:

```python
class Person:
    pub mut name: str
    pub mut age: int
    
    pub def __new__(self, name: str, age: int):
        self.name = name
        self.age = age
    
    pub def greet(self) -> str:
        return "Namaste, I am " + self.name

def main():
    let person = Person(\"Arjun\", 25)
    print(person.greet())
```

---

## Topics Covered

- Class definitions
- Fields and visibility
- Constructors (__new__)
- Methods
- Properties
- Inheritance
- Static methods
- Class methods
