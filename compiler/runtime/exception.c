// exception.c - Exception handling runtime for Desi
// Implements setjmp/longjmp-based catchable exceptions
//
// Architecture:
//   try:  → __desi_try_push(frame) + setjmp(frame.buf)
//   raise → __desi_raise(tag, msg, type_name) → longjmp to handler
//   except → __desi_get_exception() to read caught exception
//
// Performance: ~5-10ns per try entry (setjmp cost on ARM64)
// Upgrade path: replace with LLVM invoke/landingpad for zero-cost happy path

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <setjmp.h>
#include <stdint.h>

#include "exception.h"

// ==================== Exception Type Tags ====================
// These must match the tags used by the compiler's type checker

// Tag 0 = Exception (base, catches all)
// Tag 1 = ValueError
// Tag 2 = KeyError
// Tag 3 = IndexError
// Tag 4 = ZeroDivisionError
// Tag 5 = IOError
// Tag 6 = RuntimeError
// Tag 7 = OverflowError
// Tag 8 = TimeoutError
// Tag 9 = ConnectionError

// ==================== Thread-local State ====================
// Each thread gets its own exception handler stack and current exception.
// This is critical for thread-safety when using try/except inside
// TaskGroup tasks or spawned lambdas.

#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L && !defined(__STDC_NO_THREADS__)
  #define DESI_THREAD_LOCAL _Thread_local
#elif defined(__GNUC__) || defined(__clang__)
  #define DESI_THREAD_LOCAL __thread
#elif defined(_MSC_VER)
  #define DESI_THREAD_LOCAL __declspec(thread)
#else
  #define DESI_THREAD_LOCAL /* fallback: no thread-local, single-threaded only */
#endif

static DESI_THREAD_LOCAL DesiExceptionFrame* current_frame = NULL;
static DESI_THREAD_LOCAL DesiException current_exception = {0, NULL, NULL};

// ==================== API ====================

// Push a try frame onto the handler stack.
// Called at the top of a try block, before setjmp.
void __desi_try_push(DesiExceptionFrame* frame) {
    frame->prev = current_frame;
    current_frame = frame;
}

// Pop the current try frame from the handler stack.
// Called when try body completes normally (no exception).
void __desi_try_pop(void) {
    if (current_frame) {
        current_frame = current_frame->prev;
    }
}

// Raise an exception: store it and longjmp to the nearest handler.
// If no handler is active, print and exit (unhandled exception).
void __desi_raise(int32_t type_tag, const char* message, const char* type_name) {
    current_exception.type_tag = type_tag;
    current_exception.message = message ? message : "(null)";
    current_exception.type_name = type_name ? type_name : "Exception";

    if (current_frame) {
        DesiExceptionFrame* frame = current_frame;
        // Pop before jumping so re-raise inside except finds parent handler
        current_frame = frame->prev;
        longjmp(frame->buf, 1);
    } else {
        // Unhandled exception — print and exit
        fprintf(stderr, "Unhandled %s: %s\n", current_exception.type_name,
                current_exception.message);
        exit(1);
    }
}

// Get a pointer to the current exception (for except handler to read).
DesiException* __desi_get_exception(void) {
    return &current_exception;
}

// Get the type tag of the current exception.
int32_t __desi_exception_tag(void) {
    return current_exception.type_tag;
}

// Get the message string of the current exception.
const char* __desi_exception_message(void) {
    return current_exception.message;
}

// Get the type name string of the current exception.
const char* __desi_exception_type_name(void) {
    return current_exception.type_name;
}

// Check if an exception matches a given type tag.
// Exception (tag 0) matches ALL exception types (it's the base).
int32_t __desi_exception_matches(int32_t caught_tag) {
    if (caught_tag == DESI_EXC_EXCEPTION) {
        // Base Exception catches everything
        return 1;
    }
    return current_exception.type_tag == caught_tag;
}

// Re-raise the current exception (for except blocks that don't handle it).
void __desi_reraise(void) {
    __desi_raise(current_exception.type_tag,
                 current_exception.message,
                 current_exception.type_name);
}
