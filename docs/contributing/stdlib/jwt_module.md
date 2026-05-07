# JWT Module Implementation

Internal documentation for the `jwt` standard library module.

## Architecture

```
compiler/lib/jwt.desi        → Desi API (encode, verify, decode)
compiler/runtime/jwt.c       → C runtime (JWT creation, HMAC-SHA256 signing)
compiler/runtime/crypto.c    → C runtime (HMAC-SHA256 primitive)
compiler/runtime/base64.c    → C runtime (base64 encode/decode)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__jwt_encode` | `jwt.encode` | `(char* payload_json, char* secret) → char*` |
| `__jwt_decode` | `jwt.decode` | `(char* token, char* secret) → char*` |
| `__jwt_verify` | `jwt.verify` | `(char* token, char* secret) → int32` (→ bool) |
| `__jwt_get_payload` | `jwt.get_payload` | `(char* token) → char*` |
| `__jwt_get_header` | `jwt.get_header` | `(char* token) → char*` |

## Dependencies

The JWT module depends on two other runtime modules:

- **`__hash_hmac_sha256`** (from `crypto.c`) — computes HMAC-SHA256 signature
- **`__base64_encode` / `__base64_decode`** (from `base64.c`) — base64url encoding

**Note**: The function was originally named `__crypto_hmac_sha256` but was renamed to `__hash_hmac_sha256` to match the actual symbol in `crypto.c`.

## Implementation Details

### Token Structure

```
header.payload.signature
```

- **Header**: Always `{"alg":"HS256","typ":"JWT"}`, base64url-encoded
- **Payload**: User-provided JSON, base64url-encoded
- **Signature**: `HMAC-SHA256(secret, header.payload)`, hex-to-bytes, base64url-encoded

### Base64url Encoding

Standard base64 with `+` → `-`, `/` → `_`, and padding (`=`) stripped.

### Signing Flow

1. Base64url-encode header and payload
2. Compute `HMAC-SHA256(secret, base64header + "." + base64payload)` → hex string
3. Convert hex string to raw bytes
4. Base64url-encode the raw bytes → signature
5. Concatenate: `base64header.base64payload.signature`

### Verification Flow

1. Split token on `.` → 3 parts
2. Recompute signature from parts[0] + "." + parts[1] using the provided secret
3. Compare with parts[2]

## Security Considerations

- Only HS256 is supported (HMAC-SHA256). No RS256/ES256.
- No automatic `exp` claim enforcement — callers must check expiration manually.
- `get_payload()` and `get_header()` do NOT verify signatures.
- The HMAC implementation uses the standard two-pass approach with ipad/opad.

## Test Coverage

- `examples/508_jwt_module.desi` — encode, verify, wrong secret rejection, decode, header/payload extraction
- Cross-verified: header encoding matches reference base64url output exactly
