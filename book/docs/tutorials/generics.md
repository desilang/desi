# Generics

!!! info "Coming Soon"
    This tutorial is under development for v0.1.0.

## Overview

Desi supports generic types:

```desi
class Box<T>:
    pub mut val: T
    
    pub def __new__(self, v: T):
        self.val = v

def main():
    let int_box = Box(42)           # Box<int>
    let str_box = Box("hello")      # Box<str>
    
    print(int_box.val)
    print(str_box.val)
```

---

## Topics Covered

- Generic classes
- Generic functions
- Type inference
- Multiple type parameters
- Constraints (future)
