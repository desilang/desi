/* CPU benchmark: tight integer arithmetic loop (100M iterations).
 * Mirrors loop_sum.desi — both sides compile at clang -O0. */
#include <stdio.h>

int main(void) {
    int acc = 0;
    for (int i = 0; i < 100000000; i++) {
        acc += i;
    }
    printf("%d\n", acc);
    return 0;
}
