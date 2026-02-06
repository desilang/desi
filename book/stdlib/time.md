# Time Module

The `time` module provides functions for working with dates, times, and durations.

## Quick Start

```python
from time import *

# Get current time
let ts = now()
print("Current timestamp:", ts)

# Format as string
print(format_time("%Y-%m-%d %H:%M:%S"))

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

### `format_time(format: str) -> str`
Format current time using strftime patterns.

```python
format_time("%Y-%m-%d")      # "2026-02-05"
format_time("%H:%M:%S")      # "13:30:45"
format_time("%A, %B %d")     # "Thursday, February 05"
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

### `readable_time(ts: float) -> str`
Human-readable time string.

```python
print(readable_time(now()))  # "Thu Feb  5 13:30:45 2026"
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

## Complete Example

```python
from time import *

def main():
    # Display current time info
    print("Today is", format_time("%A, %B %d, %Y"))
    print("Time:", format_time("%H:%M:%S"))
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
