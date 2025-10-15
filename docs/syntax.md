# Desi Syntax (revised bootstrap)

This doc tracks the **implemented** subset at each milestone. M2 extends M1 with:

- **Decorators** on declarations (currently functions) and **docstring attachment** (first `"""..."""` in a block).
- **Inline forms** for `if`/`while`/`for` (`if cond: stmt`, etc.).
- **Using/Defer** statements.
- **Augmented assignment** (`+=`, `-=`, `*=`, `/=`, `%=` , `**=`, `^=`) and multi-target `:=`.
- **Bitwise OR** `|` (now parsed, not just scanned).

## Source form

- UTF-8, `\n` newlines. Shebang `#!` ignored if present top-of-file.

## Layout and indentation

Desi uses layout with `NL`, `Indent`, `Dedent`.
**Policy:** **tabs-only** at the start of a non-blank, non-comment line. Spaces at BOL emit `DLE0003`. Blank lines and comment-only lines do not affect indentation.

## Comments

`#` to end of line (except future `#{` set literal; currently treated as `#` + `{`).

## Identifiers

Letters/`_`/digits. Builtin types still lexed as `IDENT` (see below).

## Keywords

`import from as pub def async class struct enum type let mut return if elif else while for in using defer match select await true false none and or not`

## Builtin types

`bool int i8 i16 i32 i64 i128 isize u8 u16 u32 u64 u128 usize f32 f64 str string future none list dict set tuple`

## Literals

- Integers: dec/hex/bin/oct with `_` separators.
- Floats: decimal `1.2`, `2.`, `.5`, decimal exponents; **hex floats** `0x1.fp3`.
- Strings: `"..."`, `f"..."`, long `"""..."""` (triple-quoted).
  - Triple-quoted strings are recognized specially as **docstrings** when they are the **first statement in a block**; the parser converts that first statement to a `DocStringStmt` and, for function blocks, attaches it to the decl.

## Operators & punctuators (implemented today)

Greedy tokenization (longest wins).

Grouping: `(` `)` `[` `]` `{` `}`
Delimiters: `,` `:` `.` `@`
Assignment: `=` `:=` `+=` `-=` `*=` `/=` `%=` `**=` `^=`
Arithmetic: `+` `-` `*` `/` `%` `**`
Bitwise/pipeline: `^` `|` `|>`
Compare: `==` `!=` `<` `<=` `>` `>=`
Arrows: `->` `=>`
Bang: `!`

### Expression precedence (high → low)

```

** (right-assoc)
unary: -  !  not  await
*  /  %
+  -
^
|
< <= > >=
== !=
|>
and
or


```

## M2 statements

- `let [mut] name [: Type] = Expr`
- `return [Expr]`
- `if Expr: SimpleStmt` or block form (`:` + NL + indented block). `elif`/`else` supported in both forms.
- `while Expr: SimpleStmt` or block form.
- `for Target in Expr: SimpleStmt` or block form (Target is parsed-only).
- `using Target [= Expr]: NL Block`
- `defer CallExpr`

## Decorators & docstrings (M2)

Decorators immediately precede a decl (today: `def`). They’re attached to the decl’s AST.
Docstrings: the first triple-quoted string in a function body is attached to the function node and removed from the block.

## CLI

- `-tokens <file>` — dump tokens and any lexer diagnostics
- `-demo-layout` — print layout token stream for a small sample
- `-diag` — render a sample diagnostic using `codes.json`
- `-ast <file>` — parse and pretty-print AST
- `-version` — tool version

