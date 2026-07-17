/* Float benchmark: 80x80 matrix multiply. Fills both matrices from the
 * same rolling 0..9 seed in the same order as matrix_mul.desi (IEEE 754
 * makes the checksum bit-identical). C uses flat double arrays vs Desi's
 * boxed list elements — that representation difference is part of what
 * the benchmark measures. */
#include <stdio.h>
#include <stdlib.h>

#define N 80

int main(void) {
    double* a = malloc(N * N * sizeof(double));
    double* b = malloc(N * N * sizeof(double));
    double seed = 0.0;
    for (int i = 0; i < N; i++) {
        for (int j = 0; j < N; j++) {
            a[i * N + j] = seed;
            seed += 1.0;
            if (seed >= 10.0) seed = 0.0;
            b[i * N + j] = seed;
            seed += 1.0;
            if (seed >= 10.0) seed = 0.0;
        }
    }

    double total = 0.0;
    for (int r = 0; r < N; r++) {
        for (int c = 0; c < N; c++) {
            double acc = 0.0;
            for (int k = 0; k < N; k++) {
                acc += a[r * N + k] * b[k * N + c];
            }
            total += acc;
        }
    }

    /* %g matches Desi's float print format ("1.024e+07") exactly */
    printf("%g\n", total);
    free(a);
    free(b);
    return 0;
}
