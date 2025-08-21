#include "../include/desi_rt.h"

// Stage-0 stubs so the runtime links. We'll implement ARC shortly.
void* desi_retain(void* p) { return p; }
void  desi_release(void* p) { (void)p; }
