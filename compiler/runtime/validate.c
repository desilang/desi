// validate.c - Input validation module
// Pure C, no external deps. Provides common validation checks.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

// ============================================================
// Email validation (simplified RFC-ish check)
// ============================================================
int __validate_is_email(const char* s) {
    if (!s) return 0;
    size_t len = strlen(s);
    if (len < 3 || len > 254) return 0;

    const char* at = strchr(s, '@');
    if (!at || at == s) return 0;
    if (strchr(at + 1, '@')) return 0; // multiple @

    // Must have dot after @
    const char* dot = strrchr(at, '.');
    if (!dot || dot == at + 1) return 0;
    if (dot[1] == '\0') return 0; // trailing dot
    if ((size_t)(dot - at) < 2) return 0;

    // Local part: no spaces
    for (const char* p = s; p < at; p++) {
        if (*p == ' ') return 0;
    }
    // Domain: letters, digits, dots, hyphens
    for (const char* p = at + 1; *p; p++) {
        if (!isalnum((unsigned char)*p) && *p != '.' && *p != '-') return 0;
    }
    return 1;
}

// ============================================================
// URL validation
// ============================================================
int __validate_is_url(const char* s) {
    if (!s) return 0;
    if (strncmp(s, "http://", 7) == 0 || strncmp(s, "https://", 8) == 0) {
        const char* after = strchr(s + 7, '/');
        // Must have at least host after scheme
        const char* host = s + (s[4] == 's' ? 8 : 7);
        if (strlen(host) == 0) return 0;
        // Host can't start with dot
        if (*host == '.') return 0;
        return 1;
    }
    return 0;
}

// ============================================================
// IP validation
// ============================================================
int __validate_is_ipv4(const char* s) {
    if (!s) return 0;
    int parts[4];
    int n = sscanf(s, "%d.%d.%d.%d", &parts[0], &parts[1], &parts[2], &parts[3]);
    if (n != 4) return 0;

    // Verify no extra characters
    char buf[20];
    snprintf(buf, sizeof(buf), "%d.%d.%d.%d", parts[0], parts[1], parts[2], parts[3]);
    if (strcmp(buf, s) != 0) return 0;

    for (int i = 0; i < 4; i++) {
        if (parts[i] < 0 || parts[i] > 255) return 0;
    }
    return 1;
}

int __validate_is_ipv6(const char* s) {
    if (!s) return 0;
    size_t len = strlen(s);
    if (len < 2 || len > 45) return 0;

    int colons = 0;
    int double_colon = 0;
    for (size_t i = 0; i < len; i++) {
        char c = s[i];
        if (c == ':') {
            colons++;
            if (i + 1 < len && s[i + 1] == ':') {
                double_colon++;
                if (double_colon > 1) return 0;
            }
        } else if (!isxdigit((unsigned char)c)) {
            return 0;
        }
    }
    if (colons < 2 || colons > 7) return 0;
    return 1;
}

// ============================================================
// Format checks
// ============================================================
int __validate_is_hex(const char* s) {
    if (!s || *s == '\0') return 0;
    // Skip 0x prefix
    if (s[0] == '0' && (s[1] == 'x' || s[1] == 'X')) s += 2;
    if (*s == '\0') return 0;
    for (; *s; s++) {
        if (!isxdigit((unsigned char)*s)) return 0;
    }
    return 1;
}

int __validate_is_json(const char* s) {
    if (!s) return 0;
    // Skip whitespace
    while (*s && isspace((unsigned char)*s)) s++;
    if (*s == '\0') return 0;
    // Must start with { or [ or " or digit or true/false/null
    return (*s == '{' || *s == '[' || *s == '"' ||
            isdigit((unsigned char)*s) || *s == '-' ||
            strncmp(s, "true", 4) == 0 || strncmp(s, "false", 5) == 0 ||
            strncmp(s, "null", 4) == 0);
}

int __validate_is_uuid(const char* s) {
    if (!s || strlen(s) != 36) return 0;
    for (int i = 0; i < 36; i++) {
        if (i == 8 || i == 13 || i == 18 || i == 23) {
            if (s[i] != '-') return 0;
        } else {
            if (!isxdigit((unsigned char)s[i])) return 0;
        }
    }
    return 1;
}

int __validate_is_semver(const char* s) {
    if (!s) return 0;
    int major, minor, patch;
    char extra;
    int n = sscanf(s, "%d.%d.%d%c", &major, &minor, &patch, &extra);
    if (n < 3) return 0;
    if (major < 0 || minor < 0 || patch < 0) return 0;
    // Allow pre-release suffix like -beta, -rc.1
    if (n == 4 && extra != '-' && extra != '+') return 0;
    return 1;
}
