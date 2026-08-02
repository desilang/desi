# Desi Memory Model

Desi has no garbage collector and no manual `free`. The compiler inserts
deterministic cleanup at the point where a value's owner goes out of
scope. This page states exactly what is freed, when, and what is not —
so you can reason about memory precisely.

## The core rule

Every heap value has one **owner**: the variable (or temporary) that
holds it. When the owner's scope ends, the compiler frees the value.
There are no pauses, no background collector, and no reference-counting
overhead except where you explicitly ask for it (`rc`/`arc`).

```desi
def process():
    let items = [1, 2, 3]     # `items` owns the list
    let table = {"a": 1}      # `table` owns the dict
    # ... use them ...
    # scope ends here: `table` freed, then `items` freed (reverse order)
```

## What gets freed at scope exit

| Value | Cleanup |
|---|---|
| `list`, `set`, `dict` | the container and its backing storage |
| `struct`, `enum` | the value, plus any heap fields it owns, recursively |
| class instance | its `__del__` method (and each base class's), then the instance |
| `rc[T]` / `arc[T]` | reference count decremented; freed when the last owner drops |
| `weak[T]` | the weak reference is released; it never keeps the value alive |
| string **temporaries** | results of `+`, f-strings, `str(x)`, `.replace()`, `.join()`, etc. are freed at the end of the statement's scope if nothing takes ownership |

## Move semantics: ownership transfers, never duplicates

Binding, returning, passing to a function, storing into a collection, or
constructing a struct/class **moves** ownership. The compiler tracks the
move and does not free the source — so a value is freed exactly once.

```desi
let a = [1, 2, 3]
let b = a           # ownership moves to `b`; `a` is not freed again
xs.append(a)        # ownership moves into the list
return a            # ownership moves to the caller
```

## Borrowing: reading without owning

Some expressions produce a **borrow** — a view into a value someone else
owns. Borrows are never freed independently; the owner's cleanup handles
them.

```desi
let x = xs[0]        # borrows an element out of `xs`
let n = obj.field    # borrows a field of `obj`
let v = res?         # borrows the payload out of a Result/Option
```

## String accumulators

A mutable string built up in a loop is recognized and given owned,
in-place semantics so it neither leaks intermediates nor copies the whole
string each step:

```desi
let mut s = ""
for word in words:
    s := s + word    # appends in place (realloc), frees the previous buffer
```

This applies only when the compiler can prove `s` is used solely in
borrowing positions (concatenation, comparison, `len`, `print`, f-strings,
`return`). If it can't prove that — for example you pass `s` to another
function, store it in a collection, or alias it with `let t = s` — the
optimization is skipped and the default rules apply.

## What leaks, by design

Desi's cleanup is **conservative**: when the compiler cannot prove it is
safe to free something, it leaks it rather than risk freeing memory that
is still in use. These are the current boundaries where that happens:

- **Heap elements inside a collection.** A `list[str]`, `list[MyStruct]`,
  or `dict` with heap values frees its backing array, but the individual
  strings/structs it holds are not freed. (Numbers and booleans are
  stored inline, so those collections leak nothing.)
- **String locals that may alias a literal.** A `str` variable bound from
  something other than a tracked temporary could point at a string
  literal (which lives in the program image and must never be freed), so
  it is left alone.
- **Values with cross-function ownership.** Anything passed to a
  user-defined function is assumed possibly-retained by that function and
  is not freed by the caller.
- **Temporaries inside `match` arms and comprehensions.** These lower into
  conditional blocks; a temporary created there is leaked rather than
  freed at a point where the free would be unsafe.

Leaks are bounded to the process lifetime and never corrupt memory. Most
programs — short-lived tools, request handlers, batch jobs — never notice
them. Long-running services that build many heap-element collections
should be aware of the first boundary above.

## The safety guarantee

In safe code (no `unsafe` blocks, no raw FFI), the cleanup machinery
never emits a free for a pointer that could still be reachable. Unclear
ownership always resolves to "leak", never to "free and hope". This means
the automatic memory management does not produce use-after-free or
double-free by construction. It does **not** guarantee the absence of all
leaks — see the boundaries above.

The same standard applies to the other end of memory. Running out of stack
is a write past its end — an access violation the program cannot report on,
because the fault happens while it is happening. Every function that can
recurse carries a guard that raises a `RuntimeError` while there is still
stack left to raise it on, so exhaustion is an error a `try` can catch and a
supervisor can survive, not a crash.

What is measured is the space itself, not a count of frames. A frame count
is only a proxy for the space it stands for, and it fails in both directions:
large frames exhaust the stack in fewer calls than the count allows, and a
raised limit would otherwise authorise recursing past the end of the stack.
`set_recursion_limit` is therefore a ceiling and never a licence — it is
clamped to the real end of the stack, so it can lift the limit but never past
the point where the program would fault.

Also unguarded, deliberately: an infinite *tail* recursion. It compiles to a
jump and consumes no stack, so nothing runs out — it is an infinite loop, and
Desi does not claim to detect those.

Implementation and the measurements behind it:
[contributing/compiler/limits.md](contributing/compiler/limits.md).

## For contributors

The implementation, invariants, and the gotchas that have caused real
bugs are documented in
[contributing/runtime/drops-implementation.md](contributing/runtime/drops-implementation.md).
The roadmap toward closing the remaining leak boundaries (collection
element ownership, full string-local ownership, scope arenas) is in
[roadmap/todo/hybrid_memory_management.md](roadmap/todo/hybrid_memory_management.md).
