# Base64 Module

The `base64` module provides Base64 encoding and decoding, including URL-safe variants. RFC 4648 compliant.

## Import

```desi
import base64
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `encode(s)` | `str` | Standard Base64 encode |
| `decode(s)` | `str` | Standard Base64 decode |
| `url_encode(s)` | `str` | URL-safe Base64 (no padding, `+`→`-`, `/`→`_`) |
| `url_decode(s)` | `str` | URL-safe Base64 decode |

## Usage Example

```desi
import base64

def main() -> int:
    let encoded = base64.encode("Hello, World!")
    print(encoded)  # SGVsbG8sIFdvcmxkIQ==

    let decoded = base64.decode(encoded)
    print(decoded)  # Hello, World!

    # URL-safe for web applications
    let token = base64.url_encode("user:password")
    print(token)
    0
```

## See Also

- [Hash Module](hash.md) — Cryptographic hashing
