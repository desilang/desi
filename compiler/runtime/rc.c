/**
 * Desi Reference Counting Runtime
 * 
 * Rc/Weak object layout:
 * - refcount at offset 0 (4 bytes)
 * - weakcount at offset 4 (4 bytes)
 * - inner pointer at offset 8 (8 bytes on 64-bit)
 */

#include <stdlib.h>
#include <stdint.h>

// Rc object header - stored before the inner value
typedef struct {
    int32_t refcount;
    int32_t weakcount;
    void* inner;
} DesiRc;

/**
 * Create a new Rc wrapping the given inner pointer.
 * Initial refcount = 1, weakcount = 0.
 */
void* __rc_new(void* inner) {
    DesiRc* rc = (DesiRc*)malloc(sizeof(DesiRc));
    if (rc == NULL) return NULL;
    rc->refcount = 1;
    rc->weakcount = 0;
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
 * If refcount reaches 0, free the inner value.
 * If both refcount and weakcount reach 0, free the Rc object.
 */
void __rc_dec(void* rc_ptr) {
    if (rc_ptr == NULL) return;
    DesiRc* rc = (DesiRc*)rc_ptr;
    rc->refcount--;
    if (rc->refcount <= 0) {
        // Free the inner value if it was heap-allocated
        if (rc->inner != NULL) {
            free(rc->inner);
            rc->inner = NULL;
        }
        // Only free the header if no weak pointers remain
        if (rc->weakcount <= 0) {
            free(rc);
        }
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

// === Weak Pointer Support ===

/**
 * Create a new Weak pointer from an Rc pointer.
 * Increments the weakcount and returns the same control block pointer.
 */
void* __weak_new(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    DesiRc* rc = (DesiRc*)rc_ptr;
    rc->weakcount++;
    return rc_ptr;
}

/**
 * Decrement the weak reference count.
 * If both refcount and weakcount reach 0, free the Rc control block.
 */
void __weak_dec(void* rc_ptr) {
    if (rc_ptr == NULL) return;
    DesiRc* rc = (DesiRc*)rc_ptr;
    rc->weakcount--;
    if (rc->weakcount <= 0 && rc->refcount <= 0) {
        free(rc);
    }
}

/**
 * Try to upgrade a Weak reference to a strong Rc.
 * Returns the rc_ptr if refcount > 0 (and increments refcount).
 * Returns NULL if the referenced object has already been deallocated.
 */
void* __weak_upgrade(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    DesiRc* rc = (DesiRc*)rc_ptr;
    if (rc->refcount > 0) {
        rc->refcount++;
        return rc_ptr;
    }
    return NULL;
}

/**
 * Get the current weak reference count (for debugging/testing).
 */
int32_t __weak_count(void* rc_ptr) {
    if (rc_ptr == NULL) return 0;
    DesiRc* rc = (DesiRc*)rc_ptr;
    return rc->weakcount;
}
