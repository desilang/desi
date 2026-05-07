# url — URL Parsing & Building

Parse, encode, decode, and construct URLs.

## Import

```desi
import url
```

## API Reference

| Function | Description |
|---|---|
| `url.parse(raw) -> cptr` | Parse a URL string into a handle |
| `url.get_scheme(h) -> str` | Get scheme (e.g. `"https"`) |
| `url.get_host(h) -> str` | Get hostname |
| `url.get_port(h) -> int` | Get port number (0 if not specified) |
| `url.get_path(h) -> str` | Get path component |
| `url.get_query(h) -> str` | Get raw query string |
| `url.get_fragment(h) -> str` | Get fragment (after `#`) |
| `url.encode(raw) -> str` | Percent-encode a string |
| `url.decode(encoded) -> str` | Decode a percent-encoded string |
| `url.query_get(qs, key) -> str` | Extract a value from a query string |
| `url.build(scheme, host, port, path, query, fragment) -> str` | Construct a URL from parts |
| `url.join(base, relative) -> str` | Resolve a relative URL against a base |
| `url.free(h) -> int` | Free a parsed URL handle |

## Examples

### Parse a URL

```desi
import url

def main() -> int:
    let h = url.parse("https://example.com:8080/api/v1?page=1#top")
    print(url.get_scheme(h))    # https
    print(url.get_host(h))      # example.com
    print(url.get_port(h))      # 8080
    print(url.get_path(h))      # /api/v1
    print(url.get_query(h))     # page=1
    print(url.get_fragment(h))  # top
    url.free(h)
    0
```

### Build a URL

```desi
import url

def main() -> int:
    let result = url.build("https", "api.example.com", 443, "/v2/users", "active=true", "")
    print(result)  # https://api.example.com:443/v2/users?active=true
    0
```

### Encode & Decode

```desi
import url

def main() -> int:
    let encoded = url.encode("hello world & more")
    print(encoded)  # hello%20world%20%26%20more

    let decoded = url.decode(encoded)
    print(decoded)  # hello world & more
    0
```

### Query String Extraction

```desi
import url

def main() -> int:
    let page = url.query_get("page=2&sort=name&limit=10", "sort")
    print(page)  # name
    0
```
