# Log Module

The `log` module provides structured logging with level prefixes for different severity levels.

## Import

```desi
import log
```

## Functions

| Function | Prefix | Use For |
|----------|--------|---------|
| `log.debug(msg)` | `[DEBUG]` | Verbose development details |
| `log.info(msg)` | `[INFO]` | Normal application events |
| `log.warn(msg)` | `[WARN]` | Potential problems |
| `log.error(msg)` | `[ERROR]` | Recoverable errors |
| `log.fatal(msg)` | `[FATAL]` | Unrecoverable errors |

## Usage

```desi
import log

def main() -> int:
    log.info("Application started")
    log.debug("Loading configuration...")
    log.warn("Disk space is low")
    log.error("Connection failed")
    log.fatal("Out of memory")
    0
```

### Output

```
[INFO] Application started
[DEBUG] Loading configuration...
[WARN] Disk space is low
[ERROR] Connection failed
[FATAL] Out of memory
```

## Typical Application Pattern

```desi
import log

def process(data: str):
    log.debug(f"Processing: {data}")
    if len(data) == 0:
        log.warn("Empty input received")
        return
    log.info(f"Processed {len(data)} characters")

def main() -> int:
    log.info("=== App Start ===")
    process("hello")
    process("")
    log.info("=== App End ===")
    0
```

## See Also

- [sys Module](sys.md) — Standard streams (`stdout`, `stderr`)
- [Math Module](math.md) — Mathematical functions
