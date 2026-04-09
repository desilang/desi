# Crypto Module

The `crypto` module provides cryptographic hashing, HMAC, token generation, and password hashing — all with zero external dependencies.

## Import

```desi
import crypto
```

## API Reference

### Hashing

| Function | Returns | Description |
|----------|---------|-------------|
| `sha256(data)` | `str` | SHA-256 hex digest |
| `sha512(data)` | `str` | SHA-512 hex digest |
| `md5(data)` | `str` | MD5 hex digest |
| `sha1(data)` | `str` | SHA-1 hex digest |

### HMAC

| Function | Returns | Description |
|----------|---------|-------------|
| `hmac_sha256(key, msg)` | `str` | HMAC-SHA256 hex digest |

### Comparison

| Function | Returns | Description |
|----------|---------|-------------|
| `constant_time_equal(a, b)` | `bool` | Timing-safe string comparison |

### Token Generation

| Function | Returns | Description |
|----------|---------|-------------|
| `token_hex(n)` | `str` | Generate `n` random hex bytes (2n chars) |

### Password Hashing

| Function | Returns | Description |
|----------|---------|-------------|
| `hash_password(password)` | `str` | Hash password with random salt (SHA-256) |
| `verify_password(password, hash)` | `bool` | Verify password against stored hash |

## Usage Examples

### Basic Hashing

```desi
import crypto

def main() -> int:
    let data = "Hello, Desi!"

    print(f"SHA-256: {crypto.sha256(data)}")
    print(f"SHA-512: {crypto.sha512(data)}")
    print(f"MD5: {crypto.md5(data)}")
    print(f"SHA-1: {crypto.sha1(data)}")
    0
```

### API Authentication

```desi
import crypto

def sign_request(secret: str, payload: str) -> str:
    return crypto.hmac_sha256(secret, payload)

def verify_signature(secret: str, payload: str, signature: str) -> bool:
    let expected = crypto.hmac_sha256(secret, payload)
    return crypto.constant_time_equal(expected, signature)
```

### Session Tokens

```desi
import crypto

# Generate a 32-byte (64-character) random token
let session_token = crypto.token_hex(32)
print(f"Session: {session_token}")
```

### Password Storage

```desi
import crypto

# Registration: hash the password
let hashed = crypto.hash_password("mypassword123")
# Store `hashed` in database

# Login: verify the password
let valid = crypto.verify_password("mypassword123", hashed)
if valid:
    print("Login successful!")
```

> [!WARNING]
> The `md5()` and `sha1()` functions are provided for **compatibility only**.
> For security-sensitive applications, always use `sha256()` or `sha512()`.

> [!NOTE]
> Password hashing uses SHA-256 with a random salt. The hash format is `salt:hash` where both are hex-encoded.

## See Also

- [Hash Module](hash.md) — Lower-level hash functions
- [UUID Module](uuid.md) — UUID generation
