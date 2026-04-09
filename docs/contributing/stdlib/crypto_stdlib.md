# crypto Module Implementation

The `crypto` module provides cryptographic hashing, HMAC, token generation, and password hashing with zero external dependencies.

## Architecture

```
import crypto
    ↓
compiler/lib/crypto/__mod.desi  →  @extern("C") bindings
    ↓
compiler/runtime/crypto.c  →  C functions (__crypto_*)
    ↓
compiler/runtime/hash.c  →  Reuses existing SHA/MD5 primitives
```

## Files

| File | Purpose |
|------|---------|
| `compiler/lib/crypto/__mod.desi` | Desi bindings |
| `compiler/runtime/crypto.c` | C runtime (wrappers around hash.c) |
| `compiler/runtime/hash.c` | Underlying SHA-256/512, MD5, SHA-1 implementations |

## C Runtime Functions

| C Function | Desi Binding | Return | Description |
|-----------|-------------|--------|-------------|
| `__crypto_sha256(data)` | `sha256(data)` | `str` | SHA-256 hex digest |
| `__crypto_sha512(data)` | `sha512(data)` | `str` | SHA-512 hex digest |
| `__crypto_md5(data)` | `md5(data)` | `str` | MD5 hex digest |
| `__crypto_sha1(data)` | `sha1(data)` | `str` | SHA-1 hex digest |
| `__crypto_hmac_sha256(key, msg)` | `hmac_sha256(key, msg)` | `str` | HMAC-SHA256 |
| `__crypto_constant_time_equal(a, b)` | `constant_time_equal(a, b)` | `bool` | Timing-safe compare |
| `__crypto_token_hex(n)` | `token_hex(n)` | `str` | Random hex token |
| `__crypto_hash_password(pw)` | `hash_password(pw)` | `str` | Salt+hash |
| `__crypto_verify_password(pw, hash)` | `verify_password(pw, hash)` | `bool` | Verify password |

## Design Decisions

- **Zero dependencies**: All hash algorithms are implemented in `hash.c` — no OpenSSL, no libcrypto
- **HMAC**: Implements RFC 2104 HMAC using SHA-256 internally
- **Token generation**: Uses `/dev/urandom` on POSIX, `BCryptGenRandom` on Windows
- **Password format**: `salt:hash` where salt is 16 random bytes, hash is SHA-256 of `salt+password`
- **Constant-time compare**: Iterates full length regardless of mismatch position

## Security Notes

- MD5 and SHA-1 are provided for compatibility only — not recommended for security
- Password hashing uses SHA-256 with salt — adequate for most use cases, but not bcrypt/argon2 level
- `token_hex()` is cryptographically secure (reads from OS CSPRNG)
