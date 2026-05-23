/*
 * perf.c — Benchmark timing engine for Desi @perf decorator
 *
 * Provides high-resolution timing for @perf-decorated functions.
 * Uses CLOCK_MONOTONIC on Linux and mach_absolute_time() on macOS.
 *
 * Public API:
 *   __perf_bench_start(name)              → Record start timestamp
 *   __perf_bench_finish(name, iterations) → Record end, compute stats, print
 *   __perf_bench_reset()                  → Clear all stored entries
 *
 * Output format:
 *   bench  name  avg=1.234ms  min=1.100ms  max=1.400ms  iters=1000
 *
 * Stripped in release builds via MacroProtocol.StripInRelease = true.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#ifdef __APPLE__
#include <mach/mach_time.h>
static uint64_t perf_nanos(void) {
    static mach_timebase_info_data_t tbi = {0, 0};
    if (tbi.denom == 0) mach_timebase_info(&tbi);
    return mach_absolute_time() * tbi.numer / tbi.denom;
}
#elif defined(_WIN32)
#include <windows.h>
static uint64_t perf_nanos(void) {
    LARGE_INTEGER freq, cnt;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&cnt);
    return (uint64_t)(cnt.QuadPart * 1000000000LL / freq.QuadPart);
}
#else
#include <time.h>
static uint64_t perf_nanos(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000000ULL + (uint64_t)ts.tv_nsec;
}
#endif

/* ---------- Internal tracking ---------- */

#define PERF_MAX_ENTRIES 256

typedef struct {
    const char *name;
    uint64_t    start_ns;
    uint64_t    min_ns;
    uint64_t    max_ns;
    uint64_t    total_ns;
    int         runs;
} PerfEntry;

static PerfEntry perf_entries[PERF_MAX_ENTRIES];
static int perf_count = 0;

static PerfEntry* perf_find_or_create(const char *name) {
    for (int i = 0; i < perf_count; i++) {
        if (strcmp(perf_entries[i].name, name) == 0)
            return &perf_entries[i];
    }
    if (perf_count >= PERF_MAX_ENTRIES) {
        fprintf(stderr, "[perf] warning: max entries exceeded, ignoring '%s'\n", name);
        return NULL;
    }
    PerfEntry *e = &perf_entries[perf_count++];
    e->name     = name;
    e->start_ns = 0;
    e->min_ns   = UINT64_MAX;
    e->max_ns   = 0;
    e->total_ns = 0;
    e->runs     = 0;
    return e;
}

/* ---------- Formatting helpers ---------- */

static void format_duration(uint64_t ns, char *buf, int bufsize) {
    if (ns >= 1000000000ULL) {
        snprintf(buf, bufsize, "%.3fs", (double)ns / 1e9);
    } else if (ns >= 1000000ULL) {
        snprintf(buf, bufsize, "%.3fms", (double)ns / 1e6);
    } else if (ns >= 1000ULL) {
        snprintf(buf, bufsize, "%.3fμs", (double)ns / 1e3);
    } else {
        snprintf(buf, bufsize, "%lluns", (unsigned long long)ns);
    }
}

/* ---------- Public API ---------- */

void __perf_bench_start(const char *name) {
    PerfEntry *e = perf_find_or_create(name);
    if (!e) return;
    e->start_ns = perf_nanos();
}

void __perf_bench_finish(const char *name, int iterations) {
    uint64_t end_ns = perf_nanos();
    PerfEntry *e = perf_find_or_create(name);
    if (!e || e->start_ns == 0) return;

    uint64_t elapsed = end_ns - e->start_ns;
    uint64_t per_iter = (iterations > 0) ? elapsed / (uint64_t)iterations : elapsed;

    if (per_iter < e->min_ns) e->min_ns = per_iter;
    if (per_iter > e->max_ns) e->max_ns = per_iter;
    e->total_ns += elapsed;
    e->runs++;

    /* Print immediate result */
    char avg_buf[32], min_buf[32], max_buf[32];
    uint64_t avg_ns = e->total_ns / (uint64_t)e->runs;
    if (iterations > 0) avg_ns = avg_ns / (uint64_t)iterations;

    format_duration(avg_ns, avg_buf, sizeof(avg_buf));
    format_duration(e->min_ns, min_buf, sizeof(min_buf));
    format_duration(e->max_ns, max_buf, sizeof(max_buf));

    int iters = (iterations > 0) ? iterations : 1;
    fprintf(stdout, "bench  %-32s  avg=%s  min=%s  max=%s  iters=%d\n",
            name, avg_buf, min_buf, max_buf, iters);
    fflush(stdout);

    e->start_ns = 0;
}

void __perf_bench_reset(void) {
    perf_count = 0;
    memset(perf_entries, 0, sizeof(perf_entries));
}
