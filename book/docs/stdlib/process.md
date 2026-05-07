# process — Subprocess Execution

Run external commands and capture their output.

## Import

```desi
import process
```

## API Reference

| Function | Description |
|---|---|
| `process.run(cmd, args) -> cptr` | Run a command with arguments |
| `process.shell(cmd_str) -> cptr` | Run a shell command via `/bin/sh -c` |
| `process.get_stdout(h) -> str` | Get captured stdout |
| `process.get_stderr(h) -> str` | Get captured stderr |
| `process.get_exit_code(h) -> int` | Get exit code |
| `process.get_ok(h) -> bool` | `true` if exit code is 0 |
| `process.output(cmd, args) -> str` | Run command, return stdout directly |
| `process.shell_output(cmd_str) -> str` | Shell command, return stdout directly |
| `process.free(h) -> int` | Free a process result handle |
| `process.pid() -> int` | Get current process ID |
| `process.ppid() -> int` | Get parent process ID |

## Examples

### Run a Shell Command

```desi
import process

def main() -> int:
    let h = process.shell("echo hello world")
    print(process.get_stdout(h))   # hello world
    print(process.get_ok(h))       # true
    print(process.get_exit_code(h)) # 0
    process.free(h)
    0
```

### Capture Output Directly

```desi
import process

def main() -> int:
    let output = process.shell_output("date +%Y")
    print(output)  # 2026
    0
```

### Check for Failure

```desi
import process

def main() -> int:
    let h = process.shell("exit 1")
    if process.get_ok(h) == false:
        print("Command failed!")
        print(process.get_exit_code(h))  # 1
    process.free(h)
    0
```

### Process Info

```desi
import process

def main() -> int:
    print(f"PID: {str(process.pid())}")
    print(f"Parent PID: {str(process.ppid())}")
    0
```
