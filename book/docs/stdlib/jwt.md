# jwt — JSON Web Tokens

Create, verify, and decode JWT tokens using HMAC-SHA256.

## Import

```desi
import jwt
```

## API Reference

| Function | Description |
|---|---|
| `jwt.encode(payload_json, secret) -> str` | Create a signed JWT token |
| `jwt.verify(token, secret) -> bool` | Verify a token's signature |
| `jwt.decode(token, secret) -> str` | Verify and return the payload JSON |
| `jwt.get_payload(token) -> str` | Extract payload **without** verification |
| `jwt.get_header(token) -> str` | Extract header **without** verification |

## Algorithm

All tokens use **HS256** (HMAC-SHA256). The header is always:

```json
{"alg": "HS256", "typ": "JWT"}
```

## Examples

### Create and Verify a Token

```desi
import jwt

def main() -> int:
    let secret = "my-secret-key-256-bits"
    let payload = "{\"sub\":\"user123\",\"role\":\"admin\",\"exp\":9999999999}"

    # Create token
    let token = jwt.encode(payload, secret)
    print(token)  # eyJhbGci...

    # Verify signature
    print(jwt.verify(token, secret))          # true
    print(jwt.verify(token, "wrong-secret"))  # false

    # Decode (verified)
    let decoded = jwt.decode(token, secret)
    print(decoded)  # {"sub":"user123","role":"admin","exp":9999999999}

    # Wrong secret returns empty string
    let bad = jwt.decode(token, "wrong-secret")
    print(bad == "")  # true
    0
```

### Inspect Without Verification

```desi
import jwt

def main() -> int:
    let token = jwt.encode("{\"user\":\"alice\"}", "secret")

    # These do NOT verify the signature
    let header = jwt.get_header(token)
    print(header)   # {"alg":"HS256","typ":"JWT"}

    let payload = jwt.get_payload(token)
    print(payload)  # {"user":"alice"}
    0
```

## Security Notes

- Always use a strong, random secret key (at least 32 bytes).
- Check the `exp` claim manually to enforce token expiration.
- Never trust `get_payload()` or `get_header()` in security-sensitive contexts — always use `decode()` which verifies the signature first.
