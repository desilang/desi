/* Allocator benchmark: 500k short-lived 3-element arrays, freed each
 * iteration. Mirrors alloc_churn.desi (list literal + scope-exit drop). */
#include <stdio.h>
#include <stdlib.h>

int main(void) {
    int total = 0;
    for (int i = 0; i < 500000; i++) {
        int* t = malloc(3 * sizeof(int));
        t[0] = i % 100;
        t[1] = i % 50;
        t[2] = i % 10;
        total += t[0] + t[1] + t[2];
        free(t);
    }
    printf("%d\n", total);
    return 0;
}
