# File System Module

The `fs` module provides comprehensive file system operations: reading, writing, copying, moving files, recursive directory operations, temporary files, metadata, and executable lookup.

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
| `write(path, data)` | `bool` | Write string to file |
| `append(path, data)` | `bool` | Append to file |
| `write_lines(path, lines)` | `bool` | Write list of lines to file |

### File Info

| Function | Returns | Description |
|----------|---------|-------------|
| `exists(path)` | `bool` | True if path exists |
| `is_file(path)` | `bool` | True if regular file |
| `is_dir(path)` | `bool` | True if directory |
| `size(path)` | `int` | File size in bytes |
| `mtime(path)` | `int` | Modification time (Unix timestamp) |
| `permissions(path)` | `int` | File permission bits (octal) |

### File Operations

| Function | Returns | Description |
|----------|---------|-------------|
| `copy(src, dst)` | `bool` | Copy file |
| `move(src, dst)` | `bool` | Move/rename file |
| `remove(path)` | `bool` | Delete file or empty directory |
| `mkdir(path)` | `bool` | Create directory (with parents) |
| `chmod(path, mode)` | `bool` | Set file permissions |

### Recursive Operations

| Function | Returns | Description |
|----------|---------|-------------|
| `copy_dir(src, dst)` | `bool` | Recursive directory copy (like `cp -r`) |
| `remove_all(path)` | `bool` | Recursive delete (like `rm -rf`) |
| `walk(path)` | `list[str]` | Recursively list all paths |

### Listing & Lookup

| Function | Returns | Description |
|----------|---------|-------------|
| `list_dir(path)` | `list[str]` | List directory contents |
| `abs(path)` | `str` | Get absolute path |
| `which(cmd)` | `str` | Find executable on PATH |

### Temporary Files

| Function | Returns | Description |
|----------|---------|-------------|
| `temp_file()` | `str` | Create temp file, return path |
| `temp_dir()` | `str` | Create temp directory, return path |

## Usage Examples

### Read and Write

```desi
import fs

def main() -> int:
    fs.write("todo.txt", "Buy milk\nWalk dog\n")
    let lines = fs.read_lines("todo.txt")
    for line in lines:
        print(f"  - {line}")

    fs.copy("todo.txt", "todo_backup.txt")
    let exists = fs.exists("todo_backup.txt")
    let size = fs.size("todo.txt")
    print(f"Backup exists: {str(exists)}")
    print(f"Size: {str(size)} bytes")

    fs.remove("todo.txt")
    fs.remove("todo_backup.txt")
    0
```

### Recursive Directory Operations

```desi
import fs

def main() -> int:
    # Create a directory tree and copy it
    fs.mkdir("project/src")
    fs.write("project/src/main.desi", "print('hello')")
    
    fs.copy_dir("project", "project_backup")
    print(fs.exists("project_backup/src/main.desi"))  # true
    
    # Walk all files recursively
    let files = fs.walk("project")
    for f in files:
        print(f)
    
    # Clean up
    fs.remove_all("project")
    fs.remove_all("project_backup")
    0
```

### Temporary Files

```desi
import fs

def main() -> int:
    let tmp = fs.temp_file()
    fs.write(tmp, "temporary data")
    print(fs.read(tmp))
    fs.remove(tmp)
    
    let tmpdir = fs.temp_dir()
    fs.write(tmpdir + "/notes.txt", "scratch work")
    fs.remove_all(tmpdir)
    0
```

### Find Executables

```desi
import fs

def main() -> int:
    let python = fs.which("python3")
    if python != "":
        print(f"Python found at: {python}")
    else:
        print("Python not found")
    0
```

## Comparison

| Desi | Python | Go |
|---|---|---|
| `fs.read(path)` | `open(path).read()` | `os.ReadFile(path)` |
| `fs.walk(path)` | `os.walk(path)` | `filepath.Walk(path)` |
| `fs.copy_dir(s, d)` | `shutil.copytree(s, d)` | Manual recursion |
| `fs.which(cmd)` | `shutil.which(cmd)` | `exec.LookPath(cmd)` |
| `fs.temp_file()` | `tempfile.mkstemp()` | `os.CreateTemp()` |
| `fs.remove_all(p)` | `shutil.rmtree(p)` | `os.RemoveAll(p)` |

## See Also

- [OS Module](os.md) — System functions and file I/O
- [Path Module](path.md) — Path manipulation


