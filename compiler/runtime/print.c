#include <stdio.h>
#include <stdint.h>

// Simple print function for integers
void print_int(int64_t value) {
    printf("%lld\n", (long long)value);
}

// Simple print function for strings (wrapper around puts)
void print_str(const char* s) {
    puts(s);
}
