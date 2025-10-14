# Desi Syntax (revised bootstrap)

This document currently describes the **lexical** rules implemented today. Parser rules are landing milestone-by-milestone; see the grammar for full plans and the M1 subset implemented now.

## Source form

- **Encoding:** UTF-8
- **Newlines:** `\n`
- **Shebang:** a top-line `#!…` is treated as a comment and ignored.

## Layout and indentation

Desi uses **layout** (off-side rule) with three tokens:

- `NL` at the end of a physical line
- `Indent` when indentation increases
- `Dedent` when indentation decreases

**Indentation policy:** **tabs-only** at the start of a line. If a non-blank, non-comment line begins with spaces, the lexer emits `DLE0003` (tabs required) and continues.

Blank lines and comment-only lines do not affect indentation.

## Comments

- From `#` to end of line (unless it begins a `#{...}` set literal in future revisions; the lexer currently treats `#{` like `#` followed by `{`).
- There are no block comments.

## Identifiers

```

Ident = (Letter | "*") { Letter | Digit | "*" }

```

Unicode letters are accepted. Examples: `x`, `_tmp`, `Point2D`.

## Keywords and builtin types

**Keywords** (reserved as identifiers):
`import from as pub def async class struct enum type let mut return if elif else while for in using defer match select await true false none and or not`

**Builtin types** (not keywords; still lexed as `IDENT`):
`bool int float str bytes list dict set tuple any none never`

## Literals

(unchanged; see `docs/guides/m0-basics.md` for examples)

## Operators & punctuators (subset)

Greedy tokenization is used — the longest operator wins.

- Grouping: `(` `)` `[` `]` `{` `}`
- Delimiters: `,` `:` `.`
- Assignment: `=` `:=` `+=` `-=` `*=` `/=` `%=` `**=` `^=`
- Arithmetic: `+` `-` `*` `/` `%` `**`
- Bitwise / pipeline: `^` `|` `|>`
- Compare: `==` `!=` `<` `<=` `>` `>=`
- Arrows: `->` `=>`
- Bang: `!`

**Notes (today):**

- Parser M1 supports: `**`, unary, `* / %`, `+ -`, `^`, `< <= > >=`, `== !=`, `|>`, `and/or`.
- `|` (bitwise OR) and `**=` are **scanned** but **not parsed yet**.

## CLI

- `-tokens <file>` — dump tokens and then diagnostics
- `-demo-layout` — print layout token stream for a small sample
- `-diag` — print a demo diagnostic
- `-ast <file>` — parse a file and pretty-print the AST (M1 subset)
- `-version` — version string

## Preview: async lambda (planned)

We plan to support async lambdas in the async milestone:

```desi
let bodies = await gather(urls |> map(async lambda u: await http.get(u)))
```

Grammar sketch (see `docs/grammar.ebnf`):
`AsyncLambdaExpr = "async" "lambda" LambdaParams ":" Expr` (expression-only body).

