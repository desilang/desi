#!/bin/sh
# Desi vs C benchmark runner (macOS / Linux).
#
# For each benchmark pair (<name>.desi / <name>.c):
#   - builds the Desi program via build-desi.sh
#   - builds the C program via clang at the SAME opt level for parity
#     (default -O0; --release compiles both sides at -O2)
#   - runs each 3 times via /usr/bin/time, reports best wall time and
#     maximum resident set size
#
# Usage: ./benchmarks/run_benchmarks.sh [--release]
set -e

RELEASE=0
if [ "$1" = "--release" ] || [ "$1" = "-r" ]; then
    RELEASE=1
fi

BENCH_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$BENCH_DIR")"
OUT_DIR="$BENCH_DIR/out"
mkdir -p "$OUT_DIR"

if [ "$RELEASE" = "1" ]; then
    C_OPT="-O2"
    echo "Mode: RELEASE (-O2 both sides)"
else
    C_OPT="-O0"
    echo "Mode: default (-O0 both sides)"
fi

# max RSS: macOS `time -l` reports bytes, Linux `time -v` reports kbytes.
OS="$(uname -s)"

run_timed() {
    exe="$1"
    best_ms=999999999
    peak_kb=0
    out=""
    for _ in 1 2 3; do
        if [ "$OS" = "Darwin" ]; then
            start=$(python3 -c 'import time; print(int(time.time()*1000))')
            out=$("$exe")
            end=$(python3 -c 'import time; print(int(time.time()*1000))')
            ms=$((end - start))
            rss_bytes=$(/usr/bin/time -l "$exe" 2>&1 >/dev/null | awk '/maximum resident set size/{print $1}')
            kb=$((rss_bytes / 1024))
        else
            start=$(date +%s%3N)
            out=$("$exe")
            end=$(date +%s%3N)
            ms=$((end - start))
            kb=$(/usr/bin/time -v "$exe" 2>&1 >/dev/null | awk -F': ' '/Maximum resident set size/{print $2}')
        fi
        [ "$ms" -lt "$best_ms" ] && best_ms=$ms
        [ "$kb" -gt "$peak_kb" ] && peak_kb=$kb
    done
    echo "$best_ms $peak_kb $out"
}

printf "%-14s %10s %10s %10s %10s   %s\n" "Benchmark" "Desi ms" "C ms" "Desi MB" "C MB" "Output"

for src in "$BENCH_DIR"/*.desi; do
    name=$(basename "$src" .desi)

    # Build, and say which benchmark failed and why when it does. The output
    # used to be discarded, so `set -e` aborted the run with nothing but the
    # step's exit code — a compiler bug that only this benchmark's shape
    # triggered showed up in CI as an unexplained "exit code 1". Removing the
    # executable first also stops a stale one from being timed as if it were
    # freshly built.
    desi_exe="$REPO_ROOT/build/output/$name"
    rm -f "$desi_exe"

    if [ "$RELEASE" = "1" ]; then
        build_log=$("$REPO_ROOT/build-desi.sh" --release "$src" 2>&1) || true
    else
        build_log=$("$REPO_ROOT/build-desi.sh" "$src" 2>&1) || true
    fi
    if [ ! -x "$desi_exe" ]; then
        echo "FAILED to build $name.desi:" >&2
        echo "$build_log" >&2
        exit 1
    fi

    c_exe="$OUT_DIR/${name}_c"
    clang $C_OPT "$BENCH_DIR/$name.c" -o "$c_exe"

    set -- $(run_timed "$desi_exe")
    d_ms=$1; d_kb=$2; d_out=$3
    set -- $(run_timed "$c_exe")
    c_ms=$1; c_kb=$2; c_out=$3

    if [ "$d_out" != "$c_out" ]; then
        echo "WARN: output mismatch for $name: desi=$d_out c=$c_out" >&2
    fi

    d_mb=$(awk "BEGIN{printf \"%.1f\", $d_kb/1024}")
    c_mb=$(awk "BEGIN{printf \"%.1f\", $c_kb/1024}")
    printf "%-14s %10s %10s %10s %10s   %s\n" "$name" "$d_ms" "$c_ms" "$d_mb" "$c_mb" "$d_out"
done
