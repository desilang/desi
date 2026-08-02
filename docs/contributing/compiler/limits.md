# Stack Guard Implementation

**Scope:** Preventing stack exhaustion from turning into a fault.

---

## What it protects

Unbounded recursion writes past the end of the stack. On Windows that is an
access violation, on Linux a SIGSEGV — a memory-safety failure, and one the
program cannot report on, because the fault happens *while* it is happening.

The guard turns that into an ordinary Desi `RuntimeError`, raised before the
stack runs out, so a program can report it and a supervisor can survive it.

---

## Why headroom, not a frame count

The guard used to count frames: every user function incremented a thread-local
on entry and decremented it on return, panicking past 1000. That was replaced,
for two reasons.

**It was the wrong measurement.** A frame count is a proxy for stack space, and
a poor one when frames differ in size. The proxy fails in both directions — a
function with large frames exhausts the stack in far fewer than 1000 calls, and
`set_recursion_limit(10000000)` instructed the old guard to keep counting long
past the point where the stack was gone. That second case is example 553: a
counter cannot catch it, because it is doing exactly what it was told.

**It was expensive.** Measured on `fib(32)` at `clang -O2`:

| design | guard cost |
|---|---|
| out-of-line counter (as emitted) | 16.8 ms |
| the same counter, inlined | 13.8 ms |
| inlined, plain global instead of thread-local | 16.2 ms |
| **stack headroom check** | **2.5 ms** |

Inlining barely helped, and a plain global was no faster than the thread-local,
so neither the call nor the TLS was the cost — the paired increment and
decrement was. A headroom check is stateless: nothing is changed on the way in,
so nothing has to be undone on the way out, and **the exit hook disappears
entirely**. That is most of the difference.

---

## Runtime (`compiler/runtime/limits.c`)

A thread-local floor: the lowest stack address the thread may touch.

```c
#define DESI_FLOOR_UNINIT   ((char*)UINTPTR_MAX)
#define DESI_FLOOR_DISABLED ((char*)0)

DESI_THREAD_LOCAL char* __desi_stack_floor = DESI_FLOOR_UNINIT;
```

The two sentinels exist so the emitted check needs exactly one compare. A thread
starts at the highest address there is, so its first guarded call compares below
it and takes the cold path — which is what computes the real floor. A platform
that will not report its stack bounds gets the lowest address, and the check
then never fires. There is no separate "is this set up yet" test.

Bounds come from `GetCurrentThreadStackLimits` on Windows,
`pthread_get_stackaddr_np` on macOS, and `pthread_getattr_np` elsewhere. Where a
platform will not say, the guard stands down rather than inventing a floor: a
wrong floor either panics healthy programs or never fires at all.

The floor is computed on demand rather than at thread entry. Threads are created
in four runtime files — futures, the scheduler, the supervisor pool, the
websocket pinger — and an embedding host can make one too. One missed entry
point would leave a thread silently unguarded; a lazy first call cannot.

### The limit is a ceiling, never a licence

```
floor = max(current_sp - limit * FRAME_ESTIMATE,   /* what was asked for */
            stack_low + RESERVE)                   /* what actually exists */
```

`set_recursion_limit(n)` still raises how deep a program may go, and still
cannot authorise going deeper than the stack allows. This is the property that
makes the guard a safety mechanism rather than a convention.

`RESERVE` (64 KB) covers two things. The panic path needs room to run, on the
stack that just ran out. And the check reads the address of the *first* alloca
in the frame — the frame's high end — so a function can dip that far past the
floor after being waved through. Desi frames are small by construction (locals
of any size live on the heap; allocas are box and result slots of a few words,
all hoisted into one entry block), so 64 KB is well beyond what that shape
reaches.

A thread whose stack cannot hold twice the reserve is left unguarded rather than
given a floor above its own ceiling.

---

## Codegen (`emit_func.go`)

Emitted inline at the top of every user function except `main` and `__top__`:

```llvm
entry:
  %__gprobe0 = alloca i8
  %__gfloor0 = load ptr, ptr @__desi_stack_floor
  %__glow0 = icmp ult ptr %__gprobe0, %__gfloor0
  br i1 %__glow0, label %__gslow0, label %__gok0
__gslow0:
  call void @__desi_call_enter(ptr @.str.0)   ; cold: initialise, or raise
  br label %__gok0
__gok0:
  ...
```

The probe is an alloca, so its address is this frame's position on the stack.
`hoistAllocasToEntry` moves every alloca to the top of the entry block, which
puts it above this code — exactly where it needs to be.

Two details, each of which costs a measurable amount to get wrong:

- The global is declared `thread_local(initialexec)`. Under the default
  general-dynamic model, ELF resolves a thread-local through a call to
  `__tls_get_addr` — a call back in the prologue, undoing the inlining. Desi
  emits executables, never a dlopened library, so initial-exec is always valid.
- Emitting a call to a runtime helper instead of the compare costs about 4 ms on
  `fib(32)`. On Linux nothing inlines it away, because the Makefile builds no
  bitcode hot set the way `build.ps1` does.

There is **no exit hook**. Nothing was changed on the way in.

---

## Results

`fib_recursive`, floor-subtracted:

| | Linux | Windows |
|---|---|---|
| frame counter | 23 ms | 16.0 ms |
| headroom, as a call | 18 ms | 9.2 ms |
| headroom, inlined | 14 ms | — |
| + initial-exec TLS | **10 ms** | **6.8 ms** |
| C | 6 ms | 6.7 ms |

---

## Tail calls

A self-recursive tail call compiles to a jump, so it consumes no stack and the
guard correctly never fires — an infinite tail recursion is an infinite loop,
and Desi does not claim to detect those. This was true of the frame counter too,
which deliberately decremented before a tail call.

Example 330 exists to pin this down: its recursive call is **not** in tail
position, because a tail-position version would loop forever instead of
panicking.

---

## Testing

```bash
./test_examples.sh          # 330 limit reached, 333 limit raised,
                            # 553 limit clamped to the real stack
```

Worth re-running by hand after any change here, since none of it shows up in
ordinary output:

- recursion with fat frames still raises
- recursion on a spawned task's own thread still raises — each thread computes
  its own floor
- `set_recursion_limit` to an absurd value still raises rather than faulting

`DESI_NO_RECURSION_GUARD=1` omits the guard entirely, for measuring what it
costs. It prints a warning naming what it removes; a program built with it has
no protection against stack exhaustion.
