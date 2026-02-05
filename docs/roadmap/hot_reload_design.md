# Hot Reload Design for Desi

**Status**: DESIGN PROPOSAL  
**Date**: Feb 5, 2026

---

## Executive Summary

This document proposes a hot reload strategy for Desi that balances development experience with production reliability. Our recommendation is a **hybrid approach**: native compilation for production with an optional **incremental recompilation + dynamic linking** mode for development.

---

## Question 1: VM/Bytecode vs Native Only?

### Recommendation: **Native with Dynamic Linking** (not a VM)

| Approach | Pros | Cons |
|----------|------|------|
| **Bytecode VM** | Easy hot reload, portable | Slower runtime (10-100x), new codebase to maintain, debugging harder |
| **Native only** | Fast, simple, no runtime overhead | Cold restarts only, loses state |
| **Native + Dynamic Linking** ✅ | Fast production, hot reload in dev | Platform-specific `.so`/`.dylib`, more complex build |

### Justification

1. **Desi's identity is systems-adjacent**: With LLVM backend, FFI to C, and ownership semantics, Desi is positioned between Python and Rust. A VM would undermine this.

2. **Development-only hot reload is sufficient**: Most hot reload usage is during development. Production deployments rarely need it (and when they do, rolling restarts work).

3. **Incremental recompilation is fast enough**: With LLVM's incremental compilation and smart module dependency tracking, we can recompile only changed functions in <1 second for typical edits.

4. **Dynamic linking is mature**: `dlopen`/`dlsym` on Unix, `LoadLibrary` on Windows. We can swap `.so` modules at runtime.

### Implementation Sketch

```
Developer makes edit → 
  1. Detect changed files (file watcher)
  2. Recompile only affected modules to .so
  3. Pause running program at safe point
  4. dlclose old module, dlopen new module
  5. Rewire function pointers in dispatch table
  6. Resume execution
```

---

## Question 2: Handling Mutable Global State During Reload?

### Recommendation: **Hybrid Auto + Manual State Hooks**

The compiler automatically generates serialization for most types, but developers can override for special cases.

| Approach | Pros | Cons |
|----------|------|------|
| **Manual only** | Full control | Tedious, error-prone |
| **Auto-generate only** | Zero effort | Breaks on File/Mutex/FFI |
| **Hybrid: Auto + Override** ✅ | Best of both | Needs trait system |

### How It Works

**Step 1: Compiler auto-generates hooks for simple types**

```desi
# You write this:
class UserSession:
    user_id: int
    last_active: int
    preferences: dict[str, str]

# Compiler AUTO-GENERATES (invisible to you):
# def __to_dict__(self) -> dict[str, any]
# @staticmethod def __from_dict__(d: dict[str, any]) -> UserSession
```

**Step 2: You override ONLY when needed**

```desi
class UserSession:
    user_id: int
    cache_file: File      # ⚠️ Can't serialize a file handle!
    db_conn: Connection   # ⚠️ Can't serialize a connection!
    
    # Developer explicitly handles non-serializable fields:
    def __to_dict__(self) -> dict[str, any]:
        return {
            "user_id": self.user_id,
            "cache_path": self.cache_file.path  # Save PATH, not handle
        }
    
    @staticmethod
    def __from_dict__(d: dict[str, any]) -> UserSession:
        return UserSession(
            user_id=d["user_id"],
            cache_file=File.open(d["cache_path"]),  # REOPEN file
            db_conn=Database.connect()              # RECONNECT
        )
```

### Detection of Non-Serializable Types

The compiler warns when auto-generation is impossible:

```
warning[DHR001]: Type 'UserSession' contains non-serializable field 'cache_file: File'
  --> src/session.desi:3:5
   |
 3 |     cache_file: File
   |     ^^^^^^^^^^^^^^^^
   = help: Implement __to_dict__ and __from_dict__ manually
   = help: Or mark field with @transient to exclude from state
```

### The `@transient` Escape Hatch

For fields that don't need to survive reload:

```desi
class RequestHandler:
    config: Config           # Preserved
    @transient
    temp_buffer: list[byte]  # Excluded from state (recreated on reload)
```

### State Categories (Updated)

| Type | Auto-Serializable? | Developer Action |
|------|-------------------|------------------|
| `int`, `float`, `str`, `bool` | ✅ Yes | None needed |
| `list[T]`, `dict[K,V]`, `Option[T]` | ✅ Yes (if T is) | None needed |
| User `class` with simple fields | ✅ Yes | None needed |
| `File`, `Connection`, `Socket` | ❌ No | Override hooks |
| `sync.Mutex`, `sync.Channel` | ❌ No | Override or `@transient` |
| C FFI pointers | ❌ No | Must override |

---

## Question 2.5: Debugging - Critical for Adoption

### Philosophy: **Debugging Must Be BETTER Than Other Languages**

> *"No coder would go through pain if a new programming language doesn't offer any benefits!"*

Desi hot reload must include **first-class debugging** that makes developers WANT to use it.

### Debugging Features for Hot Reload

#### 1. **Live REPL in Running Process**

```bash
$ desic attach myserver
Connected to myserver (pid 12345)

desi> users.len()
=> 42

desi> handler.config.timeout
=> 30

desi> handler.config.timeout = 60  # LIVE EDIT
=> Updated handler.config.timeout to 60

desi> debug.breakpoint("myendpoint.desi:25")
=> Breakpoint set at myendpoint.handle:25
```

#### 2. **Reload with Diff Preview**

```bash
$ desic reload myendpoint --preview
Changes to reload:

  def handle(request):
-     tax_rate = 1.1   # OLD
+     tax_rate = 1.08  # NEW
      return total * tax_rate

State impact:
  ✓ UserSession: 42 instances will be migrated
  ✓ config: preserved (no changes)
  ⚠ RequestCache: will be cleared (@transient)

Proceed? [y/N]
```

#### 3. **Time-Travel Debugging** (Future)

Record state snapshots, replay requests:

```bash
$ desic replay --request-id abc123
Replaying request abc123 with NEW code...
  → Input: {"amount": 100}
  → OLD result: {"total": 110.0}  # 1.1x bug
  → NEW result: {"total": 108.0}  # 1.08x fixed ✓
```

#### 4. **Stack Traces Across Reloads**

When an error occurs, show both old and new code context:

```
Error at myendpoint.desi:25 (reloaded 2 mins ago)
    Current code:
      25 |     return total / count  # FIXED
    
    Previous code (before reload):
      25 |     return total / cnt    # HAD TYPO
    
    Tip: Error was in OLD code, already fixed in current version.
```

#### 5. **Live Metrics Dashboard**

```
$ desic watch --dashboard

┌─ Hot Reload Status ─────────────────────────┐
│ Last reload: 10s ago (myendpoint.desi)      │
│ Active versions: 2 (draining 1 old request) │
│ State: 42 UserSession, 1 Config preserved   │
├─ Performance ───────────────────────────────┤
│ Avg response: 12ms (pre-reload: 15ms)       │
│ Errors: 0 (down from 5/min before fix)      │
└─────────────────────────────────────────────┘
```

### Why This Matters for Adoption

| Feature | Benefit for Developer |
|---------|----------------------|
| Live REPL | Inspect state without restart, experiment live |
| Diff preview | Confidence about what's changing |
| Time-travel | Verify fix worked without manual testing |
| Cross-reload traces | Understand if bug was in old vs new code |
| Metrics dashboard | See immediate impact of hot fix |

### Comparison: Desi vs Others

| Feature | Desi (Proposed) | Elixir | Go | Python |
|---------|----------------|--------|-----|--------|
| Hot reload | ✅ Native | ✅ VM | ❌ Restart | ⚠️ Limited |
| Live REPL | ✅ | ✅ IEx | ❌ | ⚠️ pdb |
| Auto state migration | ✅ | ⚠️ Manual | ❌ | ❌ |
| Diff preview | ✅ | ❌ | ❌ | ❌ |
| Time-travel debug | ✅ Future | ❌ | ❌ | ❌ |

**The goal: Make Desi hot reload SO good that developers choose Desi BECAUSE of debugging, not despite it.**

---

## Question 3: Module Versioning - Semantic or Content Hashing?

### Recommendation: **Content Hashing for Dev, Semantic for Distribution**

| Approach | Pros | Cons |
|----------|------|------|
| **Semantic versioning** | Human readable, dep management | Requires manual version bumps |
| **Content hashing** ✅ for dev | Automatic, precise | Hashes are opaque |
| **Hybrid** ✅ | Best of both | Slightly more complex |

### Justification

1. **Hot reload is for development**: During dev, you don't care about version numbers. You care "did this file change?" Content hashing answers that perfectly.

2. **Semantic versions matter for distribution**: When publishing packages, users need to reason about compatibility. `1.2.3 -> 1.3.0` means "new features, backward compatible."

3. **Content hashes catch transitive changes**: If module A imports B, and B changes, A's "effective hash" changes even if A's source didn't. This triggers recompilation of A's callers.

### Proposed Scheme

```
# Development mode:
module_hash = sha256(source_code + import_hashes)
if module_hash != cached_hash:
    recompile()

# Distribution:
[package]
name = "mylib"
version = "1.2.3"  # Semantic version for humans
content_hash = "abc123"  # For lockfile reproducibility
```

---

## Question 4: State Migration Hooks?

### Recommendation: **Yes, with `__migrate__(old_version, old_state)` Convention**

### Justification

1. **Erlang proves this works**: Erlang's `code_change/3` callback has been used in production for 30+ years.

2. **Type changes need explicit handling**: If a class gains a field, the migration hook initializes it with a default.

3. **Optional but discoverable**: Most modules won't need it. But when they do, the pattern is documented.

### Proposed Hooks

```desi
class UserSession:
    user_id: int
    last_active: int
    preferences: dict[str, str]  # NEW FIELD in v2
    
    @staticmethod
    def __migrate__(version: int, old_data: dict[str, any]) -> UserSession:
        """Migrate from older versions."""
        if version < 2:
            # v1 didn't have preferences
            old_data["preferences"] = {"theme": "dark"}
        return UserSession(**old_data)
```

### Migration Flow

```
1. Before reload: serialize all live instances via __to_dict__()
2. Store: {class_name: version, instances: [...]}
3. After reload: for each instance:
   - If version matches: deserialize directly
   - If version differs: call __migrate__(old_ver, data)
```

---

## Question 5: Concurrency - What Happens to Running Tasks?

### Recommendation: **Drain + Barrier Pattern**

| Approach | Pros | Cons |
|----------|------|------|
| **Kill all tasks** | Simple | Loses work, can corrupt state |
| **Wait for all tasks** | Safe | Can block forever on long tasks |
| **Drain + timeout** ✅ | Balanced | Some tasks may be killed |
| **Version coexistence** | Zero downtime | Complexity explosion |

### Justification

1. **Tasks may hold locks**: Killing a task holding a mutex leaves the mutex poisoned. We need graceful shutdown.

2. **Infinite waits are bad UX**: If a task runs for hours, hot reload should still work. Timeout + cancel is reasonable.

3. **Inspiration from Go's `http.Server.Shutdown`**: Drain active requests, give them a deadline, then force close.

### Proposed Protocol

```desi
# Hot reload sequence:
1. Set global flag: reloading = true
2. New task spawns are queued (not started)
3. Signal all running tasks: check_reload_requested()
4. Wait up to 5 seconds for tasks to complete
5. Force-cancel remaining tasks (with warning)
6. Serialize state via __on_reload_save__
7. Unload old code, load new code
8. Restore state via __on_reload_restore__
9. Resume queued tasks with new code
10. Set reloading = false
```

### Runtime API

```desi
# Tasks should cooperate:
async def my_long_task():
    while True:
        if sync.reload_requested():
            # Graceful exit point
            save_checkpoint()
            return
        await process_next_item()
```

---

## Summary of Recommendations

| Question | Recommendation |
|----------|----------------|
| **VM vs Native?** | Native + dynamic linking (no VM) |
| **Global State?** | Explicit save/restore hooks |
| **Versioning?** | Content hash for dev, semver for dist |
| **Migration Hooks?** | `__migrate__(version, data)` pattern |
| **Concurrency?** | Drain + timeout + graceful cancel |

---

## Implementation Phases (Ordered by Foundational Value)

> **Ordering Philosophy**: Features are ordered by how much they *enable future features*, not by ease of implementation. A foundation-first approach means each phase builds leverage for the next.

---

### Phase 1: File Watcher + Auto-Restart (Foundation)

**Why First**: Everything else depends on detecting changes and triggering rebuilds.

- [ ] `desic watch` command with file system watcher (fsnotify)
- [ ] Automatic recompile on file save
- [ ] Process restart (loses state, but enables fast iteration)
- [ ] **Basic diff preview** (console output of changed lines)

**Enables**: All future phases - can't reload without detecting changes.

---

### Phase 2: State Serialization Infrastructure

**Why Second**: State preservation is the #1 user pain point. Auto-serialization reduces friction to near-zero.

- [ ] Auto-generate `__to_dict__` / `__from_dict__` for simple types
- [ ] Compiler warning `DHR001` for non-serializable fields
- [ ] `@transient` decorator for excluded fields
- [ ] State file persistence (JSON) between restarts
- [ ] `__migrate__(version, data)` hooks for schema changes

**Enables**: Hot reload with state preservation, time-travel debugging, rollback.

---

### Phase 3: Live REPL (`desic attach`)

**Why Third**: Highest developer leverage. Inspect, modify, experiment without restart.

- [ ] Attach to running Desi process via IPC
- [ ] Read/write global variables in REPL
- [ ] Call functions interactively
- [ ] Set/clear breakpoints from REPL
- [ ] Tab completion with type hints

**Enables**: Live debugging, exploratory testing, quick experiments.

---

### Phase 4: Dynamic Linking (True Hot Reload)

**Why Fourth**: This is the core hot reload mechanism, but needs state infra first.

- [ ] Compile modules to `.so` / `.dylib` files
- [ ] Function pointer dispatch table in runtime
- [ ] Runtime module swap via `dlopen` / `dlclose`
- [ ] Incremental recompilation (only changed functions)
- [ ] Zero-downtime reload (old code finishes, new code for new requests)

**Enables**: Production-grade hot reload, true zero-downtime fixes.

---

### Phase 5: Debugging Tooling Suite

**Why Fifth**: Now that hot reload works, make debugging exceptional.

- [ ] **Diff preview with state impact** (show which instances migrate)
- [ ] **Cross-reload stack traces** (show old vs new code context)
- [ ] **Live metrics dashboard** (`desic watch --dashboard`)
- [ ] **Breakpoint persistence across reloads**

**Enables**: Developer confidence, quick bug identification.

---

### Phase 6: Advanced Features (Future)

**Why Last**: These are powerful but require all prior phases to be solid.

- [ ] **Time-travel debugging** (record requests, replay with new code)
- [ ] **Rollback support** (revert to previous code version)
- [ ] **Canary testing** (route X% of traffic to new code)
- [ ] **IDE integration** (VS Code extension for hot reload UX)

---

## Debugging Feature Priority (Detailed Justification)

| Priority | Feature | Foundational Value | Long-term Benefit |
|----------|---------|-------------------|-------------------|
| **1** | File watcher + auto-restart | ★★★★★ (enables everything) | All future features depend on this |
| **2** | State serialization | ★★★★★ (enables persistence) | Required for: rollback, time-travel, migration |
| **3** | Live REPL | ★★★★☆ (highest dev leverage) | Developers interact with code live |
| **4** | Dynamic linking | ★★★★☆ (core reload mechanism) | Zero-downtime, production-ready |
| **5** | Diff preview | ★★★☆☆ (confidence) | Developers trust the reload |
| **6** | Cross-reload traces | ★★★☆☆ (debugging) | Know if bug is old vs new code |
| **7** | Metrics dashboard | ★★☆☆☆ (visibility) | Nice-to-have, professional feel |
| **8** | Time-travel | ★★☆☆☆ (advanced) | Killer feature, but complex |
| **9** | Rollback | ★★☆☆☆ (safety net) | Production peace of mind |
| **10** | Canary testing | ★☆☆☆☆ (advanced ops) | Enterprise feature |

---

## Advanced Recommendations

### 1. Rollback Support

If hot reload introduces a bug, developers need an immediate escape hatch:

```bash
$ desic rollback
Rolled back to version 3 (2 mins ago)
  myendpoint.desi: handle() reverted
  State: 42 UserSession instances preserved
```

**Implementation**: Keep last N compiled `.so` files + state snapshots. Rollback swaps back.

---

### 2. Canary Testing (Gradual Rollout)

For production safety, route only X% of traffic to new code:

```bash
$ desic reload myendpoint --canary 10%
Routing 10% of requests to new code...
  → 100 requests processed
  → 0 errors (was 5/min on old code)
  
Promote to 100%? [y/N]
```

**Implementation**: Dispatch table with weighted routing. Metrics track error rates per version.

---

### 3. Breakpoint Persistence Across Reloads

Developers lose breakpoints on reload. Desi should preserve them:

```bash
$ desic attach myserver
desi> breakpoint("myendpoint.desi:25")
=> Breakpoint set

# Developer makes edit, hot reload happens

desi> 
Hit breakpoint at myendpoint.desi:25 (re-mapped after reload)
  Old line: return total * 1.1
  New line: return total * 1.08  # <-- stopped here
```

**Implementation**: Breakpoints stored by source location, not address. Re-map after reload using source maps.

---

### 4. IDE Integration (VS Code Extension)

Hot reload should feel native in the editor:

```
┌─────────────────────────────────────────────────┐
│ myendpoint.desi                            [▶]  │  ← Run with hot reload
├─────────────────────────────────────────────────┤
│ 25 │ return total * 1.08                        │
│    │        ^^^^^^^^^^^^                        │
│    │        Changed (will hot reload on save)   │
├─────────────────────────────────────────────────┤
│ 🔥 Last reload: 5s ago | State: 42 sessions     │
│ ✓ 0 errors since reload                         │
└─────────────────────────────────────────────────┘
```

**Implementation**: Language Server Protocol (LSP) extension. We already have `desilsp`.

---

### 5. Test-in-Production Mode

Run specific test requests against new code before full switch:

```bash
$ desic reload myendpoint --test-request '{"amount": 100}'
Test result:
  Input:  {"amount": 100}
  Output: {"total": 108.0}  # Expected: 108.0 ✓

Proceed with full reload? [y/N]
```

**Implementation**: Queue a synthetic request, route only to new code, compare output.

---

### 6. Health Checks Post-Reload

Automatically verify the reload didn't break anything:

```desi
# In module:
def __after_reload_check__() -> bool:
    """Called after reload. Return false to auto-rollback."""
    # Verify critical state
    assert db.ping(), "Database connection lost"
    assert cache.len() > 0, "Cache should not be empty"
    return true
```

**Implementation**: Call hook after reload. If returns false, trigger automatic rollback.

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| **Dynamic linking is platform-specific** | Abstract behind runtime module; Windows/macOS/Linux implementations |
| **C FFI state can't be migrated** | Document limitation; require explicit cleanup in `__on_reload_save__` |
| **Type changes may corrupt data** | Version tags + migration hooks; fall back to full restart on error |
| **Performance overhead in dev mode** | Dispatch table indirection is ~1-2ns; acceptable for dev |
| **Reload introduces bug** | Rollback support (Phase 6) |
| **Breakpoints lost on reload** | Breakpoint persistence via source mapping (Phase 5) |
| **IDE doesn't show reload status** | VS Code extension integration (Phase 6) |

---

## Why This Order?

```
                    ┌─────────────────────────────────────┐
                    │  Phase 6: Advanced Features         │
                    │  (Time-travel, Rollback, Canary)    │
                    └───────────────┬─────────────────────┘
                                    │ requires
                    ┌───────────────▼─────────────────────┐
                    │  Phase 5: Debugging Suite           │
                    │  (Diff preview, Cross-reload trace) │
                    └───────────────┬─────────────────────┘
                                    │ requires
                    ┌───────────────▼─────────────────────┐
                    │  Phase 4: Dynamic Linking           │
                    │  (.so swap, dispatch table)         │
                    └───────────────┬─────────────────────┘
                                    │ requires
                    ┌───────────────▼─────────────────────┐
                    │  Phase 3: Live REPL                 │
                    │  (desic attach, inspect state)      │
                    └───────────────┬─────────────────────┘
                                    │ requires
                    ┌───────────────▼─────────────────────┐
                    │  Phase 2: State Serialization       │
                    │  (Auto __to_dict__, @transient)     │
                    └───────────────┬─────────────────────┘
                                    │ requires
                    ┌───────────────▼─────────────────────┐
                    │  Phase 1: File Watcher              │ ← START HERE
                    │  (desic watch, auto-restart)        │
                    └─────────────────────────────────────┘
```

Each phase builds on the previous. Skipping a phase would create gaps that make later phases incomplete or fragile.

---

## References

- [Erlang Code Loading](https://www.erlang.org/doc/reference_manual/code_loading.html)
- [Elixir Hot Code Swapping](https://elixir-lang.org/getting-started/mix-otp/supervisor-and-application.html)
- [Vite HMR](https://vitejs.dev/guide/features.html#hot-module-replacement)
- [Go graceful shutdown](https://pkg.go.dev/net/http#Server.Shutdown)
- [Julia's incremental compilation](https://docs.julialang.org/en/v1/devdocs/locks/#Code-Loading)
- [VS Code Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/)
