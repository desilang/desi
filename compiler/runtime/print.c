#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

// ============================================================
// DesiStream: Unified stream type for all I/O operations
// Same type works for stdout, stderr, and user files
// ============================================================

typedef struct {
    FILE* handle;      // Underlying C stream
    int is_owned;      // 1 = close on free, 0 = static (stdout/stderr)
} DesiStream;

// Global streams - initialized by __desi_runtime_init
static DesiStream __stdout_stream = {NULL, 0};
static DesiStream __stderr_stream = {NULL, 0};

DesiStream* __desi_stdout = NULL;
DesiStream* __desi_stderr = NULL;

// Runtime initialization - called at program start
void __desi_runtime_init(void) {
    __stdout_stream.handle = stdout;
    __stdout_stream.is_owned = 0;
    __desi_stdout = &__stdout_stream;
    
    __stderr_stream.handle = stderr;
    __stderr_stream.is_owned = 0;
    __desi_stderr = &__stderr_stream;
}

// Accessor functions for sys.stdout/sys.stderr
DesiStream* __get_stdout(void) {
    return __desi_stdout;
}

DesiStream* __get_stderr(void) {
    return __desi_stderr;
}

// ============================================================
// Stream print functions (generic - work with any DesiStream)
// ============================================================

void stream_print_str(DesiStream* s, const char* str) {
    if (s && s->handle && str) fputs(str, s->handle);
}

void stream_print_int(DesiStream* s, int64_t n) {
    if (s && s->handle) fprintf(s->handle, "%lld", (long long)n);
}

void stream_print_float(DesiStream* s, double f) {
    if (s && s->handle) fprintf(s->handle, "%g", f);
}

void stream_print_bool(DesiStream* s, int b) {
    if (s && s->handle) fputs(b ? "true" : "false", s->handle);
}

void stream_flush(DesiStream* s) {
    if (s && s->handle) fflush(s->handle);
}

// ============================================================
// ANSI color codes for styled output
// ============================================================

// Reset code
#define ANSI_RESET "\033[0m"

// Print start color codes based on style string
void print_style_start(FILE* stream, const char* style) {
    if (!stream || !style) return;
    
    // Simple style parsing: comma-separated values
    // Supported: red, green, yellow, blue, magenta, cyan, white, bold, underline
    const char* p = style;
    while (*p) {
        // Skip whitespace and commas
        while (*p == ' ' || *p == ',') p++;
        if (!*p) break;
        
        // Match style names
        if (strncmp(p, "red", 3) == 0) { fprintf(stream, "\033[31m"); p += 3; }
        else if (strncmp(p, "green", 5) == 0) { fprintf(stream, "\033[32m"); p += 5; }
        else if (strncmp(p, "yellow", 6) == 0) { fprintf(stream, "\033[33m"); p += 6; }
        else if (strncmp(p, "blue", 4) == 0) { fprintf(stream, "\033[34m"); p += 4; }
        else if (strncmp(p, "magenta", 7) == 0) { fprintf(stream, "\033[35m"); p += 7; }
        else if (strncmp(p, "cyan", 4) == 0) { fprintf(stream, "\033[36m"); p += 4; }
        else if (strncmp(p, "white", 5) == 0) { fprintf(stream, "\033[37m"); p += 5; }
        else if (strncmp(p, "bold", 4) == 0) { fprintf(stream, "\033[1m"); p += 4; }
        else if (strncmp(p, "underline", 9) == 0) { fprintf(stream, "\033[4m"); p += 9; }
        else if (strncmp(p, "dim", 3) == 0) { fprintf(stream, "\033[2m"); p += 3; }
        else { p++; } // skip unknown characters
    }
}

void print_style_end(FILE* stream) {
    if (stream) fprintf(stream, ANSI_RESET);
}

// Styled print: outputs text with ANSI colors
void print_styled(const char* text, const char* style) {
    if (!text) return;
    print_style_start(stdout, style);
    fputs(text, stdout);
    print_style_end(stdout);
}

void stream_print_styled(DesiStream* s, const char* text, const char* style) {
    if (!s || !s->handle || !text) return;
    print_style_start(s->handle, style);
    fputs(text, s->handle);
    print_style_end(s->handle);
}

// Convenience wrappers for stdout
void print_style_start_stdout(const char* style) {
    print_style_start(stdout, style);
}

void print_style_end_stdout(void) {
    print_style_end(stdout);
}

// ============================================================
// Legacy print functions (kept for backwards compatibility)
// ============================================================

void print_int(int64_t n) {
    printf("%lld\n", (long long)n);
}

const char* bool_to_cstring(bool b) {
    return b ? "true" : "false";
}

void print_str(const char* s) {
    puts(s);
}

// Legacy stream functions (FILE* based, for gradual migration)
void print_str_to_stream(FILE* stream, const char* s) {
    if (stream && s) fputs(s, stream);
}

void print_int_to_stream(FILE* stream, int64_t n) {
    if (stream) fprintf(stream, "%lld", (long long)n);
}

void print_float_to_stream(FILE* stream, double f) {
    if (stream) fprintf(stream, "%g", f);
}

void print_bool_to_stream(FILE* stream, int b) {
    if (stream) fputs(b ? "true" : "false", stream);
}

// ============================================================
// Log functions with colored prefixes
// ============================================================

void log_info(const char* msg) {
    fprintf(stdout, "\033[32m[INFO]\033[0m %s\n", msg ? msg : "");
}

void log_warn(const char* msg) {
    fprintf(stderr, "\033[33m[WARN]\033[0m %s\n", msg ? msg : "");
}

void log_error(const char* msg) {
    fprintf(stderr, "\033[31m[ERROR]\033[0m %s\n", msg ? msg : "");
}

void log_debug(const char* msg) {
    fprintf(stdout, "\033[2m[DEBUG]\033[0m %s\n", msg ? msg : "");
}
