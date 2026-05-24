// Desi program entry point
// Provides main() that initializes runtime and calls user's __top__()

#include <stdlib.h>
#include <stdio.h>

#ifdef _WIN32
  #include <windows.h>
  #include <io.h>
  #include <fcntl.h>
#endif

// Forward declarations
extern void __desi_runtime_init(void);
extern void __args_init(int argc, char** argv);
extern int __top__(void);

int main(int argc, char* argv[]) {
#ifdef _WIN32
    /* Always output UTF-8 — same behavior as Python/Rust on all platforms.
     * Without this, Windows uses the system code page (e.g. 850/1252),
     * which garbles non-ASCII characters and emoji. */
    SetConsoleOutputCP(CP_UTF8);
    SetConsoleCP(CP_UTF8);
    /* Also set stdio to binary mode so UTF-8 bytes aren't mangled */
    _setmode(_fileno(stdout), _O_BINARY);
    _setmode(_fileno(stderr), _O_BINARY);
#endif

    // Store CLI arguments for the args module
    __args_init(argc, argv);

    // Initialize the Desi runtime (streams, scheduler, etc.)
    __desi_runtime_init();

    // Run the user's program
    int result = __top__();

    return result;
}
