/* Hash-map benchmark: 100k int-keyed inserts, then 100k lookups.
 * Open-addressing table with linear probing — a typical hand-rolled C
 * map, mirroring dict_ops.desi's built-in dict. */
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>

#define CAP (1 << 18) /* 262144 slots for 100k entries */

typedef struct {
    int64_t key;
    int val;
    int used;
} Slot;

static uint64_t hash_key(int64_t k) {
    uint64_t h = 14695981039346656037ULL;
    h ^= (uint64_t)k;
    h *= 1099511628211ULL;
    return h;
}

int main(void) {
    Slot* table = calloc(CAP, sizeof(Slot));

    for (int64_t i = 0; i < 100000; i++) {
        uint64_t idx = hash_key(i) & (CAP - 1);
        while (table[idx].used && table[idx].key != i) {
            idx = (idx + 1) & (CAP - 1);
        }
        table[idx].key = i;
        table[idx].val = (int)(i % 100);
        table[idx].used = 1;
    }

    int total = 0;
    for (int64_t j = 0; j < 100000; j++) {
        uint64_t idx = hash_key(j) & (CAP - 1);
        while (table[idx].used && table[idx].key != j) {
            idx = (idx + 1) & (CAP - 1);
        }
        total += table[idx].val;
    }
    printf("%d\n", total);
    free(table);
    return 0;
}
