// color.c - Terminal color module
// ANSI escape codes for terminal text styling. No external deps.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static char* wrap_ansi(const char* s, const char* code, const char* reset) {
    if (!s) return strdup("");
    size_t slen = strlen(s);
    size_t clen = strlen(code);
    size_t rlen = strlen(reset);
    char* result = (char*)malloc(clen + slen + rlen + 1);
    if (!result) return strdup(s);
    memcpy(result, code, clen);
    memcpy(result + clen, s, slen);
    memcpy(result + clen + slen, reset, rlen);
    result[clen + slen + rlen] = '\0';
    return result;
}

// ============================================================
// Foreground colors
// ============================================================
char* __color_red(const char* s)     { return wrap_ansi(s, "\033[31m", "\033[0m"); }
char* __color_green(const char* s)   { return wrap_ansi(s, "\033[32m", "\033[0m"); }
char* __color_yellow(const char* s)  { return wrap_ansi(s, "\033[33m", "\033[0m"); }
char* __color_blue(const char* s)    { return wrap_ansi(s, "\033[34m", "\033[0m"); }
char* __color_magenta(const char* s) { return wrap_ansi(s, "\033[35m", "\033[0m"); }
char* __color_cyan(const char* s)    { return wrap_ansi(s, "\033[36m", "\033[0m"); }
char* __color_white(const char* s)   { return wrap_ansi(s, "\033[37m", "\033[0m"); }
char* __color_gray(const char* s)    { return wrap_ansi(s, "\033[90m", "\033[0m"); }

// ============================================================
// Styles
// ============================================================
char* __color_bold(const char* s)      { return wrap_ansi(s, "\033[1m", "\033[0m"); }
char* __color_dim(const char* s)       { return wrap_ansi(s, "\033[2m", "\033[0m"); }
char* __color_italic(const char* s)    { return wrap_ansi(s, "\033[3m", "\033[0m"); }
char* __color_underline(const char* s) { return wrap_ansi(s, "\033[4m", "\033[0m"); }
char* __color_strike(const char* s)    { return wrap_ansi(s, "\033[9m", "\033[0m"); }

// ============================================================
// Background colors
// ============================================================
char* __color_bg_red(const char* s)    { return wrap_ansi(s, "\033[41m", "\033[0m"); }
char* __color_bg_green(const char* s)  { return wrap_ansi(s, "\033[42m", "\033[0m"); }
char* __color_bg_yellow(const char* s) { return wrap_ansi(s, "\033[43m", "\033[0m"); }
char* __color_bg_blue(const char* s)   { return wrap_ansi(s, "\033[44m", "\033[0m"); }

// ============================================================
// RGB (24-bit true color)
// ============================================================
char* __color_rgb(const char* s, int r, int g, int b) {
    if (!s) return strdup("");
    if (r < 0) r = 0; if (r > 255) r = 255;
    if (g < 0) g = 0; if (g > 255) g = 255;
    if (b < 0) b = 0; if (b > 255) b = 255;

    // \033[38;2;R;G;Bm ... \033[0m
    size_t slen = strlen(s);
    char* result = (char*)malloc(slen + 40);
    if (!result) return strdup(s);
    int prefix_len = sprintf(result, "\033[38;2;%d;%d;%dm", r, g, b);
    memcpy(result + prefix_len, s, slen);
    memcpy(result + prefix_len + slen, "\033[0m", 4);
    result[prefix_len + slen + 4] = '\0';
    return result;
}

char* __color_bg_rgb(const char* s, int r, int g, int b) {
    if (!s) return strdup("");
    if (r < 0) r = 0; if (r > 255) r = 255;
    if (g < 0) g = 0; if (g > 255) g = 255;
    if (b < 0) b = 0; if (b > 255) b = 255;

    size_t slen = strlen(s);
    char* result = (char*)malloc(slen + 40);
    if (!result) return strdup(s);
    int prefix_len = sprintf(result, "\033[48;2;%d;%d;%dm", r, g, b);
    memcpy(result + prefix_len, s, slen);
    memcpy(result + prefix_len + slen, "\033[0m", 4);
    result[prefix_len + slen + 4] = '\0';
    return result;
}

// ============================================================
// Strip ANSI codes
// ============================================================
char* __color_strip(const char* s) {
    if (!s) return strdup("");
    size_t len = strlen(s);
    char* result = (char*)malloc(len + 1);
    if (!result) return strdup("");

    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        if (s[i] == '\033') {
            // Skip until 'm'
            while (i < len && s[i] != 'm') i++;
            continue;
        }
        result[j++] = s[i];
    }
    result[j] = '\0';
    return result;
}
