#include <stdio.h>
#include <stdint.h>
#include <stdbool.h> // Added for 'bool' type

// Simple print function for integers
void print_int(int64_t n) {
    printf("%lld\n", n);
}

const char* bool_to_cstring(bool b) {
    return b ? "true" : "false";
}

// Simple print function for strings (wrapper around puts)
void print_str(const char* s) {
    puts(s);
}
