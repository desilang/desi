# desifmt — Code Formatter

## Overview

`desifmt` (also available as `desic fmt`) is the canonical code formatter for Desi.
It uses a **scan–rewrite** architecture: parse to validate syntax, then re-scan
tokens to deterministically rewrite only whitespace/trivia.

## Source Location

| File | Purpose |
|------|---------|
| `compiler/internal/format/format.go` | Core rewrite engine |
| `compiler/cmd/desifmt/main.go` | CLI entry point |
| `compiler/internal/format/testdata/` | Golden test fixtures |

## Architecture

```
Source Bytes
    │
    ▼
parse.ParseFile()          ← bail early on syntax errors
    │
    ▼
lex.NewScanner()           ← fresh scan (tokens carry line/col)
    │
    ▼
rewrite(src)               ← walk tokens, emit formatted output
    │
    ▼
Formatted Bytes
```

### Token Processing Loop (`rewrite`)

The main loop in `rewrite()` processes tokens from the scanner:

| Token | Action |
|-------|--------|
| `EOF` | Emit trailing comment lines, flush |
| `NL` | Emit skipped comment lines, attach EOL comments, emit newline |
| `Indent` | Increment indent level |
| `Dedent` | Decrement indent level |
| Default | Emit skipped comments, spacing, then the token itself |

### Key Design Decisions

1. **Tabs-only indentation**: The formatter always outputs `\t` for indentation.
   The scanner enforces this too (`lexer.tabs_only_indentation` diagnostic).

2. **String literal preservation**: String tokens (STR, LONGSTR, RAWSTR, FSTR_*)
   are always reconstructed from the original source bytes via
   `reconstructStringLiteral()`. This preserves quotes, escape sequences, and
   f-string delimiters. Numeric literals use the scanner's `Lexeme` directly.

3. **Comment preservation**: The scanner skips comment-only lines entirely
   (returns `isCommentLine=true`). The formatter re-emits them by:
   - Tracking `lastSourceLine` to detect gaps between tokens
   - Checking skipped lines via `isCommentOnlyLine()`
   - Preserving original tab indentation for idempotent round-tripping
   - Suppressing duplicate newlines when NL handler follows comment emission

4. **EOL comments**: Trailing comments on code lines (e.g., `x = 1  # note`)
   are preserved via `findEOLCommentSuffix()`.

## Modifying the Formatter

### Adding New Token Types

If you add a new token category:

1. Add a case in the `switch cat` block in `rewrite()`
2. Ensure `shouldSpaceBefore()` handles spacing rules
3. If it's a literal with delimiters (like strings), use source reconstruction

### Comment Handling Gotchas

The scanner reports NL tokens for comment-only lines with `Line = commentLine + 1`
(after advancing past the newline). This means:

- `emitSkippedCommentLines(currentLine)` checks lines `[lastSourceLine+1, currentLine)`
- The helper advances `lastSourceLine` to `currentLine - 1` (not `currentLine`)
- Code-token and EOF callsites explicitly set `lastSourceLine = it.Line`
- The NL handler skips its `w.nl()` when comments were emitted on a non-code line

### Testing

```bash
# Run formatter tests only
go test ./compiler/internal/format/...

# Update golden files (manual: format input, save as .golden.desi)
./bin/desifmt testdata/phase1/foo.in.desi > testdata/phase1/foo.golden.desi

# Verify idempotency
./bin/desifmt file.desi > /tmp/pass1.desi
./bin/desifmt /tmp/pass1.desi > /tmp/pass2.desi
diff /tmp/pass1.desi /tmp/pass2.desi  # must be empty
```

### Common Failure Patterns

| Symptom | Likely Cause |
|---------|-------------|
| Quotes stripped from strings | `CatLiteral` handler using raw `Lexeme` instead of `reconstructStringLiteral()` |
| Comments dropped | `emitSkippedCommentLines()` not called in the right token handler |
| Extra blank lines around comments | NL handler not suppressing `w.nl()` when comments were emitted |
| Wrong indentation on comments | Using `w.writeIndent()` instead of preserving original tab count |
| Idempotency failures | `lastSourceLine` advancement doesn't match token line semantics |
