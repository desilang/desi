# Validate Module

The `validate` module provides common input validation functions. **No other language ships this built-in** — developers always need a third-party library for these checks.

## Import

```desi
import validate
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `is_email(s)` | `bool` | RFC-ish email format check |
| `is_url(s)` | `bool` | HTTP/HTTPS URL check |
| `is_ipv4(s)` | `bool` | IPv4 address (0-255.x.x.x) |
| `is_ipv6(s)` | `bool` | IPv6 address format |
| `is_hex(s)` | `bool` | Hex string (with optional 0x prefix) |
| `is_json(s)` | `bool` | Valid JSON format check |
| `is_uuid(s)` | `bool` | UUID format (8-4-4-4-12) |
| `is_semver(s)` | `bool` | Semantic version (1.2.3) |

## Usage Example

```desi
import validate

def main() -> int:
    let email = "user@example.com"
    if validate.is_email(email):
        print("Valid email")

    if validate.is_ipv4("192.168.1.1"):
        print("Valid IPv4")

    if validate.is_semver("2.1.0"):
        print("Valid semver")
    0
```

## See Also

- [Re Module](re.md) — Regular expressions for custom validation
- [Strings Module](strings.md) — String manipulation
