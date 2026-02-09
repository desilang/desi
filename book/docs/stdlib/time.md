# Time Module

The `time` module provides functions and classes for working with dates, times, durations, and timing.

## Import

```desi
from time import *
```

## Core Functions

### Current Time

```desi
now() -> float           # Current time as Unix timestamp
utc_now() -> float       # Current UTC time as Unix timestamp
monotonic() -> float     # Monotonic clock (for measuring intervals)
perf_counter() -> float  # High-resolution performance counter
```

### Sleep

```desi
sleep(seconds: float)    # Pause execution
```

### Formatting

```desi
humanize(seconds: float) -> str        # "2h 5m 30s"
relative(timestamp: float) -> str      # "3 hours ago"
strftime(format: str, ts: float) -> str  # Custom format
format_timestamp(ts: float) -> str     # ISO 8601 format
ctime(ts: float) -> str                # Human-readable date
```

### Parsing

```desi
parse_time(s: str) -> float            # Parse ISO 8601 string
from_parts(y, mo, d, h, mi, s) -> float  # Build timestamp
```

### Components

```desi
year(), month(), day(), hour(), minute(), second(), weekday()  # Current
year_of(ts), month_of(ts), day_of(ts), ...  # From timestamp
```

### Arithmetic

```desi
add_seconds(ts, n) -> float
add_minutes(ts, n) -> float
add_hours(ts, n) -> float
add_days(ts, n) -> float
diff(ts1, ts2) -> float    # Difference in seconds
```

### Timezone

```desi
timezone_offset() -> int     # Offset from UTC in seconds
timezone_name() -> str       # e.g., "PST", "UTC"
is_dst(ts: float) -> bool    # Daylight saving time check
to_utc(ts: float) -> float   # Convert to UTC
to_local(ts: float) -> float # Convert to local time
```

## Duration Class

Represents a time span for arithmetic and formatting.

```desi
# Create from various units
let d1 = Duration.from_seconds(90.0)   # 90 seconds
let d2 = Duration.from_minutes(5.0)    # 5 minutes
let d3 = Duration.from_hours(2.5)      # 2.5 hours
let d4 = Duration.from_days(1.0)       # 1 day
let d5 = Duration.from_millis(500.0)   # 500 milliseconds

# Factory function
let d6 = create_duration(120.5)        # 120.5 seconds

# Get components
d1.total_seconds()  # Total as seconds
d1.humanize()       # e.g., "1m 30s"
```

## Stopwatch Class

Simple timing utility for measuring code execution.

### Basic Usage

```desi
let sw = Stopwatch()
sw.start()
# ... code to time ...
print("Took:", sw.elapsed_str())  # e.g., "245.3ms"
```

### Factory Functions

```desi
# Auto-starts immediately
let sw1 = stopwatch()

# Silent mode - doesn't print when scope exits
let sw2 = stopwatch_silent()
```

### Methods

```desi
sw.start()          # Start or restart timing
sw.elapsed() -> float       # Get elapsed seconds
sw.elapsed_str() -> str     # Human-readable elapsed time
```

## Example

```desi
from time import *

def main():
    # Time a code block
    let sw = Stopwatch()
    sw.start()
    
    # Do some work
    for i in range(1000000):
        pass
    
    print("Loop took:", sw.elapsed_str())
    
    # Format a duration
    let d = Duration.from_seconds(3661.0)
    print("Duration:", d.humanize())  # "1h 1m 1s"
    
    # Current time
    print("Now:", format_timestamp(now()))
    print("Relative:", relative(now() - 3600))  # "1 hour ago"
```

## See Also

- [Concurrency](../concurrency/index.md) - Async timing with TaskGroup
- [Sync Module](sync.md) - Thread-safe timing patterns
