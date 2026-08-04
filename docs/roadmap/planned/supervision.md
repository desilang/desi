# Supervision

Desi's supervision is the most distinctive thing in the language — nobody else
offers OTP-style supervision in a compiled language with no garbage collector.
Elixir has the supervision but runs on a VM; Rust and Go have no supervision
trees at all.

This page states exactly what supervision does in **v0.1.0**, and what is
planned beyond it, so release material describes the real thing.

## Where v0.1.0 stands

**Desi has a supervised worker pool. It does not have a supervision tree.**

That is a real and useful thing, and it is what the README currently claims —
*"Go's concurrency — channels, `select`, structured concurrency with
supervisors"* — which is accurate and does not overreach. But measured against
the Elixir/OTP goal, the gap is the tree, not the supervisor.

## What is built

- A thread pool with a work queue: `submit(fn)` enqueues, workers dequeue.
- Persistent children with auto-restart: `start_child(fn)`.
- Two restart strategies, `SUPERVISOR_ONE_FOR_ONE` and `SUPERVISOR_ONE_FOR_ALL`.
- Restart limiting: `DEFAULT_MAX_RESTARTS` 5 within `DEFAULT_RESTART_WINDOW` 60s.
- Lifecycle and introspection: `stop`, `pool_size`, `child_count`, `is_running`.
- Stack exhaustion raises a catchable `RuntimeError` rather than aborting, so a
  child that recurses too deep is restartable rather than fatal. This is OTP
  thinking applied to memory and is genuinely better than Rust or Go here.

## Gaps, roughly in order of how much they matter

1. **No nesting, so no escalation.** A child is a function pointer, not
   another supervisor. A child can create a supervisor internally, but if that
   inner supervisor exhausts its restarts nothing propagates to the parent.
   Escalation is the property that makes OTP trees work, and it is the single
   biggest gap.

2. **`one_for_all` is unreachable from Desi — confirmed.**
   `lower_call.go:289` hardcodes the strategy argument:

   ```go
   Args: []hir.Value{
       hir.ConstInt{Text: "0"}, // ONE_FOR_ONE
       workersArg,
   },
   ```

   `sync.Supervisor(n)` selects the worker count and nothing selects the
   strategy. So `SUPERVISOR_ONE_FOR_ALL` is implemented in `supervisor.c`,
   exercised by no test, and reachable from no Desi program.

   It is worse than dead code: `compiler/lib/sync/supervisor.desi` documents
   both strategies in its header comment, so the shipped documentation offers
   users a feature the compiler cannot emit. That is a docs-versus-reality
   mismatch of exactly the kind the release gate exists to catch.

   **Either make it selectable or stop documenting it — before v0.1.0.**
   Making it selectable is the better answer and is small: a second optional
   argument, a string-to-enum mapping in the lowerer, a test, and an example.

3. **`rest_for_one` missing.** OTP has three strategies; the enum defines two.

4. **No restart types.** OTP distinguishes `permanent`, `transient` and
   `temporary` children. Desi treats a normal return from a persistent child as
   a crash — `supervisor.c` says so outright. That is the opposite of OTP's
   `transient`, where a clean exit is not a restart trigger, and it means a
   child that finishes its work legitimately gets restarted anyway.

5. **No links or monitors.** OTP has bidirectional links (death propagates) and
   one-way monitors (death notifies). Neither exists.

6. **No named registry.** No way to look a supervisor or child up by name.

## Planned

**v0.1.x** — selectable strategy (gap 2), and restart types (gap 4). Both are
small: a constructor argument, and a policy field per child. Restart types also
fix the current oddity where a child that finishes its work cleanly is
restarted as though it crashed.

**v0.2.0** — `rest_for_one` (gap 3), links and monitors (gap 5), named
registry (gap 6). Conventional OTP features with no architectural obstacle.

**v0.x.0, needs design first** — nesting and escalation (gap 1), the one that
changes what supervision *means* in Desi.

The obstacle is not effort, it is that **Elixir's model does not port
directly**. BEAM processes are isolated: each owns its heap and shares nothing,
so killing one and restarting it is safe by construction. Desi's tasks are OS
threads over shared memory, and a thread that dies holding a lock, or midway
through mutating something another task can see, cannot be restarted into a
consistent world. This is the same reason Go cannot kill a goroutine and Rust
cannot kill a thread.

Two routes out, and they should be chosen deliberately:

1. **Isolate tasks properly** — a heap each, communication by copying. This is
   what BEAM does, and it would cost much of the performance work already done.
2. **Supervise only what is provably isolated** — have the compiler determine
   which tasks share no mutable state, and give those real supervision.

Route 2 is the same question as data-race safety: *what may safely cross a
channel* and *what may safely be killed and restarted* have one answer —
**state that is not shared**. Answering it once buys both, and keeps the
surface annotation-free, which is why it is worth designing rather than
building piecemeal. See [../vision/goals.md](../vision/goals.md).

## What release material may say

**"Structured concurrency with supervisors"** — accurate today, and what the
README says.

**Not** "OTP", and **not** "supervision trees", until nesting and escalation
land.
