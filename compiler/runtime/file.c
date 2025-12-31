// File I/O runtime support
// Design: Result-based error handling, refcounted handles, async-ready
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Opaque file handle with refcount for future sharing
typedef struct {
    FILE* handle;
    int refcount;
    int closed;  // Guard against double-close
} DesiFile;

// Result struct for file operations (tag: 0=Ok, 1=Err)
typedef struct {
    int tag;
    union {
        void* ok_value;
        char* err_msg;
    };
} FileResult;

// Open file - returns DesiFile* on success, NULL + sets error on failure
DesiFile* file_open(const char* path, const char* mode, char** err_out) {
    if (!path || !mode) {
        if (err_out) *err_out = strdup("invalid arguments");
        return NULL;
    }
    
    FILE* f = fopen(path, mode);
    if (!f) {
        if (err_out) {
            char buf[256];
            snprintf(buf, sizeof(buf), "cannot open '%s': file not found or permission denied", path);
            *err_out = strdup(buf);
        }
        return NULL;
    }
    
    DesiFile* df = (DesiFile*)malloc(sizeof(DesiFile));
    if (!df) {
        fclose(f);
        if (err_out) *err_out = strdup("memory allocation failed");
        return NULL;
    }
    
    df->handle = f;
    df->refcount = 1;
    df->closed = 0;
    return df;
}

// Read entire file contents - caller must free returned string
// Returns 0 on success (out filled), 1 on error (err_out filled)
int file_read_all(DesiFile* f, char** out, char** err_out) {
    if (!f || f->closed || !f->handle) {
        if (err_out) *err_out = strdup("file handle is closed or invalid");
        return 1;
    }
    
    // Get file size
    long start = ftell(f->handle);
    if (fseek(f->handle, 0, SEEK_END) != 0) {
        if (err_out) *err_out = strdup("failed to seek in file");
        return 1;
    }
    long size = ftell(f->handle);
    if (fseek(f->handle, start, SEEK_SET) != 0) {
        if (err_out) *err_out = strdup("failed to seek in file");
        return 1;
    }
    
    // Handle empty file
    if (size <= 0) {
        *out = strdup("");
        return 0;
    }
    
    // Allocate and read
    char* buf = (char*)malloc(size + 1);
    if (!buf) {
        if (err_out) *err_out = strdup("memory allocation failed");
        return 1;
    }
    
    size_t read = fread(buf, 1, size, f->handle);
    buf[read] = '\0';
    *out = buf;
    return 0;
}

// Write string to file
// Returns 0 on success, 1 on error
int file_write(DesiFile* f, const char* data, char** err_out) {
    if (!f || f->closed || !f->handle) {
        if (err_out) *err_out = strdup("file handle is closed or invalid");
        return 1;
    }
    
    if (!data) data = "";
    
    size_t len = strlen(data);
    if (len > 0) {
        size_t written = fwrite(data, 1, len, f->handle);
        if (written != len) {
            if (err_out) *err_out = strdup("write failed: not all bytes written");
            return 1;
        }
    }
    
    // Flush to ensure data is written
    fflush(f->handle);
    return 0;
}

// Close file handle
void file_close(DesiFile* f) {
    if (!f) return;
    
    // Decrement refcount
    f->refcount--;
    
    // Only close if refcount hits 0 and not already closed
    if (f->refcount <= 0 && !f->closed && f->handle) {
        fclose(f->handle);
        f->handle = NULL;
        f->closed = 1;
    }
}

// Check if file is still open
int file_is_open(DesiFile* f) {
    return f && !f->closed && f->handle != NULL;
}

// Increment refcount (for future sharing)
void file_retain(DesiFile* f) {
    if (f) f->refcount++;
}

// Free file handle (decrements refcount, closes if needed)
void file_free(DesiFile* f) {
    if (!f) return;
    file_close(f);
    if (f->refcount <= 0) {
        free(f);
    }
}

// ============================================================
// DesiStream bridge for print(file=f) support
// ============================================================

// Forward declaration of DesiStream from print.c
typedef struct {
    FILE* handle;
    int is_owned;
} DesiStream;

// Static stream wrapper for user files (not owned - file manages lifetime)
static DesiStream __file_stream = {NULL, 0};

// Get a DesiStream* from a DesiFile* for print() compatibility
// Returns a temporary stream wrapping the file's handle
DesiStream* desifile_get_stream(DesiFile* f) {
    if (!f || f->closed || !f->handle) {
        return NULL;
    }
    // Wrap the file's handle in a static stream (not owned = won't close)
    __file_stream.handle = f->handle;
    __file_stream.is_owned = 0;  // Don't close - DesiFile manages lifetime
    return &__file_stream;
}
