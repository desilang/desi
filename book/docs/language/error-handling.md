# Error Handling

!!! info "Coming Soon"
    This language reference is under development for v0.1.0.

## Overview

Desi uses `Option` and `Result` types for error handling:

```python
def divide(a: int, b: int) -> Option<int>:
    if b == 0:
        return Nothing
    return Some(a / b)
```
