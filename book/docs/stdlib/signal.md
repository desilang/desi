# signal — OS Signal Handling

Register handlers for Unix signals like SIGINT, SIGTERM, and more.

## Import

```desi
import signal
```

## Signal Constants

| Constant | Value | Description |
|---|---|---|
| `signal.SIGINT` | 2 | Interrupt (Ctrl+C) |
| `signal.SIGTERM` | 15 | Termination |
| `signal.SIGHUP` | 1 | Hangup |
| `signal.SIGQUIT` | 3 | Quit |
| `signal.SIGUSR1` | 10 | User-defined 1 |
| `signal.SIGUSR2` | 12 | User-defined 2 |
| `signal.SIGALRM` | 14 | Alarm |
| `signal.SIGPIPE` | 13 | Broken pipe |

## API Reference

| Function | Description |
|---|---|
| `signal.on(signum, handler)` | Register a handler for a signal |
| `signal.on_interrupt(handler)` | Shortcut: register SIGINT handler |
| `signal.on_terminate(handler)` | Shortcut: register SIGTERM handler |
| `signal.reset(signum)` | Reset signal to default behavior |
| `signal.ignore(signum)` | Ignore a signal |
| `signal.send(signum)` | Send a signal to the current process |
| `signal.name(signum) -> str` | Get human-readable name for a signal |

## Examples

### Catch Ctrl+C

```desi
import signal

def main() -> int:
    signal.on_interrupt(lambda:
        print("Caught Ctrl+C! Shutting down...")
        0
    )
    
    # Your long-running program here
    print("Press Ctrl+C to stop")
    while true:
        pass
    0
```

### Graceful Shutdown

```desi
import signal

let running = true

def main() -> int:
    signal.on_terminate(lambda:
        print("SIGTERM received, cleaning up...")
        running = false
        0
    )
    
    while running:
        # Do work
        pass
    
    print("Shutdown complete")
    0
```

### Signal Names

```desi
import signal

def main() -> int:
    print(signal.name(signal.SIGINT))   # SIGINT
    print(signal.name(signal.SIGTERM))  # SIGTERM
    print(signal.name(signal.SIGHUP))   # SIGHUP
    0
```

### Ignore SIGPIPE

```desi
import signal

def main() -> int:
    signal.ignore(signal.SIGPIPE)
    # Write to closed pipes won't crash the program
    0
```

## Comparison

| Desi | Python | Go | Rust |
|---|---|---|---|
| `signal.on(sig, fn)` | `signal.signal(sig, fn)` | `signal.Notify(ch, sig)` | `ctrlc::set_handler(fn)` |
| `signal.on_interrupt(fn)` | `signal.signal(SIGINT, fn)` | Manual | `ctrlc::set_handler(fn)` |
| `signal.ignore(sig)` | `signal.signal(sig, SIG_IGN)` | `signal.Ignore(sig)` | Manual |
| `signal.name(sig)` | `signal.Signals(sig).name` | N/A | N/A |
