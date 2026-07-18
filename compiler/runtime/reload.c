// reload.c - Hot reload state serialization support
// These functions allow Desi programs to save and restore state between reloads.

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

// Check if this is a reload (not first run)
// Returns true if DESI_RELOAD=true
bool __desi_is_reload(void) {
    const char* val = getenv("DESI_RELOAD");
    return val != NULL && strcmp(val, "true") == 0;
}

// Get the reload count (how many times we've reloaded)
// Returns 0 if DESI_RELOAD_COUNT is not set
int __desi_reload_count(void) {
    const char* val = getenv("DESI_RELOAD_COUNT");
    if (val == NULL) return 0;
    return atoi(val);
}

// Get the state file path
// Returns the path from DESI_STATE_FILE, or NULL if not set
const char* __desi_state_file(void) {
    return getenv("DESI_STATE_FILE");
}

// Write state to the state file (JSON string)
// Returns true on success, false on failure
bool __desi_write_state(const char* json_str) {
    const char* path = __desi_state_file();
    if (path == NULL || json_str == NULL) return false;
    
    FILE* f = fopen(path, "w");
    if (f == NULL) return false;
    
    size_t len = strlen(json_str);
    size_t written = fwrite(json_str, 1, len, f);
    fclose(f);
    
    return written == len;
}

// Read state from the state file
// Returns the JSON string (caller must free), or NULL if file doesn't exist
char* __desi_read_state(void) {
    const char* path = __desi_state_file();
    if (path == NULL) return NULL;
    
    FILE* f = fopen(path, "r");
    if (f == NULL) return NULL;
    
    // Get file size
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    
    if (size <= 0) {
        fclose(f);
        return NULL;
    }
    
    // Allocate buffer and read
    char* buffer = (char*)malloc(size + 1);
    if (buffer == NULL) {
        fclose(f);
        return NULL;
    }
    
    size_t read_bytes = fread(buffer, 1, size, f);
    fclose(f);
    
    buffer[read_bytes] = '\0';
    return buffer;
}

// Delete the state file after successful restore
// Returns true on success
bool __desi_delete_state(void) {
    const char* path = __desi_state_file();
    if (path == NULL) return false;
    return remove(path) == 0;
}


