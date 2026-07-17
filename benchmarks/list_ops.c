/* Collections benchmark: build a 1M-element growable array, sum by index,
 * free. Elements are i % 100 so the sum stays inside 32-bit range on both
 * sides. Mirrors list_ops.desi (list append + index + scope-exit drop). */
#include <stdio.h>
#include <stdlib.h>

int main(void) {
    size_t cap = 8, len = 0;
    int* data = malloc(cap * sizeof(int));
    for (int i = 0; i < 1000000; i++) {
        if (len == cap) {
            cap *= 2;
            data = realloc(data, cap * sizeof(int));
        }
        data[len++] = i % 100;
    }

    int total = 0;
    for (size_t j = 0; j < len; j++) {
        total += data[j];
    }
    printf("%d\n", total);
    free(data);
    return 0;
}
