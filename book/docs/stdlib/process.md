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
| `process.run_timeout(cmd, args, secs) -> cptr` | Run with timeout (kills after N seconds) |
| `process.shell_timeout(cmd, secs) -> cptr` | Shell command with timeout |
| `process.kill(pid) -> bool` | Kill a process by PID (SIGKILL) |
| `process.send_signal(pid, sig) -> bool` | Send a signal to a process |

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

### Run with Timeout

```desi
import process

def main() -> int:
    # Kill the command if it takes more than 5 seconds
    let h = process.shell_timeout("sleep 30", 5)
    if process.get_ok(h) == false:
        print("Command timed out or failed")
        print(process.get_stderr(h))  # "process killed: timeout after 5 seconds"
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

## Comparison

| Desi | Python | Go |
|---|---|---|
| `process.shell(cmd)` | `subprocess.run(cmd, shell=True)` | `exec.Command("sh", "-c", cmd)` |
| `process.output(cmd, args)` | `subprocess.check_output([cmd])` | `exec.Command(cmd).Output()` |
| `process.shell_timeout(cmd, 5)` | `subprocess.run(cmd, timeout=5)` | `exec.CommandContext(ctx, cmd)` |
| `process.kill(pid)` | `os.kill(pid, SIGKILL)` | `process.Kill()` |
