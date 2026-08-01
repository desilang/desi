# Launch post — draft

Angle: the testing-discipline story. Not "look how many tests I have" —
"here is what my tests missed, and how I found it." Specifics are the whole
point; every number and every bug below is real and traceable to a commit.

Target: r/ProgrammingLanguages, HN. Voice: first person, solo author.
Trim freely — the individual stories stand alone.

---

## Desi 0.1.0 — a compiled language with a Python-shaped syntax, and what its test suite failed to catch

I've been building a language called **Desi**. It looks like Python, compiles
through LLVM to a native binary, has no garbage collector, moves values instead
of copying them, and ships with a standard library that includes an ORM and
Go-style concurrency. `desic run hello.desi` and you get an executable.

That paragraph is the easy part to write and the least interesting thing about
the project. Here's the part I'd actually want to read.

### The thing people are going to say

A new language from one person, written fast, in the current era, gets one
review before anyone opens the code: *this was vibe-coded, it'll fall over the
moment you push on it.*

It's a fair prior. So rather than argue, let me show you the last day of work.

### 518 tests passed while a program couldn't compile at all

Desi's suite is 518 end-to-end example programs. Each one compiles, runs, and
has its output compared against a declared expectation, at **two optimization
levels**, on **Windows, Linux and macOS**. That's the gate.

Yesterday all 518 passed on every platform. A benchmark in the same repo —
`binary_tree` — could not be compiled at all. The compiler was emitting invalid
LLVM IR for it and had been for eight days.

The bug: I'd given loops an exit block so `break` had somewhere to jump. The
backend attached a branch's jump-to-merge to the block a branch *starts* in,
and those stop being the same block the moment a branch *ends* with a loop.
The exit block came out empty with no terminator, and LLVM rejected the whole
function.

Not one of 518 programs had a branch whose last statement is a loop. The
benchmark did, and benchmarks aren't built by the example harness.

**A test suite tells you about the shapes you thought of.**

### The benchmark had been reporting a pass for eight days

Which raises the obvious question: why didn't the benchmark job go red?

On Linux it did — as `Error: Process completed with exit code 1`, with no
benchmark name and no reason, because the runner discarded build output. On
Windows it didn't go red at all. The script piped the build to `Out-Null` and
never checked that anything came out of it, so a failed build left the previous
executable sitting in `build/output/` and the benchmark was timed against a
binary from eight days earlier and printed as a passing row.

I read that row myself, twice, and reported it as a pass.

Both runners now delete the executable first, treat its absence as fatal, and
print the build output. The Linux one names the benchmark.

### Two examples printed uninitialized memory for months

While auditing which examples actually assert something, I found 66 that only
check "did it exit zero". Nineteen of those print output nobody compares. Two
were printing garbage:

```
Got value: 672134932
```

That's `MyOption.Some(42)`, matched back out. Both files *documented* the bug,
in a trailing comment:

```
# SKIPPED: Non-deterministic output - prints uninitialized memory value
```

The harness only honours `# EXPECTED: SKIP`. `# SKIPPED:` means nothing to it.
So both ran, every time, on every platform, asserting nothing, printing
garbage, and counted toward the passing total.

The bug underneath was real. A generic variant's constructor is emitted once
for every `T`, so it can't take the payload by value — the caller boxes the
argument and passes the box's address. The constructor stored *that address* as
the payload, and matching a variant dereferences the payload slot exactly once,
so the binding came back as the low half of a stack address.

It only showed with a primitive. A `str` payload was never boxed — a str is
already a pointer — so it arrived as the value and was stored correctly. The
inconsistency was the defect: one constructor can't store both a value and a
pointer to one.

The same accident — *"a pointer looked right for the wrong reason"* — had
already hidden a bug in taskgroup captures, where a captured `int` arrived as
the address of its box and a captured `str` looked fine.

### A test that passed on the wrong error entirely

`230_c_global_privacy` was written to prove that importing a non-`pub` constant
fails. It expected a compile error, and it got one — from the *parser*, because
it imported a helper whose filename starts with a digit and can't be a module
path. The visibility rule it existed to check was never reached.

### A test that passed on two platforms out of three, by luck

An example asserted that three concurrent tasks print their values in the order
they were spawned. Concurrency promises no such thing. It passed on Windows and
macOS, and failed on Ubuntu, which interleaved them differently.

Chasing that turned up something worse. Stressing every concurrency example
sixty times found this:

```
WorkerWorker  12 working working
```

Two tasks tearing *inside* a single `print`. A three-argument `print` lowered to
six separate output calls; each was atomic, the sequence wasn't.

The obvious fix — wrap the sequence in a lock — is wrong, and I nearly shipped
it. Arguments are evaluated *between* those output calls, so the lock would be
held across arbitrary user code, and `print(ch.recv())` would deadlock every
other task's print. Trading garbled output for a hang is not a fix. The lowerer
now evaluates every argument into a temp first, *then* takes the lock. One print
is one whole line, and `print("a", f(), "b")` no longer prints `a ` before
running `f`.

### So what is the discipline, then

Not the test count. The count is what let all of the above hide.

- Every example runs at **-O0 and -O2+LTO**. Optimization-dependent bugs are
  their own category and a single-level suite never sees them.
- Every example runs on **three platforms**. Two of the bugs above were found
  only because Ubuntu scheduled threads differently than Windows.
- Every benchmark must produce **the same output as a C program doing the same
  work**. That is what should have caught `binary_tree`, and will now.
- The documentation is **compiled, not proofread**. Every ` ```desi ` block in
  the book is extracted and parsed; parse failures are held at zero, down from
  124. Six pages had Desi fenced as ` ```python `, which hid 38 blocks —
  including the entire `match` reference.
- Every shipped binary has a **smoke test**. `desic`, `desifmt`, `desirepl`,
  `desilsp`, `desic watch`. A compiler that works while its formatter corrupts
  your file is not a working toolchain — and the formatter *did* corrupt
  f-strings, once.
- **An example that asserts nothing is not a test.** The harness now says so.

The rule I'd write on the wall: **a passing suite is evidence about the cases
you imagined.** Everything above was found by going looking somewhere the suite
wasn't.

### What doesn't work

There's a [Known Limitations](https://desilang.org/reference/known-limitations/)
page, written by me rather than discovered by you. No `loop:` construct. No
`not in`. No line continuations. A match arm is a single expression. Generic
constructors need turbofish. A constant can't be initialized by a function call.
The database drivers have no TLS on Windows. It also lists the environment
problems that look like Desi bugs but aren't, so nobody files those.

0.1.0 is a first release. I'd rather you find the next `binary_tree` than not
look.

- Repo: https://github.com/desilang/desi
- Docs: https://desilang.org

---

## Notes before posting

- Numbers to re-check at tag time: example count (518), diagnostic codes (152
  across 17 categories), doc blocks (916 / 744 fragments / 0 parse failures),
  benchmark count (10).
- Link the specific commits for `binary_tree`, the generic enum payload, and
  print atomicity — people will want to read the diffs, and they hold up.
- Don't claim performance in the post. The benchmark README has the numbers and
  the remaining deltas; let anyone who cares go read it rather than pulling one
  favourable line into a launch thread.
