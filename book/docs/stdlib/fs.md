# File System Module

The `fs` module provides file system operations: reading, writing, copying, moving files and listing directories.

## Import

```desi
import fs
```

## API Reference

### Reading

| Function | Returns | Description |
|----------|---------|-------------|
| `read(path)` | `str` | Read entire file as string |
| `read_lines(path)` | `list[str]` | Read file as list of lines |

### Writing

| Function | Returns | Description |
|----------|---------|-------------|
| `write(path, data)` | `int` | Write string to file (0 = success) |
| `append(path, data)` | `int` | Append to file (0 = success) |
| `write_lines(path, lines)` | `int` | Write list of lines to file |

### File Info

| Function | Returns | Description |
|----------|---------|-------------|
| `exists(path)` | `bool` | True if path exists |
| `is_file(path)` | `bool` | True if regular file |
| `is_dir(path)` | `bool` | True if directory |
| `size(path)` | `int` | File size in bytes |

### File Operations

| Function | Returns | Description |
|----------|---------|-------------|
| `copy(src, dst)` | `int` | Copy file (0 = success) |
| `move(src, dst)` | `int` | Move/rename file (0 = success) |
| `remove(path)` | `int` | Delete file (0 = success) |
| `mkdir(path)` | `int` | Create directory (0 = success) |

### Listing

| Function | Returns | Description |
|----------|---------|-------------|
| `list_dir(path)` | `list[str]` | List directory contents |
| `abs(path)` | `str` | Get absolute path |

## Usage Examples

```desi
import fs

def main() -> int:
    # Write and read
    fs.write("todo.txt", "Buy milk\nWalk dog\n")
    let lines = fs.read_lines("todo.txt")
    for line in lines:
        print(f"  - {line}")

    # Copy and check
    fs.copy("todo.txt", "todo_backup.txt")
    print(f"Backup exists: {str(fs.exists('todo_backup.txt'))}")
    print(f"Size: {str(fs.size('todo.txt'))} bytes")

    # Cleanup
    fs.remove("todo.txt")
    fs.remove("todo_backup.txt")
    0
```

## See Also

- [OS Module](os.md) — System functions and file I/O
- [Path Module](path.md) — Path manipulation
