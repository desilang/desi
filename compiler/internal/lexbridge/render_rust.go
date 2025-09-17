package lexbridge

import (
  "fmt"
  "os"
  "strings"
  "unicode/utf8"
)

func RenderRustStyle(d Diagnostic, srcLoader func(string) ([]byte, error)) string {
  if srcLoader == nil {
    srcLoader = os.ReadFile
  }

  var b strings.Builder
  if d.Code != "" {
    fmt.Fprintf(&b, "%s[%s]: %s\n", d.Level, d.Code, d.Message)
  } else {
    fmt.Fprintf(&b, "%s: %s\n", d.Level, d.Message)
  }
  if d.Primary.File != "" && d.Primary.Line > 0 && d.Primary.Col > 0 {
    fmt.Fprintf(&b, " --> %s:%d:%d\n", d.Primary.File, d.Primary.Line, d.Primary.Col)
  }

  printLineWithUnderlines(&b, d.Primary, d.Secondaries, d.Suggest, srcLoader)

  for _, n := range d.Notes {
    if strings.TrimSpace(n) != "" {
      fmt.Fprintf(&b, "note: %s\n", n)
    }
  }
  if strings.TrimSpace(d.Help) != "" {
    if !strings.HasPrefix(d.Help, "help:") && !strings.HasPrefix(d.Help, "note:") {
      b.WriteString("help: ")
    }
    b.WriteString(d.Help)
    b.WriteByte('\n')
  }
  for _, s := range d.Suggest {
    lead := "help"
    if s.Applicability != "" {
      lead = fmt.Sprintf("help (%s)", s.Applicability)
    }
    msg := s.Message
    if msg == "" && s.Replacement != "" {
      msg = fmt.Sprintf("replace with %q", s.Replacement)
    }
    if msg != "" {
      fmt.Fprintf(&b, "%s: %s\n", lead, msg)
    }
  }
  return b.String()
}

func printLineWithUnderlines(b *strings.Builder, primary Span, secondaries []Span, suggs []Suggestion, loader func(string) ([]byte, error)) {
  if primary.File == "" || primary.Line <= 0 {
    return
  }
  data, err := loader(primary.File)
  if err != nil {
    return
  }
  lineText := getLineText(data, primary.Line)
  lnStr := fmt.Sprintf("%d", primary.Line)
  linePrefix := " " + lnStr + " | "
  underPrefix := " " + strings.Repeat(" ", len(lnStr)) + " | "

  fmt.Fprintf(b, "%s%s\n", linePrefix, lineText)
  b.WriteString(underPrefix)
  writeUnderline(b, lineText, primary.Col, primary.EndCol, primary.Label)
  b.WriteByte('\n')

  for _, s := range secondaries {
    if s.File == primary.File && s.Line == primary.Line {
      b.WriteString(underPrefix)
      writeUnderline(b, lineText, s.Col, s.EndCol, s.Label)
      b.WriteByte('\n')
    }
  }
  for _, sg := range suggs {
    s := sg.At
    if s.File == primary.File && s.Line == primary.Line {
      b.WriteString(underPrefix)
      label := s.Label
      if label == "" {
        if sg.Replacement != "" {
          label = fmt.Sprintf("replace with %q", sg.Replacement)
        } else if sg.Message != "" {
          label = sg.Message
        }
      }
      writeUnderline(b, lineText, s.Col, s.EndCol, label)
      b.WriteByte('\n')
    }
  }
  for _, s := range secondaries {
    if !(s.File == primary.File && s.Line == primary.Line) {
      printMiniBlock(b, s, loader)
    }
  }
  for _, sg := range suggs {
    s := sg.At
    if !(s.File == primary.File && s.Line == primary.Line) {
      printMiniBlock(b, s, loader)
    }
  }
}

func printMiniBlock(b *strings.Builder, sp Span, loader func(string) ([]byte, error)) {
  if sp.File == "" || sp.Line <= 0 {
    return
  }
  data, err := loader(sp.File)
  if err != nil {
    return
  }
  lineText := getLineText(data, sp.Line)
  lnStr := fmt.Sprintf("%d", sp.Line)
  linePrefix := " " + lnStr + " | "
  underPrefix := " " + strings.Repeat(" ", len(lnStr)) + " | "
  fmt.Fprintf(b, "%s%s\n", linePrefix, lineText)
  b.WriteString(underPrefix)
  writeUnderline(b, lineText, sp.Col, sp.EndCol, sp.Label)
  b.WriteByte('\n')
}

func writeUnderline(b *strings.Builder, line string, col, endCol int, label string) {
  vis := visualize(line)
  start := clamp(col-1, 0, len(vis))
  end := start
  if endCol > 0 && endCol > col {
    end = clamp(endCol-1, start+1, len(vis))
  } else if start < len(vis) {
    end = start + 1
  }
  b.WriteString(strings.Repeat(" ", start))
  if end-start <= 1 {
    b.WriteString("^")
  } else {
    b.WriteString("^")
    b.WriteString(strings.Repeat("~", end-start-1))
  }
  if strings.TrimSpace(label) != "" {
    b.WriteString(" ")
    b.WriteString(label)
  }
}

func getLineText(src []byte, line int) string {
  if line <= 0 {
    return ""
  }
  cur := 1
  start := 0
  for i, b := range src {
    if b == '\n' {
      if cur == line {
        return string(src[start:i])
      }
      cur++
      start = i + 1
    }
  }
  if cur == line && start <= len(src) {
    return string(src[start:])
  }
  return ""
}

func visualize(s string) []rune {
  const tabw = 4
  var vis []rune
  for len(s) > 0 {
    r, sz := utf8.DecodeRuneInString(s)
    if r == '\t' {
      for i := 0; i < tabw; i++ {
        vis = append(vis, ' ')
      }
    } else if r == utf8.RuneError && sz == 1 {
      vis = append(vis, '�')
    } else {
      vis = append(vis, r)
    }
    s = s[sz:]
  }
  return vis
}

func clamp(v, lo, hi int) int {
  if v < lo {
    return lo
  }
  if v > hi {
    return hi
  }
  return v
}
