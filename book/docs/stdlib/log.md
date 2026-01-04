# Log Module

The `log` module provides structured logging with colored output for different severity levels.

## Import

```desi
import log
```

Or import specific functions:
```desi
from log import info, warn, error, debug
```

## Functions

| Function | Description |
|----------|-------------|
| `log.info(message: str)` | Log info message with blue `[INFO]` prefix |
| `log.debug(message: str)` | Log debug message with cyan `[DEBUG]` prefix |
| `log.warn(message: str)` | Log warning message with yellow `[WARN]` prefix |
| `log.error(message: str)` | Log error message with red `[ERROR]` prefix |

## Usage

```desi
import log

def main() -> int:
    log.info("Application started")
    log.debug("Loading configuration...")
    log.warn("Disk space is low")
    log.error("Connection failed")
    0
```

### Output

```
[INFO] Application started
[DEBUG] Loading configuration...
[WARN] Disk space is low
[ERROR] Connection failed
```

!!! note "ANSI Colors"
    Log output uses ANSI escape codes for colored terminal output. Colors may not display correctly in all terminals.

## See Also

- [JSON Module](json.md) - JSON parsing and serialization
- [Math Module](math.md) - Mathematical functions
