
# P02 · `match` as Expression (Planned)

Goal: allow `match` to yield a value.

Spec sketch:

```desi
let msg =
  match r:
    Ok(x): "ok: " + str(x)
    Err(e): "err: " + e
```

Acceptance checklist:

* [ ] Grammar: expression form of `match`.
* [ ] Checker: type unification across arms; exhaustiveness.
* [ ] Codegen: evaluate to a temporary; ensure fallthrough is impossible.
