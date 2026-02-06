// Desi program entry point
// Provides main() that initializes runtime and calls user's __top__()

#include <stdlib.h>

// Forward declarations
extern void __desi_runtime_init(void);
extern int __top__(void);

int main(int argc, char* argv[]) {
    (void)argc;
    (void)argv;
    
    // Initialize the Desi runtime (streams, scheduler, etc.)
    __desi_runtime_init();
    
    // Run the user's program
    int result = __top__();
    
    return result;
}
