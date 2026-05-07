# shell — Shell Utilities & File Operations

Execute shell commands and perform common file system operations.

## Import

```desi
import shell
```

## API Reference

### Command Execution

| Function | Description |
|---|---|
| `shell.exec(cmd) -> str` | Run a command, return stdout |
| `shell.exec_status(cmd) -> int` | Run a command, return exit code |
| `shell.which(cmd) -> str` | Find command path (empty if not found) |
| `shell.glob(pattern) -> str` | Expand a glob pattern |

### File Operations

| Function | Description |
|---|---|
| `shell.cat(path) -> str` | Read file contents |
| `shell.write(path, content) -> int` | Write content to file |
| `shell.append(path, content) -> int` | Append content to file |
| `shell.lines(path) -> int` | Count lines in a file |
| `shell.cp(src, dst) -> int` | Copy a file |
| `shell.mv(src, dst) -> int` | Move/rename a file |
| `shell.rm(path) -> int` | Delete a file |

### Directory Operations

| Function | Description |
|---|---|
| `shell.mkdir_p(path) -> int` | Create directory (with parents) |
| `shell.rmdir(path) -> int` | Remove an empty directory |

### Path Checks

| Function | Description |
|---|---|
| `shell.exists(path) -> bool` | Check if path exists |
| `shell.is_file(path) -> bool` | Check if path is a file |
| `shell.is_dir(path) -> bool` | Check if path is a directory |

## Examples

### Basic Shell Usage

```desi
import shell

def main() -> int:
    # Run a command
    let output = shell.exec("uname -s")
    print(output)  # Darwin / Linux

    # Check exit status
    let code = shell.exec_status("test -f /etc/hosts")
    print(code)  # 0

    # Find a command
    let path = shell.which("python3")
    print(path)  # /usr/bin/python3
    0
```

### File I/O

```desi
import shell

def main() -> int:
    # Write and read
    shell.write("/tmp/hello.txt", "Hello, Desi!")
    let content = shell.cat("/tmp/hello.txt")
    print(content)  # Hello, Desi!

    # Append
    shell.append("/tmp/hello.txt", "\nMore text")
    print(shell.lines("/tmp/hello.txt"))  # 2

    # Cleanup
    shell.rm("/tmp/hello.txt")
    print(shell.exists("/tmp/hello.txt"))  # false
    0
```

### Directory Management

```desi
import shell

def main() -> int:
    shell.mkdir_p("/tmp/my_project/src")
    print(shell.is_dir("/tmp/my_project/src"))  # true

    shell.write("/tmp/my_project/src/main.desi", "# code here")
    shell.cp("/tmp/my_project/src/main.desi", "/tmp/my_project/src/backup.desi")
    print(shell.exists("/tmp/my_project/src/backup.desi"))  # true

    # Cleanup
    shell.rm("/tmp/my_project/src/main.desi")
    shell.rm("/tmp/my_project/src/backup.desi")
    shell.rmdir("/tmp/my_project/src")
    shell.rmdir("/tmp/my_project")
    0
```
