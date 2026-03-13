# datetime — Date and Time Operations

High-level date operations with ISO 8601 support. For timestamps and sleep, use the `time` module.

## Import

```desi
import datetime
```

## API Reference

### Parsing & Formatting

| Function | Description |
|---|---|
| `datetime.parse_date("YYYY-MM-DD") -> float` | Parse ISO date to timestamp |
| `datetime.parse("YYYY-MM-DDTHH:MM:SS") -> float` | Parse ISO datetime |
| `datetime.to_date_str(ts) -> str` | Format timestamp to `YYYY-MM-DD` |
| `datetime.to_str(ts) -> str` | Format to `YYYY-MM-DDTHH:MM:SS` |
| `datetime.today() -> str` | Today's date |
| `datetime.now() -> str` | Current datetime |

### Arithmetic

| Function | Description |
|---|---|
| `datetime.add_days(date, n) -> str` | Add days |
| `datetime.add_months(date, n) -> str` | Add months (handles lengths) |
| `datetime.add_years(date, n) -> str` | Add years |
| `datetime.diff_days(d1, d2) -> int` | Days between dates |

### Calendar

| Function | Description |
|---|---|
| `datetime.days_in_month(year, month) -> int` | Days in month (leap-aware) |
| `datetime.is_weekend(date) -> bool` | Saturday or Sunday? |
| `datetime.weekday_name(date) -> str` | "Monday", "Tuesday", etc. |
| `datetime.month_name(month) -> str` | "January", "February", etc. |
| `datetime.week_number(date) -> int` | ISO week number (1-53) |
| `datetime.is_valid(date) -> bool` | Validate date string |
| `datetime.compare(d1, d2) -> int` | -1, 0, or 1 |

## Examples

```desi
import datetime

let today = datetime.today()          # "2024-06-15"
let next = datetime.add_months(today, 3)  # "2024-09-15"
let diff = datetime.diff_days(next, today)  # 92

# Leap year handling
let leap = datetime.add_years("2024-02-29", 1)  # "2025-02-28"
let dim = datetime.days_in_month(2024, 2)        # 29

# Calendar info
print(datetime.weekday_name("2024-12-25"))  # Wednesday
print(datetime.month_name(12))              # December
```
