# M5 — Ergonomics v1 (Phase-4)

This phase adds a few Pythonic niceties on top of the M5 imports/resolution work.

## 4a. Implicit `str` on `+`

When either operand is `str`, `a + b` yields `str`. The non-`str` side is permitted for core primitives (`int`, `float`, `bool`, `str`). This is a **typing rule**; lowering to explicit `str(_)` calls may be added in a later pass.

Examples:

```desi
"v=" + 10       # str
3.14 + "π"      # str
true + " ok"    # str
```

## 4b. f-strings (stage 1)

The lexer recognizes `f"..."` as a string literal (`str`). Stage-1 treats f-strings as `str` at type-check time; format specs and expression holes are not yet parsed/desugared (future stage).

## 4c. Tuple unpacking & destructuring

Destructuring via multi-LHS is already supported:

```desi
a, b := pair()
```

Destructuring in `for` targets is slated for a follow-up (keep the `for` target simple for now).

## 4d. Slice steps `s[i:j:k]`

The parser accepts slice forms (`[i:j]`, `[i:j:k]`, `[:j]`, `[:]`, `[::k]`, etc.).
**Typing in M5:** slicing a `str` yields `str`. (Slices on other containers will be typed in a future milestone.)

Examples:

```desi
let s: str = "abcdef"
let mut t: str = ""
t := s[1:4]
t := s[:5]
t := s[::2]
t := s[2::]
t := s[1:5:2]
```

> Note: Index expressions `i`, `j`, `k` are parsed and visited; strict index-type checking and container-element typing are deferred to a later milestone.
