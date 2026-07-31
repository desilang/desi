/*
 * Desi Runtime Range Implementation
 *
 * A range holds only start/stop/step; elements are computed on demand.
 */

#include "range.h"
#include <stdlib.h>
#include <stdio.h>

DesiRange* range_new(int64_t start, int64_t stop, int64_t step) {
    DesiRange* r = malloc(sizeof(DesiRange));
    if (!r) return NULL;
    r->start = start;
    r->stop = stop;
    /* A zero step would make the sequence infinite; clamp it rather than hang. */
    r->step = (step == 0) ? 1 : step;
    return r;
}

int64_t range_len(DesiRange* r) {
    if (!r) return 0;

    int64_t span;
    int64_t step = r->step;

    if (step > 0) {
        if (r->stop <= r->start) return 0;
        span = r->stop - r->start;
    } else {
        if (r->stop >= r->start) return 0;
        span = r->start - r->stop;
        step = -step;
    }

    /* Round up: the final element is included when the span is not a multiple. */
    return (span + step - 1) / step;
}

int64_t range_get(DesiRange* r, int64_t idx) {
    if (!r) {
        fprintf(stderr, "range_get: null range\n");
        return 0;
    }

    int64_t n = range_len(r);

    /* Negative indices count back from the end, as they do for lists. */
    if (idx < 0) idx = n + idx;

    if (idx < 0 || idx >= n) {
        fprintf(stderr, "range_get: index %lld out of bounds (len=%lld)\n",
                (long long)idx, (long long)n);
        return 0;
    }

    return r->start + idx * r->step;
}

bool range_contains(DesiRange* r, int64_t v) {
    if (!r) return false;

    /* Inside the half-open interval, in the direction the step travels? */
    if (r->step > 0) {
        if (v < r->start || v >= r->stop) return false;
    } else {
        if (v > r->start || v <= r->stop) return false;
    }

    /* And landed on exactly, rather than stepped over. */
    return ((v - r->start) % r->step) == 0;
}

void range_free(DesiRange* r) {
    free(r);
}
