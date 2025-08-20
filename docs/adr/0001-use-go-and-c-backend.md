# ADR 0001: Use Go front-end and a C emitter for Stage-0
Date: 2025-08-20
Status: Accepted

## Context
We need a portable, easily packaged bootstrap compiler to start the project and attract contributors on Windows/macOS/Linux. The front-end must be fast, statically typed, and simple to distribute. The backend should produce native executables quickly without committing us to a long-term codegen strategy.

## Decision
- Implement the Stage-0 compiler front-end in **Go**.
- Emit **portable C** from our typed IR and invoke the system C toolchain (clang/msvc) to link a **small C runtime** (ARC, strings, vec, channels).
- Plan to swap the backend to **LLVM/Cranelift** in Stage-2 without changing the front-end.

## Consequences
**Pros**
- One static Go binary per platform; trivial contributor onboarding.
- Fast builds, straightforward concurrency for compiler passes.
- C emitter provides immediate portability and debuggability (`#line` maps, UBSan/ASan friendly).

**Cons**
- C is a lowest-common-denominator target (SIMD/intrinsics are clumsy).
- Debug info maps through generated C until LLVM is adopted.
- Linking depends on system toolchains.

## Alternatives Considered
- **Python front-end**: great for prototyping; slower, packaging and true parallelism harder.
- **Rust front-end**: excellent long-term; slower initial iteration and steeper contributor ramp-up.
- **Direct LLVM now**: higher upfront cost; slower to first runnable compiler.

## Rollout
1. Stage-0: Go + C emitter + C runtime, minimal std.
2. Stage-1: Self-host subset in Desi.
3. Stage-2: Replace C emitter with LLVM/Cranelift; improve optimizer & debuginfo.

## Notes
- ARC chosen over tracing GC for predictable latencies (see ADR-0002 when added).
- The C runtime is small and licensed under Apache-2.0 with SPDX headers.
