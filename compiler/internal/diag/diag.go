package diag

import (
  "fmt"
  "strings"

  "github.com/desilang/desi/compiler/internal/term"
)

// Pos marks a 1-based line/column location in a file.
type Pos struct{ Line, Col int }

// Span marks a half-open range [Start, End) within a file.
type Span struct {
  Start Pos
  End   Pos
}

// Level indicates severity.
type Level string

const (
  LevelError   Level = "error"
  LevelWarning Level = "warning"
  LevelNote    Level = "note"
)

// SecondarySpan is an additional span with an optional label (for notes, hints).
type SecondarySpan struct {
  Span  Span
  Label string
}

// Diagnostic is a compiler message with structured info.
type Diagnostic struct {
  Domain      string          // "lexer" | "parser" | "type" | "other"
  Key         string          // catalog key, e.g. "undefined_name"
  Code        string          // optional override (else taken from catalog outside)
  Level       Level           // error | warning | note
  Message     string          // short title/message
  Span        Span            // primary span (optional)
  Secondaries []SecondarySpan // optional
  Notes       []string        // optional extra lines
}

// String implements fmt.Stringer (compact single-line fallback).
func (d Diagnostic) String() string {
  loc := ""
  if d.Span.Start.Line > 0 {
    loc = fmt.Sprintf("%d:%d: ", d.Span.Start.Line, d.Span.Start.Col)
  }
  code := d.Code
  if code != "" {
    code += ": "
  }
  return fmt.Sprintf("%s%s%s%s", loc, strings.ToLower(string(d.Level)), ": ", code+d.Message)
}

// Error implements error (preserves old behavior for easy logging).
func (d Diagnostic) Error() string { return d.String() }

/*
Minimal renderer (TTY-agnostic, no colors yet).
CLI can call Render() with a function to fetch the source line for a given 1-based line number.

Example:

	Render(diag, filename, func(line int) (string, bool) { ... })
*/
func Render(d Diagnostic, file string, lineGetter func(int) (string, bool)) string {
  var b strings.Builder

  // Header line
  code := d.Code
  if code != "" {
    term.Bprintf(&b, "%s[%s]: %s\n", strings.ToLower(string(d.Level)), code, d.Message)
  } else {
    term.Bprintf(&b, "%s: %s\n", strings.ToLower(string(d.Level)), d.Message)
  }

  // Primary span snippet
  if d.Span.Start.Line > 0 {
    if src, ok := lineGetter(d.Span.Start.Line); ok {
      term.Bprintf(&b, " --> %s:%d:%d\n", file, d.Span.Start.Line, d.Span.Start.Col)
      // Source line
      b.WriteString("  |\n")
      term.Bprintf(&b, "  | %s\n", src)

      // Carets underline: from Col to End.Col (fallback to Col if End empty)
      col1 := d.Span.Start.Col
      col2 := d.Span.End.Col
      if col2 <= 0 || col2 < col1 {
        col2 = col1
      }
      if col1 < 1 {
        col1 = 1
      }
      if col2 < col1 {
        col2 = col1
      }
      var underline strings.Builder
      for i := 1; i < col1; i++ {
        underline.WriteByte(' ')
      }
      underline.WriteByte('^')
      for i := col1 + 1; i <= col2; i++ {
        underline.WriteByte('^')
      }
      term.Bprintf(&b, "  | %s\n", underline.String())
    }
  }

  // Secondary spans
  for _, s := range d.Secondaries {
    if s.Span.Start.Line == 0 {
      continue
    }
    if src, ok := lineGetter(s.Span.Start.Line); ok {
      term.Bprintf(&b, "  = note: at %s:%d:%d", file, s.Span.Start.Line, s.Span.Start.Col)
      if s.Label != "" {
        term.Bprintf(&b, " — %s", s.Label)
      }
      b.WriteByte('\n')
      term.Bprintf(&b, "  | %s\n", src)
      // simple caret for secondary
      var underline strings.Builder
      for i := 1; i < s.Span.Start.Col; i++ {
        underline.WriteByte(' ')
      }
      underline.WriteByte('^')
      term.Bprintf(&b, "  | %s\n", underline.String())
    }
  }

  // Notes
  for _, n := range d.Notes {
    term.Bprintf(&b, "  = note: %s\n", n)
  }

  return b.String()
}
