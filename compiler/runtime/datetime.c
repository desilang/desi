/*
 * datetime.c — High-level date/time operations for Desi stdlib
 *
 * Builds on top of time.c. Provides:
 *   - ISO 8601 parsing/formatting
 *   - Date-only operations (no time component)
 *   - Date diffing (days between dates)
 *   - Calendar utilities (days_in_month, is_weekend, week_number)
 *   - Date validation
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <stdbool.h>
#ifdef _WIN32
#define strcasecmp _stricmp
#endif

// ============================================================
// ISO 8601 Parsing/Formatting
// ============================================================

// Parse ISO 8601 date string "YYYY-MM-DD" to timestamp
double __datetime_parse_date(const char* date_str) {
    if (!date_str) return 0.0;
    int year, month, day;
    if (sscanf(date_str, "%d-%d-%d", &year, &month, &day) != 3) return 0.0;

    struct tm t = {0};
    t.tm_year = year - 1900;
    t.tm_mon = month - 1;
    t.tm_mday = day;
    t.tm_isdst = -1;
    time_t ts = mktime(&t);
    return (ts == -1) ? 0.0 : (double)ts;
}

// Parse ISO 8601 datetime "YYYY-MM-DDTHH:MM:SS" to timestamp
double __datetime_parse(const char* datetime_str) {
    if (!datetime_str) return 0.0;
    int year, month, day, hour = 0, min = 0, sec = 0;

    // Try full datetime first
    int n = sscanf(datetime_str, "%d-%d-%dT%d:%d:%d", &year, &month, &day, &hour, &min, &sec);
    if (n < 3) {
        // Try with space separator
        n = sscanf(datetime_str, "%d-%d-%d %d:%d:%d", &year, &month, &day, &hour, &min, &sec);
    }
    if (n < 3) return 0.0;

    struct tm t = {0};
    t.tm_year = year - 1900;
    t.tm_mon = month - 1;
    t.tm_mday = day;
    t.tm_hour = hour;
    t.tm_min = min;
    t.tm_sec = sec;
    t.tm_isdst = -1;
    time_t ts = mktime(&t);
    return (ts == -1) ? 0.0 : (double)ts;
}

// Format timestamp to ISO 8601 date "YYYY-MM-DD"
char* __datetime_to_date_str(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return strdup("");
    char buf[32];
    snprintf(buf, sizeof(buf), "%04d-%02d-%02d", tm->tm_year + 1900, tm->tm_mon + 1, tm->tm_mday);
    return strdup(buf);
}

// Format timestamp to ISO 8601 datetime "YYYY-MM-DDTHH:MM:SS"
char* __datetime_to_str(double ts) {
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return strdup("");
    char buf[32];
    snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02d",
             tm->tm_year + 1900, tm->tm_mon + 1, tm->tm_mday,
             tm->tm_hour, tm->tm_min, tm->tm_sec);
    return strdup(buf);
}

// Get today's date as "YYYY-MM-DD"
char* __datetime_today(void) {
    time_t now = time(NULL);
    return __datetime_to_date_str((double)now);
}

// Get current datetime as ISO 8601
char* __datetime_now(void) {
    time_t now = time(NULL);
    return __datetime_to_str((double)now);
}

// ============================================================
// Date Arithmetic
// ============================================================

// Add days to a date string, return new date string
char* __datetime_add_days(const char* date_str, int days) {
    double ts = __datetime_parse_date(date_str);
    ts += (double)(days * 86400);
    return __datetime_to_date_str(ts);
}

// Add months to a date string (handles month lengths correctly)
char* __datetime_add_months(const char* date_str, int months) {
    if (!date_str) return strdup("");
    int year, month, day;
    if (sscanf(date_str, "%d-%d-%d", &year, &month, &day) != 3) return strdup("");

    // Add months
    month += months;
    while (month > 12) { year++; month -= 12; }
    while (month < 1) { year--; month += 12; }

    // Clamp day to max days in target month
    int max_day;
    int days_in_months[] = {0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};
    if (month == 2 && ((year % 4 == 0 && year % 100 != 0) || (year % 400 == 0))) {
        max_day = 29;
    } else {
        max_day = days_in_months[month];
    }
    if (day > max_day) day = max_day;

    char buf[32];
    snprintf(buf, sizeof(buf), "%04d-%02d-%02d", year, month, day);
    return strdup(buf);
}

// Add years
char* __datetime_add_years(const char* date_str, int years) {
    return __datetime_add_months(date_str, years * 12);
}

// Difference in days between two date strings
int __datetime_diff_days(const char* date1, const char* date2) {
    double ts1 = __datetime_parse_date(date1);
    double ts2 = __datetime_parse_date(date2);
    return (int)((ts1 - ts2) / 86400.0);
}

// ============================================================
// Calendar Utilities
// ============================================================

// Days in a given month (1-12) of a given year
int __datetime_days_in_month(int year, int month) {
    if (month < 1 || month > 12) return 0;
    int days[] = {0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};
    if (month == 2 && ((year % 4 == 0 && year % 100 != 0) || (year % 400 == 0))) {
        return 29;
    }
    return days[month];
}

// Check if a date string falls on a weekend (Sat=6 or Sun=0)
int __datetime_is_weekend(const char* date_str) {
    double ts = __datetime_parse_date(date_str);
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return 0;
    return (tm->tm_wday == 0 || tm->tm_wday == 6) ? 1 : 0;
}

// Get weekday name from date string
char* __datetime_weekday_name(const char* date_str) {
    double ts = __datetime_parse_date(date_str);
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return strdup("");
    const char* names[] = {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"};
    return strdup(names[tm->tm_wday]);
}

// Get month name from month number (1-12)
char* __datetime_month_name(int month) {
    if (month < 1 || month > 12) return strdup("");
    const char* names[] = {"", "January", "February", "March", "April", "May", "June",
                           "July", "August", "September", "October", "November", "December"};
    return strdup(names[month]);
}

// ISO week number (1-53) from date string
int __datetime_week_number(const char* date_str) {
    double ts = __datetime_parse_date(date_str);
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return 0;
    char buf[8];
    strftime(buf, sizeof(buf), "%V", tm);
    return atoi(buf);
}

// Validate a date string
int __datetime_is_valid(const char* date_str) {
    if (!date_str) return 0;
    int year, month, day;
    if (sscanf(date_str, "%d-%d-%d", &year, &month, &day) != 3) return 0;
    if (month < 1 || month > 12) return 0;
    if (day < 1 || day > __datetime_days_in_month(year, month)) return 0;
    return 1;
}

// Compare two date strings: returns -1, 0, or 1
int __datetime_compare(const char* date1, const char* date2) {
    double ts1 = __datetime_parse_date(date1);
    double ts2 = __datetime_parse_date(date2);
    if (ts1 < ts2) return -1;
    if (ts1 > ts2) return 1;
    return 0;
}

// ============================================================
// Duration Parsing & Formatting
// ============================================================

// Parse human-readable duration string to seconds
// Supports: "2h 30m", "1d 12h", "45s", "1h30m15s", "2d", "90m"
double __datetime_parse_duration(const char* dur_str) {
    if (!dur_str) return 0.0;
    double total = 0.0;
    const char* p = dur_str;

    while (*p) {
        // Skip whitespace
        while (*p == ' ' || *p == ',') p++;
        if (!*p) break;

        // Read number
        double val = 0;
        int has_num = 0;
        while (*p >= '0' && *p <= '9') {
            val = val * 10 + (*p - '0');
            has_num = 1;
            p++;
        }
        if (*p == '.') {
            p++;
            double frac = 0.1;
            while (*p >= '0' && *p <= '9') {
                val += (*p - '0') * frac;
                frac *= 0.1;
                p++;
            }
        }
        if (!has_num) { p++; continue; }

        // Skip optional space
        while (*p == ' ') p++;

        // Read unit
        if (*p == 'd' || strncmp(p, "day", 3) == 0) {
            total += val * 86400;
            while (*p && *p != ' ' && *p != ',') p++;
        } else if (*p == 'h' || strncmp(p, "hour", 4) == 0) {
            total += val * 3600;
            while (*p && *p != ' ' && *p != ',') p++;
        } else if (*p == 'm' && (*(p+1) != 's')) { // m but not ms
            total += val * 60;
            if (strncmp(p, "min", 3) == 0) { while (*p && *p != ' ' && *p != ',') p++; }
            else p++;
        } else if (*p == 's' || strncmp(p, "sec", 3) == 0) {
            total += val;
            while (*p && *p != ' ' && *p != ',') p++;
        } else if (strncmp(p, "ms", 2) == 0) {
            total += val / 1000.0;
            p += 2;
        } else if (*p == 'w' || strncmp(p, "week", 4) == 0) {
            total += val * 604800;
            while (*p && *p != ' ' && *p != ',') p++;
        } else {
            // No unit = seconds
            total += val;
        }
    }
    return total;
}

// Format seconds to human-readable duration
// e.g., 90061 -> "1d 1h 1m 1s"
char* __datetime_format_duration(double seconds) {
    char buf[128];
    char* p = buf;
    buf[0] = '\0';

    if (seconds < 0) { *p++ = '-'; seconds = -seconds; }

    long long total = (long long)seconds;
    int days = total / 86400;
    int hours = (total % 86400) / 3600;
    int mins = (total % 3600) / 60;
    int secs = total % 60;

    int first = 1;
    if (days > 0) { p += sprintf(p, "%dd", days); first = 0; }
    if (hours > 0) { p += sprintf(p, "%s%dh", first ? "" : " ", hours); first = 0; }
    if (mins > 0) { p += sprintf(p, "%s%dm", first ? "" : " ", mins); first = 0; }
    if (secs > 0 || first) { p += sprintf(p, "%s%ds", first ? "" : " ", secs); }

    return strdup(buf);
}

// ============================================================
// Age & Business Days (Desi-unique)
// ============================================================

// Calculate age in years from birthdate string
int __datetime_age(const char* birthdate) {
    if (!birthdate) return 0;
    int byear, bmonth, bday;
    if (sscanf(birthdate, "%d-%d-%d", &byear, &bmonth, &bday) != 3) return 0;

    time_t now = time(NULL);
    struct tm* today = localtime(&now);
    if (!today) return 0;

    int age = (today->tm_year + 1900) - byear;
    // If birthday hasn't occurred this year yet
    if ((today->tm_mon + 1) < bmonth ||
        ((today->tm_mon + 1) == bmonth && today->tm_mday < bday)) {
        age--;
    }
    return age;
}

// Count business days (Mon-Fri) between two dates
int __datetime_business_days(const char* start_str, const char* end_str) {
    double ts_start = __datetime_parse_date(start_str);
    double ts_end = __datetime_parse_date(end_str);
    if (ts_start == 0.0 || ts_end == 0.0) return 0;

    int count = 0;
    int direction = (ts_end >= ts_start) ? 1 : -1;
    double current = ts_start;

    while ((direction > 0 && current < ts_end) || (direction < 0 && current > ts_end)) {
        time_t t = (time_t)current;
        struct tm* tm = localtime(&t);
        if (tm && tm->tm_wday >= 1 && tm->tm_wday <= 5) {
            count++;
        }
        current += direction * 86400.0;
    }

    return count;
}

// ============================================================
// Relative Dates
// ============================================================

// Get next occurrence of a weekday ("Monday" .. "Sunday")
// Returns date string "YYYY-MM-DD"
char* __datetime_next_weekday(const char* weekday_name) {
    if (!weekday_name) return strdup("");

    const char* names[] = {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"};
    const char* short_names[] = {"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"};
    int target = -1;

    for (int i = 0; i < 7; i++) {
        if (strcasecmp(weekday_name, names[i]) == 0 || strcasecmp(weekday_name, short_names[i]) == 0) {
            target = i;
            break;
        }
    }
    if (target < 0) return strdup("");

    time_t now = time(NULL);
    struct tm* today = localtime(&now);
    if (!today) return strdup("");

    int current_wday = today->tm_wday;
    int diff = target - current_wday;
    if (diff <= 0) diff += 7; // always go forward

    double ts = (double)now + diff * 86400.0;
    return __datetime_to_date_str(ts);
}

// Get last (most recent) occurrence of a weekday
char* __datetime_last_weekday(const char* weekday_name) {
    if (!weekday_name) return strdup("");

    const char* names[] = {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"};
    const char* short_names[] = {"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"};
    int target = -1;

    for (int i = 0; i < 7; i++) {
        if (strcasecmp(weekday_name, names[i]) == 0 || strcasecmp(weekday_name, short_names[i]) == 0) {
            target = i;
            break;
        }
    }
    if (target < 0) return strdup("");

    time_t now = time(NULL);
    struct tm* today = localtime(&now);
    if (!today) return strdup("");

    int current_wday = today->tm_wday;
    int diff = current_wday - target;
    if (diff <= 0) diff += 7; // always go backward

    double ts = (double)now - diff * 86400.0;
    return __datetime_to_date_str(ts);
}

// Get start of week (Monday) for a given date
char* __datetime_start_of_week(const char* date_str) {
    double ts = __datetime_parse_date(date_str);
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return strdup("");

    int wday = tm->tm_wday;
    int days_back = (wday == 0) ? 6 : wday - 1; // Monday = 0 offset
    ts -= days_back * 86400.0;
    return __datetime_to_date_str(ts);
}

// Get end of week (Sunday) for a given date
char* __datetime_end_of_week(const char* date_str) {
    double ts = __datetime_parse_date(date_str);
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return strdup("");

    int wday = tm->tm_wday;
    int days_fwd = (wday == 0) ? 0 : 7 - wday;
    ts += days_fwd * 86400.0;
    return __datetime_to_date_str(ts);
}

// Get start of month for a given date
char* __datetime_start_of_month(const char* date_str) {
    if (!date_str) return strdup("");
    int year, month, day;
    if (sscanf(date_str, "%d-%d-%d", &year, &month, &day) != 3) return strdup("");
    char buf[32];
    snprintf(buf, sizeof(buf), "%04d-%02d-01", year, month);
    return strdup(buf);
}

// Get end of month for a given date
char* __datetime_end_of_month(const char* date_str) {
    if (!date_str) return strdup("");
    int year, month, day;
    if (sscanf(date_str, "%d-%d-%d", &year, &month, &day) != 3) return strdup("");
    int dim = __datetime_days_in_month(year, month);
    char buf[32];
    snprintf(buf, sizeof(buf), "%04d-%02d-%02d", year, month, dim);
    return strdup(buf);
}

// Quarter (1-4) from date string
int __datetime_quarter(const char* date_str) {
    if (!date_str) return 0;
    int year, month, day;
    if (sscanf(date_str, "%d-%d-%d", &year, &month, &day) != 3) return 0;
    return (month - 1) / 3 + 1;
}

// Check if a year is a leap year
int __datetime_is_leap_year(int year) {
    return ((year % 4 == 0 && year % 100 != 0) || (year % 400 == 0)) ? 1 : 0;
}

// Day of year (1-366) from date string
int __datetime_day_of_year(const char* date_str) {
    double ts = __datetime_parse_date(date_str);
    time_t t = (time_t)ts;
    struct tm* tm = localtime(&t);
    if (!tm) return 0;
    return tm->tm_yday + 1;
}
