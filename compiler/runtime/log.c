// log.c - Log module C runtime
// Level-filtered logging with ANSI colors, timestamps, and file output
// Levels: DEBUG=0 < INFO=1 < WARN=2 < ERROR=3 < FATAL=4
#include <stdio.h>
#include <string.h>
#include <time.h>
#include <stdlib.h>

// ANSI color codes
#define RESET   "\033[0m"
#define CYAN    "\033[36m"
#define BLUE    "\033[34m"
#define YELLOW  "\033[33m"
#define RED     "\033[31m"
#define BOLD_RED "\033[1;31m"
#define DIM     "\033[2m"

// Level constants
#define LVL_DEBUG 0
#define LVL_INFO  1
#define LVL_WARN  2
#define LVL_ERROR 3
#define LVL_FATAL 4

// Configuration
static int current_level = LVL_DEBUG;
static int show_timestamp = 0;     // 0=off, 1=on
static int json_format = 0;        // 0=text, 1=JSON
static FILE* log_file = NULL;      // NULL=use stdout/stderr, else file

// Get current timestamp as ISO 8601 string
static void get_timestamp(char* buf, size_t size) {
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    strftime(buf, size, "%Y-%m-%dT%H:%M:%S", tm_info);
}

// Get level name from integer
static const char* level_name(int level) {
    switch (level) {
        case LVL_DEBUG: return "DEBUG";
        case LVL_INFO:  return "INFO";
        case LVL_WARN:  return "WARN";
        case LVL_ERROR: return "ERROR";
        case LVL_FATAL: return "FATAL";
        default:        return "UNKNOWN";
    }
}

// Get ANSI color for level
static const char* level_color(int level) {
    switch (level) {
        case LVL_DEBUG: return CYAN;
        case LVL_INFO:  return BLUE;
        case LVL_WARN:  return YELLOW;
        case LVL_ERROR: return RED;
        case LVL_FATAL: return BOLD_RED;
        default:        return RESET;
    }
}

// Core log function — all public log functions route through this
static void log_message(int level, const char* msg) {
    if (level < current_level) return;
    if (!msg) msg = "";

    FILE* target;
    if (log_file) {
        target = log_file;
    } else {
        target = (level >= LVL_WARN) ? stderr : stdout;
    }

    if (json_format) {
        // Structured JSON format: {"level":"INFO","time":"...","message":"..."}
        char ts[32] = "";
        if (show_timestamp) get_timestamp(ts, sizeof(ts));
        fprintf(target, "{\"level\":\"%s\"", level_name(level));
        if (show_timestamp) fprintf(target, ",\"time\":\"%s\"", ts);
        // Escape message for JSON (basic: escape quotes and backslashes)
        fprintf(target, ",\"message\":\"");
        for (const char* p = msg; *p; p++) {
            switch (*p) {
                case '"':  fprintf(target, "\\\""); break;
                case '\\': fprintf(target, "\\\\"); break;
                case '\n': fprintf(target, "\\n"); break;
                case '\r': fprintf(target, "\\r"); break;
                case '\t': fprintf(target, "\\t"); break;
                default:   fputc(*p, target); break;
            }
        }
        fprintf(target, "\"}\n");
    } else {
        // Human-readable text format
        int use_color = (log_file == NULL); // Only colorize console output
        if (show_timestamp) {
            char ts[32];
            get_timestamp(ts, sizeof(ts));
            if (use_color) {
                fprintf(target, "%s%s%s %s[%s]%s %s\n",
                    DIM, ts, RESET,
                    level_color(level), level_name(level), RESET, msg);
            } else {
                fprintf(target, "%s [%s] %s\n", ts, level_name(level), msg);
            }
        } else {
            if (use_color) {
                fprintf(target, "%s[%s]%s %s\n",
                    level_color(level), level_name(level), RESET, msg);
            } else {
                fprintf(target, "[%s] %s\n", level_name(level), msg);
            }
        }
    }
    fflush(target);
}

// ============================================================
// Public API
// ============================================================

// Set the log level from a string name
void __log_set_level(const char* level) {
    if (strcmp(level, "DEBUG") == 0)      current_level = LVL_DEBUG;
    else if (strcmp(level, "INFO") == 0)  current_level = LVL_INFO;
    else if (strcmp(level, "WARN") == 0)  current_level = LVL_WARN;
    else if (strcmp(level, "ERROR") == 0) current_level = LVL_ERROR;
    else if (strcmp(level, "FATAL") == 0) current_level = LVL_FATAL;
}

// Enable/disable timestamps on log messages
void __log_set_timestamps(int enabled) {
    show_timestamp = enabled ? 1 : 0;
}

// Enable/disable JSON structured output
void __log_set_json(int enabled) {
    json_format = enabled ? 1 : 0;
}

// Set log output to a file (pass NULL or "" to reset to stdout/stderr)
void __log_set_file(const char* path) {
    if (log_file && log_file != stdout && log_file != stderr) {
        fclose(log_file);
        log_file = NULL;
    }
    if (path && *path) {
        log_file = fopen(path, "a");
    }
}

void __log_debug(const char* msg) { log_message(LVL_DEBUG, msg); }
void __log_info(const char* msg)  { log_message(LVL_INFO, msg); }
void __log_warn(const char* msg)  { log_message(LVL_WARN, msg); }
void __log_error(const char* msg) { log_message(LVL_ERROR, msg); }
void __log_fatal(const char* msg) { log_message(LVL_FATAL, msg); }
