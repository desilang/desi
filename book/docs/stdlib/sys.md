# sys Module

The `sys` module provides access to system internals: version info, memory layout, recursion limits, and runtime introspection.

## Import

```desi
import sys
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `version()` | `str` | Desi version string (e.g. `"0.1.0"`) |
| `maxsize()` | `int` | Maximum integer value for the platform |
| `byteorder()` | `str` | `"little"` or `"big"` endian |
| `sizeof_ptr()` | `int` | Size of a pointer in bytes (4 or 8) |
| `recursion_limit()` | `int` | Current max recursion depth |
| `call_depth()` | `int` | Current call stack depth |

## Usage Examples

### Version and Platform Info

```desi
import sys

def main() -> int:
    print(f"Desi v{sys.version()}")
    print(f"Pointer size: {str(sys.sizeof_ptr())} bytes")
    print(f"Byte order: {sys.byteorder()}")
    print(f"Max int: {str(sys.maxsize())}")
    0
```

### Recursion Monitoring

```desi
import sys

def fibonacci(n: int) -> int:
    let depth = sys.call_depth()
    if depth > sys.recursion_limit() - 10:
        print("Warning: approaching recursion limit!")
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)
```

## See Also

- [OS Module](os.md) — Operating system functions
- [IO Module](io.md) — Input/output
