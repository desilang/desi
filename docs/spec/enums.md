# Enums (Tagged Unions) — Stage-1

Enums model sum types (a value is *one* of several variants). Each variant may carry an optional payload.

## Declaration

```desi
enum Result:
  Ok: int
  Err: str

enum MaybeStr:
  Some: str
  None: none
```

Notes:

* `none`/`""`/`void` all mean “no payload” in Stage-1.
* One payload per variant (Stage-1).

## Construction

```desi
let r: Result = Result.Ok(200)
let e: Result = Result.Err("boom")

let m1: MaybeStr = MaybeStr.Some("hi")
let m2: MaybeStr = MaybeStr.None()
```

Rules:

* If a variant has a payload, the constructor takes exactly one argument.
* If a variant has no payload, the constructor takes zero arguments (empty parens).

## Pattern Matching

```desi
match r:
  Ok(x):
    io.println("ok:", x)
  Err(msg):
    io.println("err:", msg)
```

```desi
match m:
  Some(s):
    io.println("some:", s)
  None():
    io.println("none")
```

Typechecking (Stage-1):

* Scrutinee must be an enum.
* Each arm must reference a valid variant of that enum.
* Binder (e.g., `x`, `s`) is typed to the variant’s payload type.
* Duplicate arms are an error.
* Non-exhaustive matches emit a warning listing missing variants.

## C Lowering (overview)

Each enum becomes:

```c
#define <Enum>_<Variant0> 0
#define <Enum>_<Variant1> 1
typedef struct {
  int tag;
  union {
    /* one field per payloadful variant */
    <ctype-of-payload> VariantName;
  } as;
} <Enum>;
```

Constructors return designated initializers:

```c
(Result){ .tag = Result_Ok, .as.Ok = 200 }
(MaybeStr){ .tag = MaybeStr_None }
```

`match` lowers to a `switch(scrut.tag)` dispatch with local extraction of payload when present.
