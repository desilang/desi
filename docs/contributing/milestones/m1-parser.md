# Desi M1: Parser & AST Guide

This guide documents the **M1 parser subset** implemented now.

---

## 1) CLI quick start

```bash
go build ./...
go test ./...

# AST dump
go run ./compiler/cmd/desic -ast examples/07_ast_smoke.desi
```

---

## 2) What parses in M1

* **Functions:** `def` / `async def` with params, optional return type.
* **Blocks:** indentation-based; **tabs-only**. Blank/comment lines may lead a block.
* **Statements:** `let [mut] name [: Type] = Expr`, `return [Expr]`, `Expr` (expr-stmt).
* **Expressions (with precedence):**
  `**` (right-assoc) > unary (`- ! not await`) > `* / %` > `+ -` > `^` >
  `< <= > >=` > `== !=` > `|>` > `and` > `or`.
* **Postfix (greedy):** `call()`, `index[]`, `field .` in any order.
* **Types (minimal):** names only (plain or dotted), no generics.

Unsupported (for now): `|` (bitwise OR), `**=` and other assignments beyond `=`,
control-flow statements, classes, imports, decorators, lambdas, comprehensions.

---

## 3) Examples

### 3.1 Power + pipeline

```
def powpipe(a: int, b: int) -> int:
  # tabs only!
  let x = a ** 2 |> b + 1
  return x
```

AST (abridged):

```
Func powpipe(a: int, b: int) -> int
  Block
    Let x = ((Ident(a) ** Int(2)) |> (Ident(b) + Int(1)))
    Return Ident(x)
```

### 3.2 Postfix chain (greedy in any order)

```
def z():
  let a = foo(1)[i].bar(2)[j]
```

---

## 4) Diagnostics you’ll see

* `DPE0001` unexpected token (parser)
* `DPE0002` expected token
* `DPE0003` unclosed delimiter (points to the opener)
* `DPE1001` async only valid before `def`
* `DPE1002` async not allowed before `let`

Tips:

* End-of-file acts like a newline at statement end.
* Use tabs for indentation; spaces are diagnosed by the lexer (`DLE0003`).

---

## 5) Async preview

`async def` works in M1.
**Async lambdas** (`async lambda x: await f(x)`) are planned for the async milestone (see roadmap).

