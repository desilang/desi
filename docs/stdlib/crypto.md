# Crypto Module Implementation

This document covers the internal implementation details of the `crypto` module for contributors.

## Architecture

```
compiler/lib/crypto/__mod.desi  ← Desi public API (9 functions)
         ↓ (extern "C" calls)
compiler/runtime/crypto.c      ← C wrapper (__crypto_* functions)
         ↓ (calls)
compiler/runtime/hash.c        ← SHA-256/512, MD5, SHA-1 implementations
```

## Zero-Dependency Design

The crypto module reuses the existing `hash.c` which contains full implementations of:
- SHA-256 (used by `hash` module already)
- SHA-512
- MD5
- SHA-1

The `crypto.c` wrapper adds:
- HMAC-SHA256 (RFC 2104)
- Constant-time comparison
- Cryptographic random token generation
- Password hashing with salt

## HMAC Implementation

Follows RFC 2104:
```
HMAC(K, m) = H((K' ⊕ opad) || H((K' ⊕ ipad) || m))
```

Where:
- `K'` = key (padded/hashed to block size)
- `ipad` = 0x36 repeated
- `opad` = 0x5c repeated
- Block size = 64 bytes (SHA-256)

## Password Hashing

Format: `salt_hex:hash_hex`

1. Generate 16 random bytes as salt
2. Concatenate `salt + password`
3. SHA-256 hash the concatenation
4. Store as `hex(salt):hex(hash)`

Verification: split on `:`, reconstruct salt+password, compare hashes.

## Random Token Generation

Platform-specific CSPRNG:
- **macOS/Linux**: reads from `/dev/urandom`
- **Windows**: uses `BCryptGenRandom()` from bcrypt.lib
- **Fallback**: `rand()` seeded with `time() ^ getpid()` (not cryptographically secure)
