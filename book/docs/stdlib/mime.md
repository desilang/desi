# mime — MIME Type Detection

Detect MIME types from file extensions and paths.

## Import

```desi
import mime
```

## API Reference

| Function | Description |
|---|---|
| `mime.from_ext(ext) -> str` | MIME type for a file extension (e.g. `"json"`) |
| `mime.from_path(path) -> str` | MIME type for a file path |
| `mime.ext_for(mime_type) -> str` | File extension for a MIME type |
| `mime.is_text(mime_type) -> bool` | Check if MIME type is text-based |
| `mime.is_binary(mime_type) -> bool` | Check if MIME type is binary |

## Examples

### Detect MIME Types

```desi
import mime

def main() -> int:
    print(mime.from_ext("json"))           # application/json
    print(mime.from_ext("html"))           # text/html
    print(mime.from_path("photo.png"))     # image/png
    print(mime.from_path("data.csv"))      # text/csv

    print(mime.ext_for("application/pdf")) # pdf
    0
```

### Check Type Category

```desi
import mime

def main() -> int:
    print(mime.is_text("text/html"))       # true
    print(mime.is_text("application/json"))  # true
    print(mime.is_binary("image/png"))      # true
    print(mime.is_binary("text/plain"))     # false
    0
```
