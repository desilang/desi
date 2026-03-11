# OS Module

The `os` module provides operating system functions for environment variables, process control, platform detection, and system commands.

## Import

```desi
import os
```

## API Reference

### Environment Variables

| Function | Returns | Description |
|----------|---------|-------------|
| `getenv(key)` | `str` | Get env var (empty if not set) |
| `setenv(key, val)` | `none` | Set env var |

### Platform & System Info

| Function | Returns | Description |
|----------|---------|-------------|
| `platform()` | `str` | Platform ID: `"darwin"`, `"linux"`, `"windows"` |
| `name()` | `str` | OS name: `"macOS"`, `"Linux"`, `"Windows"` |
| `arch()` | `str` | CPU architecture: `"arm64"`, `"x86_64"` |
| `hostname()` | `str` | System hostname |
| `cpu_count()` | `int` | Number of CPU cores |

### Process

| Function | Returns | Description |
|----------|---------|-------------|
| `getpid()` | `int` | Current process ID |
| `exit(code)` | `none` | Exit with status code |

### Directory

| Function | Returns | Description |
|----------|---------|-------------|
| `getcwd()` | `str` | Current working directory |
| `chdir(path)` | `int` | Change directory (0 = success) |

### Command Execution

| Function | Returns | Description |
|----------|---------|-------------|
| `system(cmd)` | `int` | Run shell command, returns exit code |

## Usage Examples

```desi
import os

def main() -> int:
    # Platform detection
    print(f"Running on {os.name()} ({os.arch()})")
    print(f"Hostname: {os.hostname()}")
    print(f"CPUs: {os.cpu_count()}")

    # Environment
    os.setenv("MY_APP_PORT", "8080")
    let port = os.getenv("MY_APP_PORT")
    print(f"Port: {port}")

    # Working directory
    let cwd = os.getcwd()
    print(f"CWD: {cwd}")
    0
```

---

## File I/O

Read and write files with simple one-line calls:

### Reading Files

```desi
import os

let content = os.read_file("config.json")
print(content)
```

Returns empty string if the file doesn't exist or can't be read.

### Writing Files

```desi
# Write (creates or overwrites)
os.write_file("output.txt", "Hello, Desi!")

# Append to existing file
os.append_file("log.txt", "New log entry\n")
```

Both return `0` on success, `-1` on error.

### File I/O API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `read_file(path)` | `str` | Read entire file as string |
| `write_file(path, data)` | `int` | Create/overwrite file (0 = success) |
| `append_file(path, data)` | `int` | Append to file (0 = success) |

---

## File System

Check paths, get file info, and manage directories:

```desi
import os

# Check if paths exist
print(os.exists("myfile.txt"))    # true/false
print(os.is_file("myfile.txt"))   # true if regular file
print(os.is_dir("mydir"))         # true if directory

# File size
let size = os.file_size("data.bin")  # bytes, or -1 on error

# Directory operations
os.mkdir("new_folder")
let files = os.listdir(".")       # ["file1.txt", "file2.desi", ...]
os.rmdir("empty_folder")

# File operations
os.rename("old.txt", "new.txt")
os.remove("unwanted.txt")
```

### File System API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `exists(path)` | `bool` | True if path exists |
| `is_file(path)` | `bool` | True if regular file |
| `is_dir(path)` | `bool` | True if directory |
| `file_size(path)` | `int` | Size in bytes (-1 on error) |
| `mkdir(path)` | `int` | Create directory (0 = success) |
| `rmdir(path)` | `int` | Remove empty directory (0 = success) |
| `remove(path)` | `int` | Delete file (0 = success) |
| `rename(old, new)` | `int` | Rename file/dir (0 = success) |
| `listdir(path)` | `list[str]` | List directory contents (excludes `.` and `..`) |

---

## Complete Example

```desi
import os

def main() -> int:
    # Platform detection
    print(f"Running on {os.name()} ({os.arch()})")
    print(f"CPUs: {os.cpu_count()}")

    # File I/O
    os.write_file("/tmp/hello.txt", "Hello from Desi!")
    let content = os.read_file("/tmp/hello.txt")
    print(content)

    # Directory listing
    let files = os.listdir(".")
    print(f"Files in current dir: {files}")

    # Cleanup
    os.remove("/tmp/hello.txt")
    0
```

## See Also

- [Strings Module](strings.md) — String manipulation
- [Path Module](path.md) — Path manipulation
- [Log Module](log.md) — Structured logging

