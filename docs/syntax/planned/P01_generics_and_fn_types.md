
# P01 · Generics and Function Types (Planned)

Goal: Support parametric polymorphism and first-class function types.

Spec sketch:

```desi
def apply[T](x: T, f: (T)->T) -> T:
  f(x)

let inc = def(n: int) -> int: n + 1
let three = apply[int](2, inc)
```

Acceptance checklist:

* [ ] Parser: type params `[T, U, ...]` on `def`.
* [ ] Parser: function types `(A, B)->R`.
* [ ] Checker: type inference and substitution.
* [ ] Codegen: monomorphization or reified generics strategy.
* [ ] Errors/diags for ambiguity and constraints.
