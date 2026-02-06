// Time module C runtime
// Provides cross-platform time functions (Python-inspired + Desi innovations)

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <math.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#include <time.h>
#include <sys/time.h>
#endif

// ============================================================
// Sleep Functions
// ============================================================

void __time_sleep_secs(double seconds) {
    if (seconds <= 0.0) return;
    
    long long milliseconds = (long long)(seconds * 1000.0);
    
#ifdef _WIN32
    Sleep((DWORD)milliseconds);
#else
    struct timespec ts;
    ts.tv_sec = milliseconds / 1000;
    ts.tv_nsec = (milliseconds % 1000) * 1000000;
    while (nanosleep(&ts, &ts) == -1) {}
#endif
}

// ============================================================
// Time Retrieval Functions
// ============================================================

// Local time as Unix timestamp
double __time_now(void) {
#ifdef _WIN32
    FILETIME ft;
    GetSystemTimeAsFileTime(&ft);
    uint64_t t = ((uint64_t)ft.dwHighDateTime << 32) | ft.dwLowDateTime;
    return (double)(t - 116444736000000000ULL) / 10000000.0;
#else
    struct timeval tv;
    gettimeofday(&tv, NULL);
    return (double)tv.tv_sec + (double)tv.tv_usec / 1000000.0;
#endif
}

// UTC time as Unix timestamp (same as now() since Unix timestamps are UTC-based)
double __time_utc_now(void) {
    return __time_now();
}

// Monotonic time (for measuring elapsed time)
double __time_monotonic(void) {
#ifdef _WIN32
    return (double)GetTickCount64() / 1000.0;
#else
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (double)ts.tv_sec + (double)ts.tv_nsec / 1000000000.0;
#endif
}

// High-precision performance counter
double __time_perf_counter(void) {
#ifdef _WIN32
    LARGE_INTEGER freq, counter;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&counter);
    return (double)counter.QuadPart / (double)freq.QuadPart;
#else
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC_RAW, &ts);
    return (double)ts.tv_sec + (double)ts.tv_nsec / 1000000000.0;
#endif
}

// ============================================================
// Timezone Functions
// ============================================================

// Get timezone offset in seconds from UTC (positive = east of UTC)
int __time_timezone_offset(void) {
#ifdef _WIN32
    TIME_ZONE_INFORMATION tzi;
    GetTimeZoneInformation(&tzi);
    return -(tzi.Bias * 60);  // Bias is in minutes, west is positive
#else
    time_t now = time(NULL);
    struct tm* local = localtime(&now);
    return (int)local->tm_gmtoff;
#endif
}

// Get timezone name (e.g., "CST", "EST")
char* __time_timezone_name(void) {
#ifdef _WIN32
    TIME_ZONE_INFORMATION tzi;
    DWORD result = GetTimeZoneInformation(&tzi);
    char buffer[64];
    if (result == TIME_ZONE_ID_DAYLIGHT) {
        wcstombs(buffer, tzi.DaylightName, sizeof(buffer));
    } else {
        wcstombs(buffer, tzi.StandardName, sizeof(buffer));
    }
    return strdup(buffer);
#else
    time_t now = time(NULL);
    struct tm* local = localtime(&now);
    return strdup(local->tm_zone ? local->tm_zone : "UTC");
#endif
}

// Check if currently in daylight saving time
bool __time_is_dst(void) {
    time_t now = time(NULL);
    struct tm* local = localtime(&now);
    return local ? local->tm_isdst > 0 : false;
}

// Convert local timestamp to UTC
double __time_to_utc(double local_ts) {
    return local_ts - (double)__time_timezone_offset();
}

// Convert UTC timestamp to local
double __time_to_local(double utc_ts) {
    return utc_ts + (double)__time_timezone_offset();
}

// ============================================================
// Date/Time Formatting & Parsing
// ============================================================

char* __desi_time_strftime(const char* format) {
    if (!format) return strdup("");
    
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    if (!tm_info) return strdup("");
    
    char buffer[256];
    size_t result = strftime(buffer, sizeof(buffer), format, tm_info);
    
    if (result == 0) return strdup("");
    return strdup(buffer);
}

// Format a specific timestamp (not current time)
char* __time_format_timestamp(double ts, const char* format) {
    if (!format) return strdup("");
    
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    if (!tm_info) return strdup("");
    
    char buffer[256];
    size_t result = strftime(buffer, sizeof(buffer), format, tm_info);
    
    if (result == 0) return strdup("");
    return strdup(buffer);
}

// Parse time string to timestamp
double __time_parse(const char* time_str, const char* format) {
    if (!time_str || !format) return 0.0;
    
    struct tm tm_info = {0};
    char* result = strptime(time_str, format, &tm_info);
    
    if (!result) return 0.0;  // Parse failed
    
    tm_info.tm_isdst = -1;  // Let system determine DST
    time_t t = mktime(&tm_info);
    return (t == -1) ? 0.0 : (double)t;
}

// Create timestamp from components
double __time_from_parts(int year, int month, int day, int hour, int minute, int second) {
    struct tm tm_info = {0};
    tm_info.tm_year = year - 1900;
    tm_info.tm_mon = month - 1;
    tm_info.tm_mday = day;
    tm_info.tm_hour = hour;
    tm_info.tm_min = minute;
    tm_info.tm_sec = second;
    tm_info.tm_isdst = -1;
    
    time_t t = mktime(&tm_info);
    return (t == -1) ? 0.0 : (double)t;
}

// Human-readable time string (like Python's ctime)
char* __desi_time_ctime(double ts) {
    time_t t = (time_t)ts;
    char* result = ctime(&t);
    if (!result) return strdup("");
    
    // Remove trailing newline
    char* copy = strdup(result);
    size_t len = strlen(copy);
    if (len > 0 && copy[len-1] == '\n') copy[len-1] = '\0';
    return copy;
}

// ============================================================
// Date Component Getters
// ============================================================

int __time_year(void) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    return tm_info ? tm_info->tm_year + 1900 : 0;
}

int __time_month(void) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    return tm_info ? tm_info->tm_mon + 1 : 0;
}

int __time_day(void) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    return tm_info ? tm_info->tm_mday : 0;
}

int __time_hour(void) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    return tm_info ? tm_info->tm_hour : 0;
}

int __time_minute(void) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    return tm_info ? tm_info->tm_min : 0;
}

int __time_second(void) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    return tm_info ? tm_info->tm_sec : 0;
}

int __time_weekday(void) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    return tm_info ? tm_info->tm_wday : 0;
}

// Get components from a specific timestamp
int __time_year_of(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_year + 1900 : 0;
}

int __time_month_of(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_mon + 1 : 0;
}

int __time_day_of(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_mday : 0;
}

int __time_hour_of(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_hour : 0;
}

int __time_minute_of(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_min : 0;
}

int __time_second_of(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_sec : 0;
}

int __time_weekday_of(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_wday : 0;
}

// Day of year (1-366)
int __time_day_of_year(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm_info = localtime(&t);
    return tm_info ? tm_info->tm_yday + 1 : 0;
}

// Check if year is leap year
bool __time_is_leap_year(int year) {
    return (year % 4 == 0 && year % 100 != 0) || (year % 400 == 0);
}

// ============================================================
// Arithmetic Functions
// ============================================================

double __time_add_seconds(double ts, double seconds) {
    return ts + seconds;
}

double __time_add_days(double ts, int days) {
    return ts + (double)(days * 86400);
}

double __time_add_hours(double ts, int hours) {
    return ts + (double)(hours * 3600);
}

double __time_add_minutes(double ts, int minutes) {
    return ts + (double)(minutes * 60);
}

double __time_diff(double ts1, double ts2) {
    return ts1 - ts2;
}

// ============================================================
// Innovative Functions (Desi-specific)
// ============================================================

// Humanize a duration in seconds to readable string
// e.g., 3665 -> "1 hour, 1 minute, 5 seconds"
char* __time_humanize(double seconds) {
    char buffer[256];
    buffer[0] = '\0';
    
    int neg = seconds < 0;
    if (neg) seconds = -seconds;
    
    long long total_secs = (long long)seconds;
    
    int days = total_secs / 86400;
    int hours = (total_secs % 86400) / 3600;
    int mins = (total_secs % 3600) / 60;
    int secs = total_secs % 60;
    
    char* p = buffer;
    if (neg) p += sprintf(p, "-");
    
    int first = 1;
    if (days > 0) {
        p += sprintf(p, "%d day%s", days, days == 1 ? "" : "s");
        first = 0;
    }
    if (hours > 0) {
        p += sprintf(p, "%s%d hour%s", first ? "" : ", ", hours, hours == 1 ? "" : "s");
        first = 0;
    }
    if (mins > 0) {
        p += sprintf(p, "%s%d minute%s", first ? "" : ", ", mins, mins == 1 ? "" : "s");
        first = 0;
    }
    if (secs > 0 || first) {
        p += sprintf(p, "%s%d second%s", first ? "" : ", ", secs, secs == 1 ? "" : "s");
    }
    
    return strdup(buffer);
}

// Relative time (e.g., "3 hours ago", "in 2 days")
char* __time_relative(double ts) {
    double now = __time_now();
    double diff = ts - now;
    
    char buffer[128];
    int neg = diff < 0;
    if (neg) diff = -diff;
    
    long long total_secs = (long long)diff;
    
    const char* unit;
    int value;
    
    if (total_secs < 60) {
        value = (int)total_secs;
        unit = value == 1 ? "second" : "seconds";
    } else if (total_secs < 3600) {
        value = total_secs / 60;
        unit = value == 1 ? "minute" : "minutes";
    } else if (total_secs < 86400) {
        value = total_secs / 3600;
        unit = value == 1 ? "hour" : "hours";
    } else if (total_secs < 2592000) {  // ~30 days
        value = total_secs / 86400;
        unit = value == 1 ? "day" : "days";
    } else if (total_secs < 31536000) {  // ~365 days
        value = total_secs / 2592000;
        unit = value == 1 ? "month" : "months";
    } else {
        value = total_secs / 31536000;
        unit = value == 1 ? "year" : "years";
    }
    
    if (neg) {
        sprintf(buffer, "%d %s ago", value, unit);
    } else {
        sprintf(buffer, "in %d %s", value, unit);
    }
    
    return strdup(buffer);
}

