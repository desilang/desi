# Time Module

The `time` module provides functions and classes for working with dates, times, durations, and timing.

## Import

```desi
from time import *
```

## Core Functions

### Current Time

| Function | Description |
|----------|-------------|
| `now() -> float` | Current time as a Unix timestamp |
| `utc_now() -> float` | Current UTC time as a Unix timestamp |
| `monotonic() -> float` | Monotonic clock, for measuring intervals |
| `perf_counter() -> float` | High-resolution performance counter |

### Sleep

| Function | Description |
|----------|-------------|
| `sleep(seconds: float) -> none` | Pause execution |

### Formatting

| Function | Description |
|----------|-------------|
| `humanize(seconds: float) -> str` | A span as text — `"2h 5m 30s"` |
| `relative(ts: float) -> str` | A timestamp relative to now — `"3 hours ago"` |
| `strftime(format: str) -> str` | Format the *current* time |
| `format_timestamp(ts: float, format: str) -> str` | Format a given timestamp |
| `ctime(ts: float) -> str` | Human-readable date |

!!! note "`strftime` formats the current time"
    It takes no timestamp. To format a timestamp you already have, use
    `format_timestamp(ts, format)`.

### Parsing

| Function | Description |
|----------|-------------|
| `parse_time(time_str: str, format: str) -> float` | Parse a string using an explicit format |
| `from_parts(year: int, month: int, day: int, hour: int, minute: int, second: int) -> float` | Build a timestamp from components |

### Components

Each of these reads the current time:

| Function | Description |
|----------|-------------|
| `year() -> int` | Current year |
| `month() -> int` | Current month, 1–12 |
| `day() -> int` | Current day of month |
| `hour() -> int` | Current hour, 0–23 |
| `minute() -> int` | Current minute |
| `second() -> int` | Current second |
| `weekday() -> int` | Current day of week |

Each has an `_of` counterpart taking a timestamp:

| Function | Description |
|----------|-------------|
| `year_of(ts: float) -> int` | Year of `ts` |
| `month_of(ts: float) -> int` | Month of `ts` |
| `day_of(ts: float) -> int` | Day of month of `ts` |
| `hour_of(ts: float) -> int` | Hour of `ts` |
| `minute_of(ts: float) -> int` | Minute of `ts` |
| `second_of(ts: float) -> int` | Second of `ts` |
| `weekday_of(ts: float) -> int` | Day of week of `ts` |
| `day_of_year(ts: float) -> int` | Day of year of `ts`, 1–366 |
| `is_leap_year(year: int) -> bool` | Whether `year` is a leap year |

### Arithmetic

| Function | Description |
|----------|-------------|
| `add_seconds(ts: float, seconds: float) -> float` | Add seconds |
| `add_minutes(ts: float, minutes: int) -> float` | Add minutes |
| `add_hours(ts: float, hours: int) -> float` | Add hours |
| `add_days(ts: float, days: int) -> float` | Add days |
| `diff(ts1: float, ts2: float) -> float` | Difference in seconds |

### Timezone

| Function | Description |
|----------|-------------|
| `timezone_offset() -> int` | Offset from UTC in seconds |
| `timezone_name() -> str` | e.g. `"PST"`, `"UTC"` |
| `is_dst() -> bool` | Whether daylight saving is in effect now |
| `to_utc(local_ts: float) -> float` | Convert a local timestamp to UTC |
| `to_local(utc_ts: float) -> float` | Convert a UTC timestamp to local |

## Duration Class

Represents a time span for arithmetic and formatting.

```desi
from time import *

def main() -> int:
	# Create from various units
	let d1 = Duration.from_seconds(90.0)   # 90 seconds
	let d2 = Duration.from_minutes(5.0)    # 5 minutes
	let d3 = Duration.from_hours(2.5)      # 2.5 hours
	let d4 = Duration.from_days(1.0)       # 1 day
	let d5 = Duration.from_millis(500.0)   # 500 milliseconds

	# Factory function
	let d6 = create_duration(120.5)        # 120.5 seconds

	print(str(d1.total_seconds()))         # 90.0
	print(humanize(d1.total_seconds()))    # "1 minute, 30 seconds"
	return 0
```

| Method | Description |
|--------|-------------|
| `total_seconds() -> float` | The span in seconds |
| `total_minutes() -> float` | The span in minutes |
| `total_hours() -> float` | The span in hours |
| `total_days() -> float` | The span in days |

!!! note "Formatting a Duration"
    `Duration` has no `humanize()` method of its own. Pass its seconds to the
    module-level function: `humanize(d.total_seconds())`.

## Stopwatch Class

Simple timing utility for measuring code execution.

### Basic Usage

```desi
from time import *

def main() -> int:
	let sw = Stopwatch()
	sw.start()
	# ... code to time ...
	print("Took:", sw.elapsed_str())  # e.g. "245.3ms"
	return 0
```

### Factory Functions

| Function | Description |
|----------|-------------|
| `stopwatch() -> Stopwatch` | Auto-starts immediately |
| `stopwatch_silent() -> Stopwatch` | Silent — does not print when the scope exits |

### Methods

| Method | Description |
|--------|-------------|
| `start()` | Start or restart timing |
| `elapsed() -> float` | Elapsed seconds |
| `elapsed_str() -> str` | Human-readable elapsed time |

## Example

```desi
from time import *

def main() -> int:
	# Time a code block
	let sw = Stopwatch()
	sw.start()

	# Do some work
	for i in range(1000000):
		pass

	print("Loop took:", sw.elapsed_str())

	# Format a duration
	let d = Duration.from_seconds(3661.0)
	print("Duration:", humanize(d.total_seconds()))  # "1 hour, 1 minute, 1 second"

	# Current time
	print("Now:", format_timestamp(now(), "%Y-%m-%d %H:%M:%S"))
	print("Relative:", relative(now() - 3600.0))     # "1 hour ago"
	return 0
```

## See Also

- [Concurrency](../concurrency/thread-safety.md) - Async timing with TaskGroup
- [Sync Module](sync.md) - Thread-safe timing patterns
