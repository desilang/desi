# Time Module

The `time` module provides functions for working with dates, times, and durations.

## Quick Start

```python
from time import *

# Get current time
let ts = now()
print("Current timestamp:", ts)

# Format as string
print(strftime("%Y-%m-%d %H:%M:%S"))

# Sleep for 1 second
sleep(1.0)

# Human-readable duration
print(humanize(3665.0))  # "1 hour, 1 minute, 5 seconds"
```

---

## Core Functions

### `sleep(seconds: float)`
Pause execution for the specified duration.

```python
sleep(0.5)   # Sleep for 500ms
sleep(2.0)   # Sleep for 2 seconds
```

### `now() -> float`
Get current local Unix timestamp (seconds since Jan 1, 1970).

```python
let timestamp = now()
```

### `utc_now() -> float`
Get current UTC timestamp.

### `monotonic() -> float`
Get monotonic time (unaffected by clock changes). Use for measuring elapsed time.

```python
let start = monotonic()
# ... do work ...
let elapsed = monotonic() - start
print("Took", elapsed, "seconds")
```

### `perf_counter() -> float`
High-precision timer for benchmarking.

---

## Timezone Functions

### `timezone_offset() -> int`
Get offset from UTC in seconds.

```python
let offset = timezone_offset()  # -21600 for CST (UTC-6)
```

### `timezone_name() -> str`
Get timezone name.

```python
print(timezone_name())  # "CST", "EST", etc.
```

### `is_dst() -> bool`
Check if daylight saving time is active.

### `to_utc(local_ts: float) -> float`
Convert local timestamp to UTC.

### `to_local(utc_ts: float) -> float`
Convert UTC timestamp to local.

---

## Formatting & Parsing

### `strftime(format: str) -> str`
Format current time using strftime patterns.

```python
strftime("%Y-%m-%d")      # "2026-02-05"
strftime("%H:%M:%S")      # "13:30:45"
strftime("%A, %B %d")     # "Thursday, February 05"
```

**Common format codes:**
| Code | Meaning | Example |
|------|---------|---------|
| `%Y` | Year (4 digit) | 2026 |
| `%m` | Month (01-12) | 02 |
| `%d` | Day (01-31) | 05 |
| `%H` | Hour (00-23) | 13 |
| `%M` | Minute (00-59) | 30 |
| `%S` | Second (00-59) | 45 |
| `%A` | Weekday name | Thursday |
| `%B` | Month name | February |

### `format_timestamp(ts: float, format: str) -> str`
Format a specific timestamp.

```python
let ts = from_parts(2026, 12, 25, 0, 0, 0)
print(format_timestamp(ts, "%B %d, %Y"))  # "December 25, 2026"
```

### `parse_time(time_str: str, format: str) -> float`
Parse a time string into a timestamp.

```python
let ts = parse_time("2026-02-05", "%Y-%m-%d")
```

### `from_parts(year, month, day, hour, minute, second) -> float`
Create a timestamp from components.

```python
let christmas = from_parts(2026, 12, 25, 0, 0, 0)
```

### `ctime(ts: float) -> str`
Human-readable time string.

```python
print(ctime(now()))  # "Thu Feb  5 13:30:45 2026"
```

---

## Time Components

### Current Time Components

```python
year()      # 2026
month()     # 2 (1-12)
day()       # 5 (1-31)
hour()      # 13 (0-23)
minute()    # 30 (0-59)
second()    # 45 (0-59)
weekday()   # 4 (0=Sunday, 6=Saturday)
```

### Extract from Timestamp

```python
let ts = from_parts(2026, 7, 4, 12, 0, 0)

year_of(ts)       # 2026
month_of(ts)      # 7
day_of(ts)        # 4
hour_of(ts)       # 12
minute_of(ts)     # 0
second_of(ts)     # 0
weekday_of(ts)    # 6 (Saturday)
day_of_year(ts)   # 185
```

### `is_leap_year(year: int) -> bool`

```python
is_leap_year(2024)  # true
is_leap_year(2025)  # false
is_leap_year(2000)  # true (divisible by 400)
is_leap_year(1900)  # false (divisible by 100 but not 400)
```

---

## Arithmetic

### `add_seconds(ts: float, seconds: float) -> float`

```python
let later = add_seconds(now(), 30.0)
```

### `add_minutes(ts: float, minutes: int) -> float`

```python
let later = add_minutes(now(), 15)
```

### `add_hours(ts: float, hours: int) -> float`

```python
let later = add_hours(now(), 3)
```

### `add_days(ts: float, days: int) -> float`

```python
let tomorrow = add_days(now(), 1)
let yesterday = add_days(now(), -1)
```

### `diff(ts1: float, ts2: float) -> float`
Get difference between timestamps in seconds.

```python
let d = diff(end_time, start_time)
print("Elapsed:", d, "seconds")
```

---

## Innovative Functions 🚀

These are unique to Desi!

### `humanize(seconds: float) -> str`
Convert duration to human-readable string.

```python
humanize(0.0)      # "0 seconds"
humanize(1.0)      # "1 second"
humanize(60.0)     # "1 minute"
humanize(3665.0)   # "1 hour, 1 minute, 5 seconds"
humanize(86400.0)  # "1 day"
humanize(90061.0)  # "1 day, 1 hour, 1 minute, 1 second"
```

### `relative(ts: float) -> str`
Get relative time description.

```python
let past = add_hours(now(), -2)
print(relative(past))   # "2 hours ago"

let future = add_days(now(), 3)
print(relative(future)) # "in 3 days"
```

---

## Duration Class

A class for representing and working with time durations.

### Creating Durations

```python
from time import Duration

# Factory methods
let d1 = Duration.from_seconds(90.0)
let d2 = Duration.from_minutes(1.5)   # 90 seconds
let d3 = Duration.from_hours(2.0)     # 7200 seconds
let d4 = Duration.from_days(1.0)      # 86400 seconds
let d5 = Duration.from_millis(500.0)  # 0.5 seconds

# Or use the helper function
let d6 = create_duration(3600.0)
```

### Duration Methods

```python
let d = Duration.from_hours(2.5)

d.total_seconds()  # 9000.0
d.total_minutes()  # 150.0
d.total_hours()    # 2.5
d.total_days()     # 0.104...

# String representation (calls humanize)
print(d)  # "2 hours, 30 minutes"
```

---

## Stopwatch Class

Simple timing utility for measuring code execution.

```python
from time import Stopwatch

let sw = Stopwatch()
sw.start()

# ... do some work ...
sleep(1.0)

print(sw.elapsed())      # 1.0 (seconds)
print(sw.elapsed_str())  # "1 second"
```

### Stopwatch Methods

| Method | Return | Description |
|--------|--------|-------------|
| `start()` | none | Start or restart timing |
| `elapsed()` | float | Get elapsed seconds |
| `elapsed_str()` | str | Get human-readable elapsed time |

---

## Complete Example

```python
from time import *

def main():
    # Display current time info
    print("Today is", strftime("%A, %B %d, %Y"))
    print("Time:", strftime("%H:%M:%S"))
    print("Timezone:", timezone_name())
    
    # Calculate future date
    let deadline = add_days(now(), 7)
    print("Deadline:", format_timestamp(deadline, "%Y-%m-%d"))
    print("That's", relative(deadline))
    
    # Measure operation time
    let start = monotonic()
    sleep(0.5)
    let elapsed = monotonic() - start
    print("Operation took", humanize(elapsed))
    
    # Check leap year
    print("2024 is leap year:", is_leap_year(2024))

main()
```
