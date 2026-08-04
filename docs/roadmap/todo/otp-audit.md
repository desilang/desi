# Supervision: what Desi actually has, measured against OTP

**Audited 2026-08-03**, against `compiler/runtime/supervisor.c`,
`supervisor.h`, and `compiler/lib/sync/supervisor.desi`.

Desi's supervision is the most distinctive thing in the language — nobody else
offers OTP-style supervision in a compiled language with no garbage collector.
Elixir has the supervision but runs on a VM; Rust and Go have no supervision
trees at all. That makes it worth knowing exactly what is and is not there
before the first release describes it.

## Verdict

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

## What to do about it, and when

**Before v0.1.0:** confirm and fix gap 2 if real — a strategy that exists but
cannot be selected is a bug, not a missing feature. Everything else can wait.

**Not before v0.1.0:** gaps 1 and 4 are the ones that would change what
supervision *means* in Desi, and they deserve design rather than a rushed
implementation. Restart types (4) are cheap and mostly a policy field. Nesting
and escalation (1) is the real project.

**Do not** describe this as OTP or as supervision trees in release material
until gap 1 is closed. "Structured concurrency with supervisors" is honest and
already what the README says; keep it there.
