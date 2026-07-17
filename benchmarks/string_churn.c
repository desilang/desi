/* Memory benchmark: string allocation churn (200k iterations).
 * Each iteration heap-allocates "item <n>" and frees it, mirroring the
 * transient-temporary lifecycle string_churn.desi exercises through the
 * compiler's hybrid memory management. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(void) {
    long total = 0;
    for (int i = 0; i < 200000; i++) {
        /* mirror Desi: int -> string conversion, then concatenation */
        char* num = malloc(12);
        snprintf(num, 12, "%d", i);
        size_t len = strlen("item ") + strlen(num);
        char* s = malloc(len + 1);
        snprintf(s, len + 1, "item %s", num);
        total += (long)strlen(s);
        free(num);
        free(s);
    }
    printf("%ld\n", total);
    return 0;
}
