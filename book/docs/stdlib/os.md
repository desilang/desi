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

## See Also

- [Strings Module](strings.md) — String manipulation
- [Log Module](log.md) — Structured logging
