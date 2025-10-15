# Desi Syntax (revised bootstrap)

This document describes the **lexical** rules implemented today. Parser rules come later.

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

IDENT = (Letter | "*") { Letter | Digit | "*" }

```

Unicode letters are accepted. Examples: `x`, `_tmp`, `Point2D`.

## Keywords and builtin types

**Keywords** (reserved as identifiers):
`import from as pub def async class struct enum type let mut return if elif else while for in using defer match select await true false none and or not`

**Builtin types** (not keywords; still lexed as `IDENT`):
`bool int float str bytes list dict set tuple any none never`

## Literals

### Integer literals
- Decimal: `0`, `123`, `1_000_000`
- Binary: `0b1010_0101`
- Octal: `0o755`, `0o7_55`
- Hex: `0xDEAD_BEEF`, `0xdead_beef`

**Separators:** `_` between digits only — not leading, not trailing, not doubled.

### Decimal floats
- `1.0`, `.5`, `2.`, `1_234.5_6`
- Exponent: `1e9`, `1.25e-3`, `1.2_34e+5`
- Underscores allowed in the significand and exponent **between digits** only.

### Hexadecimal floats
- Forms:
  - `0x1p4`
  - `0x1.fp3`
  - `0x.8p+2`
- Fractional hex without `p` is **invalid** (diagnostic `DLE0016`).
- Exponent `p±<digits>` requires at least one digit; underscores allowed **between digits** only.

### String literals

- Short string: `"..."`
- f-string placeholder form (reserved for future interpolation): `f"..."` (currently just lexed as a string)
- Long string: `""" ... """` (may span lines). Only closes on an exact `"""`; `""` inside is fine.

**Escapes supported:**

```

\  "  \n  \r  \t  \0  \xNN  \uXXXX  \UXXXXXXXX  {  }

```

Invalid escapes emit `DLE0020`. Unterminated strings/long strings emit `DLE0001`.

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

**Notes:**
- `**` is power; `^` is bitwise XOR.
- `|>` is the pipeline operator.
- Keywords `and`, `or`, `not` are lexed as keywords (logical ops).

## Diagnostics (lexer domain)

The lexer collects non-fatal errors and continues scanning. The CLI prints tokens (stdout), flushes, then renders diagnostics (stderr).

Common codes:
- `DLE0001` `lexer.unterminated_string` — unterminated short/f/long string
- `DLE0003` `lexer.tabs_only_indentation` — spaces used for indentation; tabs required
- `DLE0011` `lexer.invalid_number`
- `DLE0012` `lexer.invalid_float_fraction`
- `DLE0013` `lexer.invalid_float_exponent`
- `DLE0014` `lexer.invalid_hex_literal`
- `DLE0015` `lexer.invalid_hex_fraction`
- `DLE0016` `lexer.missing_hex_exponent` — hex fraction without `p`
- `DLE0017` `lexer.invalid_hex_float_exponent`
- `DLE0018` `lexer.invalid_binary_literal`
- `DLE0019` `lexer.invalid_octal_literal`
- `DLE0020` `lexer.invalid_escape_sequence`
- `DLE0099` `lexer.generic_lexer_error`

## CLI

- `-tokens <file>` — dump tokens and then diagnostics
- `-demo-layout` — print layout token stream for a small sample
- `-diag` — print a demo diagnostic
- `-version` — version string

