# Time Module Implementation

This document covers the internal implementation details of the `time` module for contributors.

## Architecture

```
compiler/lib/time/__mod.desi  ← Desi public API
         ↓ (extern "C" calls)
compiler/runtime/time.c       ← Cross-platform C runtime
         ↓ (links with)
libc (time.h, sys/time.h)     ← System libraries
```

## Why This Design?

1. **Cross-platform**: C runtime handles platform differences (POSIX vs Windows)
2. **Performance**: Direct C calls avoid interpreter overhead
3. **Reliability**: Leverages battle-tested libc implementations

## C Runtime Functions

| C Function | Purpose |
|------------|---------|
| `__time_sleep_secs` | Sleep using `nanosleep`/`Sleep` |
| `__time_now` | Unix timestamp via `gettimeofday`/`FILETIME` |
| `__time_utc_now` | UTC timestamp (same as now for Unix time) |
| `__time_monotonic` | `CLOCK_MONOTONIC` for elapsed measurement |
| `__time_perf_counter` | `CLOCK_MONOTONIC_RAW`/`QueryPerformanceCounter` |
| `__time_timezone_offset` | Offset from UTC in seconds |
| `__time_timezone_name` | System timezone name |
| `__time_is_dst` | DST check via `tm_isdst` |
| `__time_to_utc/to_local` | Timezone conversions |
| `__desi_time_strftime` | Format time (prefixed to avoid collision) |
| `__time_format_timestamp` | Format specific timestamp |
| `__time_parse` | Parse via `strptime` |
| `__time_from_parts` | Build timestamp from components |
| `__desi_time_ctime` | Human-readable string (prefixed) |
| `__time_year/month/day/...` | Current time component getters |
| `__time_year_of/month_of/...` | Timestamp component extractors |
| `__time_is_leap_year` | Leap year calculation |
| `__time_add_*` | Arithmetic functions |
| `__time_diff` | Difference between timestamps |
| `__time_humanize` | "1 hour, 5 minutes" format |
| `__time_relative` | "3 hours ago" format |

## Naming Collision Issue

Some C stdlib functions (`strftime`, `ctime`) have the same names as Desi functions. With the new symbol mangling:

- All non-extern Desi functions are prefixed with `__desi$` at compile time
- For example, `strftime` → `__desi$strftime` in the generated LLVM IR
- This prevents collision with C's `strftime` at link time
- Users can use natural function names without workarounds

**Note**: Builtins (`print`, `len`, etc.) and `@extern` functions are NOT mangled.

## Platform Differences

| Feature | POSIX | Windows |
|---------|-------|---------|
| Sleep | `nanosleep()` | `Sleep()` |
| Timestamp | `gettimeofday()` | `GetSystemTimeAsFileTime()` |
| Monotonic | `clock_gettime(CLOCK_MONOTONIC)` | `GetTickCount64()` |
| Perf counter | `clock_gettime(CLOCK_MONOTONIC_RAW)` | `QueryPerformanceCounter()` |
| Timezone offset | `tm_gmtoff` | `GetTimeZoneInformation()` |
| Timezone name | `tm_zone` | `TIME_ZONE_INFORMATION` |

## Adding New Functions

1. Add C implementation in `compiler/runtime/time.c`
2. Add `@extern("C")` binding in `compiler/lib/time/__mod.desi`
3. Add `pub def` wrapper function with documentation
4. Rebuild with `make clean && make`
5. Add tests to `examples/400_time_sleep.desi`

## Testing

```bash
./build-desi.sh examples/400_time_sleep.desi test_time
./build/output/test_time
```

All functions are tested including edge cases for leap years, timezone round-trips, and sleep precision.
