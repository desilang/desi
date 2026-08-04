# Planned work, by release

Design notes for features that are not finished. Each page states what exists
today and what is planned, against a target release. Nothing here is a promise
about a date.

Versioning follows the usual reading: **v0.1.x** is a patch or minor addition
that does not change how existing programs are written; **v0.2.0** and later may
change the surface language.

## v0.1.x

| Page | What is planned |
|---|---|
| [supervision.md](supervision.md) | Selectable restart strategy; restart types (`permanent`/`transient`/`temporary`) |
| [hybrid_memory_management.md](hybrid_memory_management.md) | Stack allocation for values that neither escape nor grow |

## v0.2.0

| Page | What is planned |
|---|---|
| [supervision.md](supervision.md) | `rest_for_one`, links and monitors, named registry |
| [hybrid_memory_management.md](hybrid_memory_management.md) | Phase 3: explicit `arena.scope()` |
| [async_await_full.md](async_await_full.md) | Full async/await |
| [rc_arc.md](rc_arc.md) | Reference-counted sharing |
| [generic_type_aliases.md](generic_type_aliases.md) | Generic type aliases |
| [variance.md](variance.md) | Variance rules for generics |
| [runtime_type_descriptors.md](runtime_type_descriptors.md) | Runtime type descriptors |
| [hot_reload_panic.md](hot_reload_panic.md) | Hot reload hardening |

## v0.x.0 — needs design before implementation

| Page | Why it is not scheduled |
|---|---|
| [supervision.md](supervision.md) | Nesting and escalation. Elixir's model does not port to shared-memory threads; the route out is the same question as data-race safety, and wants one answer rather than two |
| [distributed_systems.md](distributed_systems.md) | Depends on the isolation model above |

## Reading these

A page describing something as shipped means shipped in the release named on
it, not "done in some branch". Where a page records a measurement, the number
was taken on the machine and configuration stated next to it — treat it as
evidence for a decision, not as a benchmark result for the project.
