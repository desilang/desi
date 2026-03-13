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
