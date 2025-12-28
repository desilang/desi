/**
 * Desi Reference Counting Runtime
 * 
 * Rc object layout: [refcount: i32][inner: ptr]
 * - refcount at offset 0 (4 bytes)
 * - inner pointer at offset 4 (8 bytes on 64-bit)
 */

#include <stdlib.h>
#include <stdint.h>

// Rc object header - stored before the inner value
typedef struct {
    int32_t refcount;
    void* inner;
} DesiRc;

/**
 * Create a new Rc wrapping the given inner pointer.
 * Initial refcount = 1.
 */
void* __rc_new(void* inner) {
    DesiRc* rc = (DesiRc*)malloc(sizeof(DesiRc));
    if (rc == NULL) return NULL;
    rc->refcount = 1;
    rc->inner = inner;
    return (void*)rc;
}

/**
 * Increment the reference count.
 */
void __rc_inc(void* rc_ptr) {
    if (rc_ptr == NULL) return;
    DesiRc* rc = (DesiRc*)rc_ptr;
    rc->refcount++;
}

/**
 * Decrement the reference count.
 * If refcount reaches 0, free the inner value and the Rc object.
 */
void __rc_dec(void* rc_ptr) {
    if (rc_ptr == NULL) return;
    DesiRc* rc = (DesiRc*)rc_ptr;
    rc->refcount--;
    if (rc->refcount <= 0) {
        // Free the inner value if it was heap-allocated
        if (rc->inner != NULL) {
            free(rc->inner);
        }
        // Free the Rc wrapper itself
        free(rc);
    }
}

/**
 * Clone an Rc - increments refcount and returns the same pointer.
 * This is how multiple owners share the same value.
 */
void* __rc_clone(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    __rc_inc(rc_ptr);
    return rc_ptr;
}

/**
 * Get the inner pointer from an Rc.
 */
void* __rc_get(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    DesiRc* rc = (DesiRc*)rc_ptr;
    return rc->inner;
}

/**
 * Get the current reference count (for debugging/testing).
 */
int32_t __rc_count(void* rc_ptr) {
    if (rc_ptr == NULL) return 0;
    DesiRc* rc = (DesiRc*)rc_ptr;
    return rc->refcount;
}
