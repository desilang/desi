# encoding — Hex and Base64 Encoding

The `encoding` module provides hex and base64 encoding/decoding utilities.

## Import

```desi
import encoding
```

## API Reference

### Hex

| Function | Description |
|---|---|
| `encoding.hex_encode(data: str) -> str` | Encode to lowercase hex |
| `encoding.hex_decode(hex: str) -> str` | Decode hex string |
| `encoding.hex_encode_upper(data: str) -> str` | Encode to uppercase hex |

### Base64

| Function | Description |
|---|---|
| `encoding.base64_encode(data: str) -> str` | Encode to base64 (RFC 4648) |
| `encoding.base64_decode(data: str) -> str` | Decode base64 string |
| `encoding.base64_url_encode(data: str) -> str` | URL-safe base64 (no padding) |
| `encoding.base64_url_decode(data: str) -> str` | Decode URL-safe base64 |

## Examples

### Hex encoding

```desi
import encoding

let hex = encoding.hex_encode("Hello")
print(hex)  # 48656c6c6f

let decoded = encoding.hex_decode(hex)
print(decoded)  # Hello

let upper = encoding.hex_encode_upper("Hello")
print(upper)  # 48656C6C6F
```

### Base64 encoding

```desi
import encoding

let b64 = encoding.base64_encode("Hello, World!")
print(b64)  # SGVsbG8sIFdvcmxkIQ==

let decoded = encoding.base64_decode(b64)
print(decoded)  # Hello, World!
```

### URL-safe base64

```desi
import encoding

# URL-safe variant: uses - and _ instead of + and /, no padding
let url_b64 = encoding.base64_url_encode("data with special chars?!")
print(url_b64)

let decoded = encoding.base64_url_decode(url_b64)
print(decoded)
```
