/*
 * fmt.c — String and number formatting for Desi stdlib
 *
 * Provides:
 *   - String padding (left, right, center)
 *   - Number formatting (comma-separated, fixed decimals)
 *   - Currency formatting
 *   - Percentage
 *   - Ordinal numbers (1st, 2nd, 3rd...)
 *   - Truncation with ellipsis
 *   - Repeat, reverse strings
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <stdint.h>

// ============================================================
// String Padding & Alignment
// ============================================================

// Left-pad string to width with fill char
char* __fmt_pad_left(const char* s, int32_t width, const char* fill) {
    if (!s) return strdup("");
    int slen = strlen(s);
    if (slen >= width) return strdup(s);
    
    char fc = (fill && fill[0]) ? fill[0] : ' ';
    int pad = width - slen;
    char* result = malloc(width + 1);
    for (int i = 0; i < pad; i++) result[i] = fc;
    memcpy(result + pad, s, slen);
    result[width] = '\0';
    return result;
}

// Right-pad string to width with fill char
char* __fmt_pad_right(const char* s, int32_t width, const char* fill) {
    if (!s) return strdup("");
    int slen = strlen(s);
    if (slen >= width) return strdup(s);
    
    char fc = (fill && fill[0]) ? fill[0] : ' ';
    int pad = width - slen;
    char* result = malloc(width + 1);
    memcpy(result, s, slen);
    for (int i = 0; i < pad; i++) result[slen + i] = fc;
    result[width] = '\0';
    return result;
}

// Center-pad string to width with fill char
char* __fmt_center(const char* s, int32_t width, const char* fill) {
    if (!s) return strdup("");
    int slen = strlen(s);
    if (slen >= width) return strdup(s);
    
    char fc = (fill && fill[0]) ? fill[0] : ' ';
    int total_pad = width - slen;
    int left_pad = total_pad / 2;
    int right_pad = total_pad - left_pad;
    
    char* result = malloc(width + 1);
    for (int i = 0; i < left_pad; i++) result[i] = fc;
    memcpy(result + left_pad, s, slen);
    for (int i = 0; i < right_pad; i++) result[left_pad + slen + i] = fc;
    result[width] = '\0';
    return result;
}

// Truncate with ellipsis
char* __fmt_truncate(const char* s, int32_t max_len) {
    if (!s) return strdup("");
    int slen = strlen(s);
    if (slen <= max_len) return strdup(s);
    if (max_len <= 3) {
        char* r = malloc(max_len + 1);
        for (int i = 0; i < max_len; i++) r[i] = '.';
        r[max_len] = '\0';
        return r;
    }
    char* result = malloc(max_len + 1);
    memcpy(result, s, max_len - 3);
    result[max_len - 3] = '.';
    result[max_len - 2] = '.';
    result[max_len - 1] = '.';
    result[max_len] = '\0';
    return result;
}

// ============================================================
// Number Formatting
// ============================================================

// Format integer with comma separators: 1234567 -> "1,234,567"
char* __fmt_comma(int64_t n) {
    char raw[32];
    int neg = n < 0;
    if (neg) n = -n;
    snprintf(raw, sizeof(raw), "%lld", (long long)n);
    
    int len = strlen(raw);
    int commas = (len - 1) / 3;
    int result_len = len + commas + (neg ? 1 : 0);
    char* result = malloc(result_len + 1);
    
    int ri = result_len;
    result[ri--] = '\0';
    
    for (int i = len - 1, count = 0; i >= 0; i--, count++) {
        if (count > 0 && count % 3 == 0) result[ri--] = ',';
        result[ri--] = raw[i];
    }
    if (neg) result[ri] = '-';
    
    return result;
}

// Format float with fixed decimal places: fmt.fixed(3.14159, 2) -> "3.14"
char* __fmt_fixed(double val, int32_t decimals) {
    char buf[64];
    snprintf(buf, sizeof(buf), "%.*f", decimals, val);
    return strdup(buf);
}

// Format as percentage: fmt.percent(0.857, 1) -> "85.7%"
char* __fmt_percent(double val, int32_t decimals) {
    char buf[64];
    snprintf(buf, sizeof(buf), "%.*f%%", decimals, val * 100.0);
    return strdup(buf);
}

// Format as currency: fmt.currency(1234.5, "$") -> "$1,234.50"
char* __fmt_currency(double val, const char* symbol) {
    if (!symbol) symbol = "$";
    int neg = val < 0;
    if (neg) val = -val;
    
    // Integer and decimal parts
    long long integer_part = (long long)val;
    int cents = (int)((val - integer_part) * 100.0 + 0.5);
    if (cents >= 100) { integer_part++; cents -= 100; }
    
    // Format integer with commas
    char* int_str = __fmt_comma(integer_part);
    
    char buf[128];
    if (neg) {
        snprintf(buf, sizeof(buf), "-%s%s.%02d", symbol, int_str, cents);
    } else {
        snprintf(buf, sizeof(buf), "%s%s.%02d", symbol, int_str, cents);
    }
    free(int_str);
    return strdup(buf);
}

// Ordinal number: 1 -> "1st", 2 -> "2nd", 3 -> "3rd", 4 -> "4th"
char* __fmt_ordinal(int32_t n) {
    char buf[32];
    int abs_n = n < 0 ? -n : n;
    int last_two = abs_n % 100;
    int last_one = abs_n % 10;
    
    const char* suffix;
    if (last_two >= 11 && last_two <= 13) {
        suffix = "th";
    } else if (last_one == 1) {
        suffix = "st";
    } else if (last_one == 2) {
        suffix = "nd";
    } else if (last_one == 3) {
        suffix = "rd";
    } else {
        suffix = "th";
    }
    
    snprintf(buf, sizeof(buf), "%d%s", n, suffix);
    return strdup(buf);
}

// ============================================================
// String Utilities
// ============================================================

// Repeat a string n times
char* __fmt_repeat(const char* s, int32_t count) {
    if (!s || count <= 0) return strdup("");
    int slen = strlen(s);
    int result_len = slen * count;
    char* result = malloc(result_len + 1);
    for (int i = 0; i < count; i++) {
        memcpy(result + i * slen, s, slen);
    }
    result[result_len] = '\0';
    return result;
}

// Reverse a string
char* __fmt_reverse(const char* s) {
    if (!s) return strdup("");
    int len = strlen(s);
    char* result = malloc(len + 1);
    for (int i = 0; i < len; i++) {
        result[i] = s[len - 1 - i];
    }
    result[len] = '\0';
    return result;
}

// Join list elements with separator (takes a separator and list)
// Note: This works with DesiList from list.h
#include "list.h"

char* __fmt_join(const char* separator, DesiList* list) {
    if (!separator || !list) return strdup("");
    int32_t len = list_len(list);
    if (len == 0) return strdup("");
    
    int sep_len = strlen(separator);
    // Calculate total length
    int total = 0;
    for (int i = 0; i < len; i++) {
        char* item = (char*)list_get(list, i);
        if (item) total += strlen(item);
        if (i < len - 1) total += sep_len;
    }
    
    char* result = malloc(total + 1);
    char* p = result;
    for (int i = 0; i < len; i++) {
        char* item = (char*)list_get(list, i);
        if (item) {
            int ilen = strlen(item);
            memcpy(p, item, ilen);
            p += ilen;
        }
        if (i < len - 1) {
            memcpy(p, separator, sep_len);
            p += sep_len;
        }
    }
    *p = '\0';
    return result;
}

// Byte size to human readable: 1536 -> "1.5 KB"
char* __fmt_bytes(int64_t bytes) {
    char buf[64];
    double val = (double)bytes;
    const char* units[] = {"B", "KB", "MB", "GB", "TB", "PB"};
    int unit = 0;
    
    while (val >= 1024.0 && unit < 5) {
        val /= 1024.0;
        unit++;
    }
    
    if (unit == 0) {
        snprintf(buf, sizeof(buf), "%lld B", (long long)bytes);
    } else {
        snprintf(buf, sizeof(buf), "%.1f %s", val, units[unit]);
    }
    return strdup(buf);
}
