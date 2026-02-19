# UUID Module

The `uuid` module generates RFC 4122 v4 UUIDs using `/dev/urandom`.

## Import

```desi
import uuid
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `v4()` | `str` | Generate random UUID v4 |
| `is_valid(s)` | `bool` | Check UUID format: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` |
| `nil()` | `str` | Nil UUID: `00000000-0000-0000-0000-000000000000` |

## Usage Example

```desi
import uuid

def main() -> int:
    let id = uuid.v4()
    print(f"Session: {id}")

    if uuid.is_valid(id):
        print("Valid UUID")
    0
```

## See Also

- [Random Module](random.md) — General random number generation
- [Hash Module](hash.md) — Cryptographic hashing
