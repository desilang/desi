// exception.h - Exception handling types and constants for Desi
//
// This header defines the exception type tags, the exception struct,
// and the exception frame used by setjmp/longjmp-based error handling.

#ifndef DESI_EXCEPTION_H
#define DESI_EXCEPTION_H

#include <setjmp.h>
#include <stdint.h>

// ==================== Exception Type Tags ====================
// These constants are shared between the runtime and the compiler.
// The compiler emits these tags when lowering `raise ExcType(msg)`.

#define DESI_EXC_EXCEPTION          0
#define DESI_EXC_VALUE_ERROR        1
#define DESI_EXC_KEY_ERROR          2
#define DESI_EXC_INDEX_ERROR        3
#define DESI_EXC_ZERO_DIVISION      4
#define DESI_EXC_IO_ERROR           5
#define DESI_EXC_RUNTIME_ERROR      6
#define DESI_EXC_OVERFLOW_ERROR     7
#define DESI_EXC_TIMEOUT_ERROR      8
#define DESI_EXC_CONNECTION_ERROR   9

// ==================== Exception Struct ====================

typedef struct DesiException {
    int32_t type_tag;        // One of DESI_EXC_* constants
    const char* message;     // Error message string
    const char* type_name;   // Human-readable type name (e.g., "ValueError")
} DesiException;

// ==================== Exception Frame ====================
// Linked-list stack of try handlers. Each try block pushes a frame.

typedef struct DesiExceptionFrame {
    jmp_buf buf;                        // setjmp buffer
    struct DesiExceptionFrame* prev;    // parent frame (for nesting)
} DesiExceptionFrame;

// ==================== API ====================

void __desi_try_push(DesiExceptionFrame* frame);
void __desi_try_pop(void);
void __desi_raise(int32_t type_tag, const char* message, const char* type_name);
DesiException* __desi_get_exception(void);
int32_t __desi_exception_tag(void);
const char* __desi_exception_message(void);
const char* __desi_exception_type_name(void);
int32_t __desi_exception_matches(int32_t caught_tag);
void __desi_reraise(void);

#endif // DESI_EXCEPTION_H
