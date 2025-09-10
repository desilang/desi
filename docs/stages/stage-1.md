# Stage-1 — Desi-lexer bridge, adapter, and self-hosting path

> Goal: Use the **lexer written in Desi** to feed the existing **Go parser**, so we can validate token parity, fix gaps, and pave the way to a fully self-hosted compiler.

---

## Why Stage-1 exists

Stage-0 gave us a reliable Go front-end that emits portable C. Stage-1 starts the self-hosting journey by compiling and running a Desi implementation of the lexer, adapting its tokens back into the Go pipeline. This lets us:

- Compare **Go-lexer vs Desi-lexer** output (`lex-diff`)
- Measure/descope **token mapping coverage** (`lex-map`)
- Incrementally wire **parse** → **typecheck** → **codegen** to the Desi lexer
- Reach the first milestone of **“lexer self-hosted”**

---

## What ships in Stage-1

- **Desi lexer** (in repo): `examples/compiler/desi/lexer.desi`
- **Bridge**: tiny wrapper program compiled with Stage-0 toolchain to C, linked via `clang`, then executed to produce tokens
- **Adapter**: converts the Desi lexer’s output into a `lexer.Source` for the Go parser
- **CLI tooling**:
  - `desic lex-desi` — run the Desi lexer and pretty-print tokens
  - `desic lex-diff` — show first N differences between Go and Desi token streams
  - `desic lex-map` — coverage report of Desi→Go token kind mapping
  - `--verbose` flag to surface the bridge’s native compiler output when needed

---

## Architecture (high level)



```
+----------------+


.desi  ---> |   Desi Lexer   |   (implemented in Desi)
+-------+--------+
\| text/NDJSON
v
+-------+--------+             +------------------+
\|  Bridge Wrapper   |   -->   | Stage-0 C code   |
\| (generated .desi) |         |   + runtime      |
+-------+--------+             +------------------+
\| build & run (clang)
v
+-------+--------+
\|   Raw tokens   |
+-------+--------+
\| adapt (map kinds, quote strings)
v
+-------+--------+
\|   Go parser    |
+----------------+

````

Key invariants:

- **No stale fallback** — we only run the bridge binary if the **current** build succeeded.
- **Quiet by default** — native compiler output is suppressed unless `--verbose`.

---

## Commands you’ll use

### Build & basic checks

```bash
go build ./...
go test ./...
```

### Legacy (Go lexer → Go parser)

```bash
# macOS/Linux
go run ./compiler/cmd/desic parse examples/lex_demo.desi | head -n 20

# Windows (PowerShell)
go run .\compiler\cmd\desic parse .\examples\lex_demo.desi | Select-Object -First 20
```

### Desi lexer (via bridge)

```bash
# Pretty tokens
go run ./compiler/cmd/desic lex-desi --format=pretty examples/lex_demo.desi | head -n 30

# Compare Go vs Desi token streams
go run ./compiler/cmd/desic lex-diff --limit=60 examples/lex_demo.desi

# Mapping coverage (which kinds are fully mapped vs TODO)
go run ./compiler/cmd/desic lex-map examples/lex_demo.desi
```

> Add `--verbose` to any of the above to surface clang output if something fails.

---

## Directory map (Stage-1 relevant)

```
compiler/
  cmd/desic/main.go            # CLI: lex-desi / lex-diff / lex-map / parse / build
  internal/
    codegen/c/emitter.go       # printf-based println, strcmp, string concat
    lexbridge/
      helpers.go               # EscapeForDesiString, NDJSON conversion, misc utils
      bridge.go                # BuildAndRunRaw(file, keepTmp, verbose)
      desisource.go            # NewSourceFromFile(...) -> lexer.Source for Go parser
runtime/
  c/desi_std.[ch]              # ARC/str/vec/channel helpers
examples/
  compiler/desi/lexer.desi     # The Desi lexer
  lex_demo.desi
gen/
  out/                         # emitted C + built bridge binary
  tmp/lexbridge/main.desi      # generated wrapper entrypoint
```

---

## Implementation notes

* **String tokens parity**: Desi `STR` text is **re-quoted** in the adapter so it matches the Go lexer contract (string tokens include quotes).
* **`println` lowering**: Codegen builds a single `printf` with a generated format string + argument list (`%s/%d...`) and appends `\n`.
* **String ops**: `==`/`!=` lower to `strcmp(...) == 0/!= 0`; `+` lowers to `desi_str_concat`.
* **Diagnostics**: For now, lexer errors may appear as `ERR` tokens; Stage-1 will route these into the standard diagnostics path.

---

## Platform notes

* **Windows**: ensure LLVM is installed and `clang` is available on PATH. The bridge binary uses `.exe`. We pass `-D_CRT_SECURE_NO_WARNINGS` to hush MSVC CRT deprecation noise when applicable.
* **macOS/Linux**: system `clang` is sufficient.

---

## What’s considered “done” for Stage-1

* [ ] `lex-desi` produces stable token streams for real inputs (compiler sources, examples)
* [ ] `lex-diff` reports zero (or only known) differences for the corpus
* [ ] `lex-map` shows **100% mapped** kinds needed by the Go parser
* [ ] `parse --use-desi-lexer <file>` (or equivalent wiring) parses via the adapter
* [ ] Diagnostics are passed through for `ERR` tokens with file/line/col

---

## Next steps after Stage-1 (toward self-hosting)

1. **Wire the parser cleanly**
   Accept an external `lexer.Source` and make `parse --use-desi-lexer` a stable flag.

2. **NDJSON at the source (optional but recommended)**
   Have `lexer.desi` emit NDJSON directly and delete the conversion shim.

3. **Tests**

* Mapping unit tests (kinds, tricky tokens, INDENT/DEDENT)
* `emitPrintln` format generation tests
* Bridge lifecycle tests (behind `clang` availability guard)

4. **Caching**
   Skip rebuilding the bridge if `lexer.desi` and the input file haven’t changed (fingerprints).

5. **Stage-1 build**
   `desic build --use-desi-lexer <entry.desi>` → end-to-end compile using the Desi lexer.

When the compiler can build itself with `--use-desi-lexer`, we’ll call the **lexer self-hosted** milestone complete and move to parser self-hosting.

