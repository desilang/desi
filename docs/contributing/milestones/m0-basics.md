# Desi M0: Lexing & Literals Guide (Revised Bootstrap)

This guide documents the **lexical** features implemented in M0. It’s hands-on: copy the snippets into `examples/*.desi` and use the CLI to see tokens and diagnostics.

---

## 1) CLI quick start

```bash
# build & test
go build ./...
go test ./...

# dump tokens for a file
go run ./compiler/cmd/desic -tokens examples/01_token_test.desi

# view the off-side rule (layout) on a tiny sample
go run ./compiler/cmd/desic -demo-layout

# print a sample diagnostic
go run ./compiler/cmd/desic -diag
```

**Tip:** `-tokens` prints tokens to **stdout**, then diagnostics to **stderr** (with a cap so you won’t get flooded).

---

## 2) Indentation & layout

Desi uses the **off-side rule**:

* Emits `NL` at the end of a physical line.
* Emits `Indent` when leading indentation increases.
* Emits `Dedent` when leading indentation decreases.

**Policy:** **tabs-only** for **leading indentation**. Spaces at the start of a non-blank, non-comment line produce:

* Code: `lexer.tabs_only_indentation`
* ID: `DLE0003`
* Help: “tabs required; configure your editor.”

Mid-line spaces for alignment are fine. Comment-only and blank lines don’t affect indentation.

---

## 3) Numbers

All numeric forms below support `_` separators **between digits only** (no leading/trailing/double underscores).

### 3.1 Integers

```desi
let dec = 1_000_000
let bin = 0b1010_0101
let oct = 0o7_55
let hex = 0xDEAD_BEEF
```

### 3.2 Decimal floats

```desi
let a = 1.0      # normal
let b = .5       # leading-dot
let c = 2.       # trailing-dot
let d = 1_234.5_6
let e = 1e9
let f = 1.25e-3
let g = 1.2_34e+5
```

### 3.3 Hexadecimal floats

Hex floats are like C/Rust/Python: significand in hex, exponent in base-2 via `p`/`P`.

```desi
let h1 = 0x1p4      # 1 * 2^4  = 16
let h2 = 0x1.fp3    # 1.F_hex * 2^3
let h3 = 0x.8p+2    # 0.8_hex * 2^2
```

**Invalid (diagnosed, scan continues):**

```desi
let bad1 = 0x1.f      # missing 'p' exponent → DLE0016
let bad2 = 0x_p1      # underscore before any digit → DLE0014
let bad3 = 0x1p+_2    # underscore right after sign → DLE0017
```

### 3.4 Numeric operators (scanned greedily)

* Power: `**` and `**=`
* Bitwise XOR: `^` and `^=`
* Pipeline: `|>` (not numeric per se but often used in expression chains)

```desi
let pow = 2 ** 4
let agg = 2 **= 3
let xor = 0b1010 ^ 0b0011
let pip = value |> transform
```

---

## 4) Strings

### 4.1 Short strings

```desi
let s = "hello"
let t = "quote: \" and backslash: \\"
```

### 4.2 f-strings (reserved)

`f"..."` is lexed like a string today; future revisions may add interpolation.

```desi
let f = f"just a string for now"
```

### 4.3 Long strings

Use triple quotes. Only closes on an **exact** `"""`. `""` inside is fine.

```desi
let blob = """
line 1
line 2 with "" inside
"""
```

### 4.4 Escape sequences (validated)

Supported:

```
\\  \"  \n  \r  \t  \0  \xNN  \uXXXX  \UXXXXXXXX  \{  \}
```

Invalid sequences produce `DLE0020` (`lexer.invalid_escape_sequence`), but scanning continues so you’ll see all issues in one pass.

**Examples:**

```desi
let ok = "A:\x41 B:\u0042 C:\U00000043 \n tab:\t quote:\" backslash:\\ zero:\0"
let bad = "oops:\q"     # DLE0020
let oops = "unterminated  # DLE0001
```

---

## 5) Operators & punctuators

Scanned with **greedy longest-match**:

* Grouping: `(` `)` `[` `]` `{` `}`
* Delimiters: `,` `:` `.`
* Assignment: `=` `:=` `+=` `-=` `*=` `/=` `%=` `**=` `^=`
* Arithmetic: `+` `-` `*` `/` `%` `**`
* Bitwise / pipeline: `^` `|` `|>`
* Compare: `==` `!=` `<` `<=` `>` `>=`
* Arrows: `->` `=>`
* Bang: `!`

Check `examples/06_ops.desi` to see the greediness (`**=` > `**`, `|>` > `|`, `>=` > `>`, `==` as a single token).

---

## 6) Identifiers, keywords, builtin types

* Identifiers: `(Letter | "_") { Letter | Digit | "_" }` (Unicode letters allowed)
* Keywords (reserved):
  `import from as pub def async class struct enum type let mut return if elif else while for in using defer match select await true false none and or not`
* Builtin types (lexed as `IDENT`, not keywords):
  `bool int float str bytes list dict set tuple any none never`

When you run `-tokens`, builtin types still appear as `IDENT`, but the CLI tags them with `(type)` for readability.

---

## 7) Diagnostics

The lexer collects non-fatal errors and **continues scanning**. The CLI prints tokens, then diagnostics (capped, with a “… N suppressed” line if needed).

A few common codes:

* `DLE0001` `lexer.unterminated_string`
* `DLE0003` `lexer.tabs_only_indentation`
* `DLE0011` `lexer.invalid_number`
* `DLE0012` `lexer.invalid_float_fraction`
* `DLE0013` `lexer.invalid_float_exponent`
* `DLE0014` `lexer.invalid_hex_literal`
* `DLE0015` `lexer.invalid_hex_fraction`
* `DLE0016` `lexer.missing_hex_exponent`
* `DLE0017` `lexer.invalid_hex_float_exponent`
* `DLE0018` `lexer.invalid_binary_literal`
* `DLE0019` `lexer.invalid_octal_literal`
* `DLE0020` `lexer.invalid_escape_sequence`
* `DLE0099` `lexer.generic_lexer_error`

---

## 8) FAQ (M0)

**Q: Do functions need a return type?**
A: No. The grammar allows `def f(): …` without `-> Type`. Enforcing return presence/compatibility comes with the type-checker.

**Q: Can I use spaces for indentation?**
A: No—Desi enforces **tabs-only** in leading indentation. Configure your editor to insert tabs on line start.

**Q: Are f-strings interpolated?**
A: Not yet. `f"..."` is reserved and lexed as a string; interpolation will come in a later milestone.

---

## 9) What’s next

* Parser (based on the EBNF in `docs/grammar.ebnf`)
* Type-checker (enforce returns, signatures, etc.)
* `desifmt` auto-converting leading spaces to tabs (opt-in fix)

```
