# FFI (Foreign Function Interface)

Stage-0 uses a C-emitter backend, so FFI will target the **C ABI**. The feature is planned but only partially implemented during Stage-0; the stable surface will firm up in Stage-1.

## Goals
- Call C functions from Desi with minimal friction.
- Pass/return primitive types reliably.
- Link system libraries or static libs at build time.
- Keep layouts/ABI explicit and conservative.

## Non-goals (Stage-0)
- No callbacks/function pointers from C into Desi yet.
- No variadic functions.
- No guaranteed stable struct/enum layout until `repr(C)` is finalized.

## Syntax (proposed)
```desi
extern "C":
  def puts(s: *u8) -> i32
  def qsort(base: *void, n: u64, size: u64, cmp: *void) -> void  # callbacks later

def main() -> i32:
  puts("hello\0".as_ptr())
  0
```

* `extern "C":` block declares functions with C calling convention.
* Raw pointers are written as `*T`. Using them is `unsafe` territory; Stage-0 treats deref/index on raw pointers as a compile error unless guarded by std helpers (to be added later).

## Linking

During Stage-0, the compiler shells out to the platform C toolchain and you can pass extra linker flags:

```
desic build hello.desi --cc=clang --ldflags="-lm"
```

An attribute form is reserved for Stage-1:

```desi
@link(name: "m")
extern "C":
  def cos(x: f64) -> f64
```

## Types allowed (Stage-0)

* Primitives: `i32, i64, u32, u64, u8, f64, bool` (mapped to standard C widths).
* Pointers: `*T` (opaque to the type checker beyond size/addr).
* `void` return.

Strings and `Vec[T]` are **not** passed directly; use pointers/length pairs exposed via std helpers (added later).

## Layout & `repr(C)` (reserved)

* A future `repr(C)` will pin struct/enum layout for FFI.
* Until then, do not pass Desi `struct`/`enum` across the boundary.

## Safety

* FFI calls are considered `unsafe` operations; Stage-0 permits them but we will gate behind an `unsafe` marker in Stage-1.
* The runtime does not manage C-owned memory; freeing/mutating across the boundary is the caller’s responsibility.

## C emitter mapping (informative)

* `extern "C"` decls generate C prototypes and are referenced directly from the emitted translation unit.
* On Windows, we’ll respect the default C calling convention; attributes for stdcall/fastcall may arrive later.
