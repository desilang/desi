/*
 * perf.h — Benchmark timing engine declarations
 *
 * These functions are called by @perf-decorated Desi code.
 * In release builds, the calls are stripped entirely by the macro system.
 */

#ifndef DESI_PERF_H
#define DESI_PERF_H

/* Start timing for a named benchmark */
void __perf_bench_start(const char *name);

/* Finish timing, compute stats, and print results.
 * iterations: number of loop iterations (0 or 1 = single run) */
void __perf_bench_finish(const char *name, int iterations);

/* Reset all stored benchmark entries */
void __perf_bench_reset(void);

#endif /* DESI_PERF_H */
