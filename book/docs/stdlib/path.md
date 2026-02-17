# Path Module

The `path` module provides filesystem path manipulation and querying functions. These functions operate on path strings — they do NOT modify the filesystem.

## Import

```desi
import path
```

## API Reference

### Queries

| Function | Returns | Description |
|----------|---------|-------------|
| `exists(p)` | `bool` | Check if path exists |
| `is_file(p)` | `bool` | Check if path is a regular file |
| `is_dir(p)` | `bool` | Check if path is a directory |
| `is_abs(p)` | `bool` | Check if path is absolute |
| `size(p)` | `int` | File size in bytes, -1 on error |

### Manipulation

| Function | Returns | Description |
|----------|---------|-------------|
| `basename(p)` | `str` | Last component: `"/a/b.txt"` → `"b.txt"` |
| `dirname(p)` | `str` | Parent directory: `"/a/b.txt"` → `"/a"` |
| `ext(p)` | `str` | File extension: `"file.txt"` → `".txt"` |
| `stem(p)` | `str` | Name without extension: `"file.txt"` → `"file"` |
| `join(a, b)` | `str` | Join path components |
| `abs(p)` | `str` | Resolve to absolute path |
| `clean(p)` | `str` | Normalize `.`, `..`, duplicate `/` |

## Usage Example

```desi
import path

def main() -> int:
    let file = path.join("/home/user", "data.csv")
    print(f"File: {file}")
    print(f"Dir:  {path.dirname(file)}")
    print(f"Name: {path.basename(file)}")
    print(f"Ext:  {path.ext(file)}")

    if path.exists(file):
        print(f"Size: {path.size(file)} bytes")
    0
```

## See Also

- [OS Module](os.md) — System operations (`getcwd`, `chdir`)
