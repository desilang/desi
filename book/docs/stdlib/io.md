# IO Module

The `io` module provides input/output functions for reading from stdin and writing to stderr.

## Import

```desi
import io
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `input(prompt)` | `str` | Read a line from stdin with optional prompt |
| `read_all()` | `str` | Read all of stdin until EOF |
| `eprint(msg)` | `none` | Print to stderr with newline |
| `ewrite(msg)` | `none` | Write to stderr without newline |
| `flush()` | `none` | Flush stdout |
| `has_input()` | `bool` | Check if stdin has data available (non-blocking) |

## Usage Examples

### Interactive Input

```desi
import io

def main() -> int:
    let name = io.input("Enter your name: ")
    let age = io.input("Enter your age: ")
    print(f"Hello {name}, you are {age} years old!")
    0
```

### Error Output

```desi
import io

def process(data: str):
    if len(data) == 0:
        io.eprint("Error: empty input")
        return
    print(f"Processing: {data}")
```

### Reading Piped Input

```desi
import io

def main() -> int:
    # Works with: echo "hello" | ./program
    if io.has_input():
        let data = io.read_all()
        print(f"Received: {data}")
    else:
        print("No piped input")
    0
```

### Progress Indicator

```desi
import io

def progress(current: int, total: int):
    io.ewrite(f"\rProgress: {str(current)}/{str(total)}")
    io.flush()
```

## See Also

- [OS Module](os.md) — File I/O and system functions
- [Strings Module](strings.md) — String manipulation
