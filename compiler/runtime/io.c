/*
 * io.c — I/O utilities for Desi
 *
 * Provides stdin reading, stderr writing, and line input.
 * print() is handled by the compiler directly via print.c.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Read a line from stdin (like Python's input())
// Displays prompt, reads until newline, strips trailing newline.
const char* __io_input(const char* prompt) {
    if (prompt && prompt[0]) {
        printf("%s", prompt);
        fflush(stdout);
    }
    
    char buf[4096];
    if (!fgets(buf, sizeof(buf), stdin)) {
        return strdup("");
    }
    
    // Strip trailing newline
    size_t len = strlen(buf);
    if (len > 0 && buf[len - 1] == '\n') buf[--len] = '\0';
    if (len > 0 && buf[len - 1] == '\r') buf[--len] = '\0';
    
    return strdup(buf);
}

// Read entire stdin as a string (until EOF)
const char* __io_read_all(void) {
    size_t cap = 4096, len = 0;
    char* buf = (char*)malloc(cap);
    
    int c;
    while ((c = fgetc(stdin)) != EOF) {
        if (len + 1 >= cap) {
            cap *= 2;
            buf = (char*)realloc(buf, cap);
        }
        buf[len++] = (char)c;
    }
    buf[len] = '\0';
    return buf;
}

// Write to stderr
void __io_eprint(const char* msg) {
    fprintf(stderr, "%s\n", msg ? msg : "");
}

// Write to stderr without newline
void __io_ewrite(const char* msg) {
    fprintf(stderr, "%s", msg ? msg : "");
    fflush(stderr);
}

// Flush stdout
void __io_flush(void) {
    fflush(stdout);
}

// Check if stdin has any data available (non-blocking check on POSIX)
int32_t __io_has_input(void) {
    return !feof(stdin);
}
