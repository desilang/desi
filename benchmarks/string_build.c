/* String accumulator benchmark: append one char 10k times using
 * amortized buffer growth (the idiomatic C approach). Mirrors
 * string_build.desi, which tracks the compiler's known accumulator
 * reassignment gap (see the .desi header). */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(void) {
    size_t cap = 16, len = 0;
    char* s = malloc(cap);
    s[0] = '\0';
    for (int i = 0; i < 10000; i++) {
        if (len + 2 > cap) {
            cap *= 2;
            s = realloc(s, cap);
        }
        s[len++] = 'x';
        s[len] = '\0';
    }
    printf("%zu\n", strlen(s));
    free(s);
    return 0;
}
