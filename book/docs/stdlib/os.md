# OS Module

The `os` module provides access to operating system functions: environment variables, current directory, platform detection, and program exit.

## Import

```desi
import os
```

## Functions

| Function | Returns | Description |
|----------|---------|-------------|
| `os.platform()` | `str` | OS name: `"darwin"`, `"linux"`, or `"windows"` |
| `os.getenv(key)` | `str` | Environment variable value, or `""` if not set |
| `os.getcwd()` | `str` | Current working directory |
| `os.exit(code)` | — | Terminate program with exit code |

## Usage

```desi
import os

def main() -> int:
    let platform = os.platform()
    print(f"Running on {platform}")

    let home = os.getenv("HOME")
    print(f"Home: {home}")

    let cwd = os.getcwd()
    print(f"Working dir: {cwd}")
    0
```

### Platform Detection

```desi
import os

def main() -> int:
    let p = os.platform()
    if p == "darwin":
        print("macOS detected")
    elif p == "linux":
        print("Linux detected")
    0
```

### Environment Variables

```desi
import os

def main() -> int:
    let path = os.getenv("PATH")
    let missing = os.getenv("NONEXISTENT")
    print(f"PATH length: {len(path)}")
    print(f"Missing is empty: {len(missing) == 0}")
    0
```

### Early Exit

```desi
import os

def main() -> int:
    let key = os.getenv("API_KEY")
    if len(key) == 0:
        print("Error: API_KEY not set")
        os.exit(1)
    0
```

## See Also

- [sys Module](sys.md) — Standard streams (`stdout`, `stderr`)
- [Log Module](log.md) — Structured logging
