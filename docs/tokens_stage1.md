
# Desi Tokens (Stage-1 snapshot)

## Keywords
```

package import def return if elif else while let mut defer
true false and or not

```

## Operators & Punctuation
```

( ) \[ ] . , : := -> |>

* * * / % = == != < <= > >=

```

### Precedence (low → high)
1. `|>`
2. `or`
3. `and`
4. `==` `!=`
5. `<` `<=` `>` `>=`
6. `+` `-`
7. `*` `/` `%`
8. unary `-` `!` `not`
9. postfix: call `(...)`, index `[...]`, field `.name`

## Literals
- **Integers**
  - Stage-0 (Go lexer): decimal, binary `0b...`, hex `0x...`
  - Stage-1 (self-hosted Desi lexer): **decimal only** (for now)
- **Strings**: `"..."` (Stage-1 lexer: no escape sequences yet)
- **Booleans**: `true`, `false`

## Whitespace & Layout
- Stage-0 (Go lexer): emits `NEWLINE`, `INDENT`, `DEDENT` (Python-style).
- Stage-1 (Desi lexer): currently **skips whitespace/newlines** and **does not** produce indent tokens. (Planned: line/indent tokens with an indent stack.)

## Comments
- Not yet in Stage-1 lexer. (Planned: `# ...` to end of line.)

## Import Resolution (Stage-0)
- `import a.b.c` resolves to a relative path: `a/b/c.desi` **relative to the entry file’s directory**.

## Type System (Checker)
- Kinds: `int`, `str`, `bool`, `void`, `unknown`.
- Variables are immutable by default; `let mut` makes them assignable with `:=`.
- Type refinement from `unknown` on first assignment; arity & arg checks on calls.
- Conditions for `if/elif/while` must be `bool`, `int`, or `unknown`.
- `defer`: only at function top level; expression must be a **call**.

## Std Shims (Checker + Codegen)
- `io.println(..)`  → `printf` (strings `%s`, others `%d`)
  - Args must be `int | str | bool` (rejects `void/unknown`).
- `fs.read_all(path: str) -> str` → `desi_fs_read_all`
- `os.exit(code: int) -> void`    → `desi_os_exit`
- `mem.free(ptr)` (planned usage with future heap APIs)
- **String API** (Stage-1 additions used by self-hosted lexer):
  - `str.len(s: str) -> int`          → `desi_str_len`
  - `str.at(s: str, i: int) -> int`   → `desi_str_at`    (Unicode scalar)
  - `str.from_code(c: int) -> str`    → `desi_str_from_code`
  - `+` with a string operand lowers to `desi_str_concat(a, b)`
  - `==`/`!=` on strings lower to `strcmp(...) ==/!= 0` (no pointer compare)

## Codegen Notes (C)
- Unary `not` → `!`
- `and` → `&&`
- `or`  → `||`
- String concat `a + b` → `desi_str_concat(a, b)`
- `main` lowered to `int main(void)`

