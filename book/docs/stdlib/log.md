# Log Module

The `log` module provides structured logging with ANSI colored output and level filtering.

## Import

```desi
import log
```

## Log Levels

Levels from lowest to highest priority:

| Level | Function | Color | Output |
|-------|----------|-------|--------|
| DEBUG | `log.debug(msg)` | Cyan | stdout |
| INFO | `log.info(msg)` | Blue | stdout |
| WARN | `log.warn(msg)` | Yellow | stderr |
| ERROR | `log.error(msg)` | Red | stderr |
| FATAL | `log.fatal(msg)` | Bold Red | stderr |

## Level Filtering

Use `log.set_level(level)` to filter messages. Only messages at or above the set level are printed.

```desi
import log

def main() -> int:
    # Default: all levels visible
    log.debug("verbose detail")
    log.info("starting up")

    # Set to WARN — only WARN, ERROR, FATAL will show
    log.set_level("WARN")
    log.info("hidden")      # Filtered out
    log.warn("visible")     # Shows

    # Reset to show everything
    log.set_level("DEBUG")
    log.debug("visible again")
    0
```

Valid level names: `"DEBUG"`, `"INFO"`, `"WARN"`, `"ERROR"`, `"FATAL"`

## Usage Examples

### Basic Logging

```desi
import log

def main() -> int:
    log.info("Server started on port 8080")
    log.warn("Disk space below 10%")
    log.error("Connection to database refused")
    0
```

### Production Configuration

```desi
import log

def main() -> int:
    log.set_level("INFO")    # Hide debug in production
    log.debug("hidden")
    log.info("App ready")
    log.error("Something broke")
    0
```

## See Also

- [OS Module](os.md) — System functions
- [sys Module](sys.md) — Standard streams (`stdout`, `stderr`)
