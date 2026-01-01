#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

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
