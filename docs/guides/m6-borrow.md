# M6 — Borrow Checker (Phase-2 quick guide)

**Param modes**
- `T` (move, default) — ownership passed into callee.
- `ref T` (shared) — many readers, no mutation.
- `inout T` (unique mutable) — exactly one writer; cannot be held across `await`.

**Caller-side rules**
- `inout` requires a **mutable lvalue**: identifier, `obj.field`, `arr[idx]`.
- **Aliasing forbidden:** in a single call, if any param is `inout`,
  two actuals must not refer to the **same base storage**; also reject `inout` + `ref` of the same base.
- **Use-after-move:** passing an identifier to a `move` param consumes it;
  subsequent uses error (`DBR0004`).
- **`ref` requires an lvalue** — passing a temporary/rvalue to `ref` is an error (**DBR0005**).

```desi
# Bad: temporary/rvalue to `ref`
def view(ref x: int) -> int:
  0
def main():
  view(1)          # DBR0005: ref argument must be an lvalue

# Good: lvalue (identifier/field/index)
let a = 1
view(a)            # ok
view(p.f)          # ok if p is a named lvalue
view(arr[i])       # ok if arr is a named lvalue
```

**Callee-side rule (Phase-2 refinement)**
- In `async def`, `await` is only illegal **before** the **last use** of any `inout` param.
- If an `inout` param is never referenced, we conservatively treat its last use as end-of-body.

**Diagnostics**
- `DBR0001` — cannot hold `inout` across `await` (lists implicated params).
- `DBR0002` — `inout` argument must be a mutable lvalue.
- `DBR0003` — `inout` cannot alias with other arguments in the same call.
- `DBR0004` — use after move (future polish: include a secondary label “moved here”).

**Future polish ideas**
- Treat primitives (`int/float/bool/str`) as **Copy**, not move.
- Finer alias analysis by field/index path when safe.
- Secondary labels for `DBR0004` pointing to the move site.
