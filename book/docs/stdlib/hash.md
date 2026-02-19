# Hash Module

The `hash` module provides cryptographic hash functions: MD5, SHA-256, SHA-512, HMAC, and file hashing. Uses CommonCrypto on macOS.

## Import

```desi
import hash
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `md5(s)` | `str` | MD5 hex digest (32 chars) |
| `sha256(s)` | `str` | SHA-256 hex digest (64 chars) |
| `sha512(s)` | `str` | SHA-512 hex digest (128 chars) |
| `hmac_sha256(key, msg)` | `str` | HMAC-SHA256 hex digest |
| `hash_file(path)` | `str` | SHA-256 hash of file contents |

## Usage Example

```desi
import hash

def main() -> int:
    let digest = hash.sha256("password123")
    print(digest)

    # Verify file integrity
    let file_hash = hash.hash_file("data.bin")
    print(f"SHA-256: {file_hash}")

    # HMAC for API authentication
    let signature = hash.hmac_sha256("secret_key", "request_body")
    print(signature)
    0
```

## See Also

- [Base64 Module](base64.md) — Encoding/decoding
- [UUID Module](uuid.md) — UUID generation
