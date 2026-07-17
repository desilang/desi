/* Function-call benchmark: naive recursive fibonacci(32) — ~5.5M calls.
 * Measures pure calling-convention overhead. Mirrors fib_recursive.desi. */
#include <stdio.h>

static int fib(int n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}

int main(void) {
    printf("%d\n", fib(32));
    return 0;
}
