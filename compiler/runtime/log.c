// log.c - Log module C runtime
// Level-filtered logging with ANSI colors
// Levels: DEBUG=0 < INFO=1 < WARN=2 < ERROR=3 < FATAL=4
#include <stdio.h>
#include <string.h>

// ANSI color codes
#define RESET   "\033[0m"
#define CYAN    "\033[36m"
#define BLUE    "\033[34m"
#define YELLOW  "\033[33m"
#define RED     "\033[31m"
#define BOLD_RED "\033[1;31m"

// Level constants
#define LVL_DEBUG 0
#define LVL_INFO  1
#define LVL_WARN  2
#define LVL_ERROR 3
#define LVL_FATAL 4

// Current log level (default: DEBUG = show everything)
static int current_level = LVL_DEBUG;

// Set the log level from a string name
void __log_set_level(const char* level) {
    if (strcmp(level, "DEBUG") == 0)      current_level = LVL_DEBUG;
    else if (strcmp(level, "INFO") == 0)  current_level = LVL_INFO;
    else if (strcmp(level, "WARN") == 0)  current_level = LVL_WARN;
    else if (strcmp(level, "ERROR") == 0) current_level = LVL_ERROR;
    else if (strcmp(level, "FATAL") == 0) current_level = LVL_FATAL;
}

void __log_debug(const char* msg) {
    if (current_level <= LVL_DEBUG)
        printf("%s[DEBUG]%s %s\n", CYAN, RESET, msg);
}

void __log_info(const char* msg) {
    if (current_level <= LVL_INFO)
        printf("%s[INFO]%s %s\n", BLUE, RESET, msg);
}

void __log_warn(const char* msg) {
    if (current_level <= LVL_WARN)
        fprintf(stderr, "%s[WARN]%s %s\n", YELLOW, RESET, msg);
}

void __log_error(const char* msg) {
    if (current_level <= LVL_ERROR)
        fprintf(stderr, "%s[ERROR]%s %s\n", RED, RESET, msg);
}

void __log_fatal(const char* msg) {
    if (current_level <= LVL_FATAL)
        fprintf(stderr, "%s[FATAL]%s %s\n", BOLD_RED, RESET, msg);
}
