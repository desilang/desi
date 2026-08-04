#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
  #include <windows.h>
  #include <io.h>
  #include <fcntl.h>
  // MSVC CRT globals for command-line arguments
  extern int __argc;
  extern char** __argv;
  // Forward declaration for args module init
  extern void __args_init(int argc, char** argv);
#else
  #include <pthread.h>
#endif

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

// ============================================================
// Print serialization
// ============================================================
//
// One Desi `print(...)` lowers to several output calls — one per value, plus
// the separators and the end terminator. Each is individually atomic, but the
// sequence is not, so two tasks printing at once used to interleave inside a
// single line:
//
//     WorkerWorker  12 working working
//
// The lowerer brackets the sequence with the two functions below so that one
// print is one uninterrupted line. It evaluates every argument *before* taking
// the lock, which is what keeps an argument that blocks — `print(ch.recv())` —
// from holding this lock while another task waits on it.
//
// The lock is still recursive, because a to_str dunder called during emission
// can itself print. A plain pthread mutex and an SRWLOCK would both deadlock
// there, so neither DESI_MUTEX_* nor SRWLOCK is used here.
#ifdef _WIN32
static CRITICAL_SECTION __print_cs;
#else
static pthread_mutex_t __print_mx;
#endif
static int __print_lock_ready = 0;

// Set the first time anything in the runtime starts a thread, and never
// cleared. Until then there is exactly one thread, so the lock protects
// nothing and print skips it: measured at 13.9 ns for the mutex pair against
// 1.7 ns for this load, which is the same as not checking at all.
//
// It is deliberately a runtime flag rather than something the compiler decides.
// The obvious compile-time test -- "this program contains no spawn" -- is not
// sound: future.c, scheduler.c, supervisor.c and websocket.c all start threads
// from code a program reaches through async, http.serve or a supervisor without
// ever writing spawn. Proving those unreachable is a whole-program call-graph
// question, and getting it wrong tears output lines. The flag is correct by
// construction and, per the measurement above, gives up nothing to get there.
//
// Ordering is not delicate. Every writer sets it before creating the thread
// that could race, and thread creation is itself a synchronisation point, so
// any thread able to reach print has already seen the write.
volatile int __desi_threads_started = 0;

// Called by every runtime path that is about to start a thread, before it
// starts it. runtime_thread_flag_test.go checks that no source file creates a
// thread without calling this.
void __desi_note_thread_start(void) {
    __desi_threads_started = 1;
}

#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L && !defined(__STDC_NO_THREADS__)
  #define DESI_PRINT_TLS _Thread_local
#elif defined(__GNUC__) || defined(__clang__)
  #define DESI_PRINT_TLS __thread
#elif defined(_MSC_VER)
  #define DESI_PRINT_TLS __declspec(thread)
#else
  #define DESI_PRINT_TLS
#endif

// How many times this thread currently holds the print lock. Unlock consults
// this rather than the flag, because the flag can turn on between a lock and
// its unlock: the one thread in existence can start one from inside a print,
// through a to_str dunder that spawns. Skipping the lock and then unlocking a
// mutex this thread never took would be a genuine error, so the decision is
// recorded rather than recomputed.
static DESI_PRINT_TLS int __print_depth = 0;

static void __desi_print_lock_init(void) {
    if (__print_lock_ready) return;
#ifdef _WIN32
    InitializeCriticalSection(&__print_cs);
#else
    pthread_mutexattr_t attr;
    pthread_mutexattr_init(&attr);
    pthread_mutexattr_settype(&attr, PTHREAD_MUTEX_RECURSIVE);
    pthread_mutex_init(&__print_mx, &attr);
    pthread_mutexattr_destroy(&attr);
#endif
    __print_lock_ready = 1;
}

void __desi_print_lock(void) {
    // Nothing to serialise against until a second thread exists.
    if (!__desi_threads_started) return;
    // __desi_runtime_init initializes this before any task exists; the guard
    // only covers a print reached before that, which is single-threaded.
    if (!__print_lock_ready) __desi_print_lock_init();
#ifdef _WIN32
    EnterCriticalSection(&__print_cs);
#else
    pthread_mutex_lock(&__print_mx);
#endif
    __print_depth++;
}

void __desi_print_unlock(void) {
    if (__print_depth == 0) return; // this thread never took it
    if (!__print_lock_ready) return;
    __print_depth--;
#ifdef _WIN32
    LeaveCriticalSection(&__print_cs);
#else
    pthread_mutex_unlock(&__print_mx);
#endif
}

// Runtime initialization - called at program start
void __desi_runtime_init(void) {
    __desi_print_lock_init();

#ifdef _WIN32
    // Always output UTF-8 — same behavior as Python/Rust on all platforms.
    // This must run for ALL programs (def main + script mode) on Windows.
    // Without this, Windows uses the system code page (e.g. 850/1252),
    // which garbles non-ASCII characters and emoji.
    SetConsoleOutputCP(CP_UTF8);
    SetConsoleCP(CP_UTF8);
    // Set stdio to binary mode so UTF-8 bytes aren't mangled by CRLF translation
    _setmode(_fileno(stdout), _O_BINARY);
    _setmode(_fileno(stderr), _O_BINARY);
    // Initialize CLI args using MSVC CRT globals so the args module works
    // for def-main programs where entry.c's main() is bypassed by the linker.
    __args_init(__argc, __argv);
#endif

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
    if (s && s->handle) {
        // Python-like formatting: ensure floats always show decimal point
        // e.g., 1.0 not 1, but 3.14 stays 3.14
        char buf[64];
        snprintf(buf, sizeof(buf), "%g", f);
        // Check if result has decimal point or 'e' (scientific notation)
        int has_decimal = 0;
        for (int i = 0; buf[i]; i++) {
            if (buf[i] == '.' || buf[i] == 'e' || buf[i] == 'E') {
                has_decimal = 1;
                break;
            }
        }
        if (has_decimal) {
            fputs(buf, s->handle);
        } else {
            fprintf(s->handle, "%s.0", buf);
        }
    }
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

const char* bool_to_cstring(int b) {
    return b ? "true" : "false";
}

// Python-like float formatting: ensures .0 for whole numbers
void print_float_py(double f) {
    char buf[64];
    snprintf(buf, sizeof(buf), "%g", f);
    // Check if result has decimal point or 'e' (scientific notation)
    int has_decimal = 0;
    for (int i = 0; buf[i]; i++) {
        if (buf[i] == '.' || buf[i] == 'e' || buf[i] == 'E') {
            has_decimal = 1;
            break;
        }
    }
    if (has_decimal) {
        printf("%s\n", buf);
    } else {
        printf("%s.0\n", buf);
    }
}

// Python-like float formatting without newline (for print_item)
void print_float_item_py(double f) {
    char buf[64];
    snprintf(buf, sizeof(buf), "%g", f);
    int has_decimal = 0;
    for (int i = 0; buf[i]; i++) {
        if (buf[i] == '.' || buf[i] == 'e' || buf[i] == 'E') {
            has_decimal = 1;
            break;
        }
    }
    if (has_decimal) {
        printf("%s", buf);
    } else {
        printf("%s.0", buf);
    }
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
    if (stream) {
        char buf[64];
        snprintf(buf, sizeof(buf), "%g", f);
        int has_decimal = 0;
        for (int i = 0; buf[i]; i++) {
            if (buf[i] == '.' || buf[i] == 'e' || buf[i] == 'E') {
                has_decimal = 1;
                break;
            }
        }
        if (has_decimal) {
            fputs(buf, stream);
        } else {
            fprintf(stream, "%s.0", buf);
        }
    }
}

void print_bool_to_stream(FILE* stream, int b) {
    if (stream) fputs(b ? "true" : "false", stream);
}


