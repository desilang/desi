/*
 * Desi Runtime Range Header
 *
 * range(start, stop, step) is a lazy arithmetic sequence of ints. It is not a
 * materialised list: a range of a billion elements costs three integers.
 *
 * A `for` over a literal range(...) call is lowered to a plain index loop and
 * never touches this file. These entry points exist for the first-class uses —
 * binding a range to a variable, passing one to a function, len(), indexing.
 */

#ifndef DESI_RANGE_H
#define DESI_RANGE_H

#include <stdint.h>
#include <stdbool.h>

typedef struct DesiRange {
    int64_t start;
    int64_t stop;
    int64_t step;
} DesiRange;

/* Allocate a range. A step of 0 is clamped to 1 so it cannot hang a loop. */
DesiRange* range_new(int64_t start, int64_t stop, int64_t step);

/* Number of elements produced, never negative. */
int64_t range_len(DesiRange* r);

/*
 * Element at idx, counted from the start, with negative indices counting back
 * from the end as elsewhere in Desi. Out-of-bounds traps rather than returning
 * a wrong number silently.
 */
int64_t range_get(DesiRange* r, int64_t idx);

/* True when v is one of the values the range produces. */
bool range_contains(DesiRange* r, int64_t v);

void range_free(DesiRange* r);

#endif /* DESI_RANGE_H */
