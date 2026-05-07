# Desi Compile-Time Benchmarks

> **Machine:** Apple Silicon (macOS 26.0)
> **Date:** 2026-05-07
> **Compiler:** desic 0.1.0-dev

## Type Check Only (`desic -check`)

| Program | Lines | Time |
|---------|-------|------|
| `38_while_loop.desi` | 14 | 6ms |
| `313_turbofish_edge_cases.desi` | 168 | 6ms |
| `407_http_client.desi` | 371 (multi-import) | 8ms |

## Full Compile + Link (`build-desi.sh`)

Includes: parse → check → LLVM IR → clang → link against libdesi.a

| Program | Lines | Imports | Time |
|---------|-------|---------|------|
| `38_while_loop.desi` | 14 | 0 | 126ms |
| `313_turbofish_edge_cases.desi` | 168 | 0 | 141ms |
| `407_http_client.desi` | 371 | http, json | 169ms |

## Notes

- Type checking is consistently sub-10ms regardless of program size (up to 371 lines)
- Full compile+link is dominated by clang/LLVM (~120ms baseline)
- Multi-import programs (http, json, etc.) add ~30ms for additional linking
- All measurements are cold-cache, single run
