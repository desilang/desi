# sys Module

The `sys` module provides access to system streams (stdout, stderr).

## Quick Start

```desi
import sys

print("Error message", file=sys.stderr)
print("Normal output")  # goes to stdout by default
```

## Streams

| Symbol | Type | Description |
|--------|------|-------------|
| `sys.stdout` | `ptr` | Standard output stream |
| `sys.stderr` | `ptr` | Standard error stream |

## Usage

### Printing to stderr

```desi
import sys

print("Error: something went wrong", file=sys.stderr)
```

### Separating Output

Use stderr for error/diagnostic output and stdout for normal output:

```desi
import sys

def process(data: str):
    if len(data) == 0:
        print("Warning: empty input", file=sys.stderr)
        return
    print(f"Processing: {data}")
```

## Future Additions

The sys module will be expanded to include:
- `sys.argv` - Command line arguments
- `sys.exit(code)` - Exit with status code
- `sys.stdin` - Standard input stream

## See Also

- [Error Handling](../language/error-handling.md) - For handling errors
