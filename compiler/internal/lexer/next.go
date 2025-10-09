package lexer

import "unicode"

// Next returns the next token. It never panics on user input.
func (lx *Lexer) Next() Token {
  // Emit any queued tokens first
  if n := len(lx.pending); n > 0 {
    t := lx.pending[0]
    lx.pending = lx.pending[1:]
    return t
  }

  // Handle indentation if at beginning of a logical line
  if lx.bol {
    lx.handleBOL()
    if n := len(lx.pending); n > 0 {
      t := lx.pending[0]
      lx.pending = lx.pending[1:]
      return t
    }
  }

  // EOF: unwind remaining indents, then emit EOF
  if lx.atEOF() {
    if !lx.eofEmitted {
      // Safety: ensure indent stack unwound
      for len(lx.indents) > 1 {
        lx.indents = lx.indents[:len(lx.indents)-1]
        return lx.make(TokDedent, "", lx.line, lx.col)
      }
      lx.eofEmitted = true
    }
    return lx.make(TokEOF, "", lx.line, lx.col)
  }

  // Skip mid-line spaces/tabs
  for {
    ch, ok := lx.peek()
    if !ok {
      break
    }
    if ch == ' ' || ch == '\t' {
      lx.advance()
      continue
    }
    break
  }

  startLine, startCol := lx.line, lx.col+1

  // Line continuation: a backslash immediately before a newline suppresses the newline.
  // Support both "\n" and "\r\n".
  for {
    ch, ok := lx.peek()
    if !ok || ch != '\\' {
      break
    }
    // consume '\'
    if !lx.match('\\') {
      break
    }
    // if next is CR, consume it and check for following LF
    if ch2, ok2 := lx.peek(); ok2 && ch2 == '\r' {
      lx.advance()
      if ch3, ok3 := lx.peek(); ok3 && ch3 == '\n' {
        lx.advance()
        // after continuation, skip any spaces/tabs on the next physical line
        for {
          if ch4, ok4 := lx.peek(); ok4 && (ch4 == ' ' || ch4 == '\t') {
            lx.advance()
            continue
          }
          break
        }
        continue // allow chaining
      }
      // lone '\r' line continuation also allowed: treat as newline suppression
      // after continuation, skip spaces/tabs
      for {
        if ch4, ok4 := lx.peek(); ok4 && (ch4 == ' ' || ch4 == '\t') {
          lx.advance()
          continue
        }
        break
      }
      continue
    }
    // plain '\n'
    if lx.match('\n') {
      for {
        if ch4, ok4 := lx.peek(); ok4 && (ch4 == ' ' || ch4 == '\t') {
          lx.advance()
          continue
        }
        break
      }
      continue
    }
    // not followed by a newline → treat '\' as unknown; fall through
    break
  }

  // Newline terminates a statement, emit NEWLINE and go to BOL
  if ch, ok := lx.peek(); ok && (ch == '\n' || ch == '\r') {
    // if CRLF, consume both; else consume the single newline char
    if lx.match('\r') {
      if next, ok2 := lx.peek(); ok2 && next == '\n' {
        lx.advance()
      }
    } else {
      lx.advance() // '\n'
    }
    lx.bol = true
    return lx.make(TokNewline, "", startLine, startCol)
  }

  // Comment mid-line: consume to EOL, then emit NEWLINE
  if ch, ok := lx.peek(); ok && ch == '#' {
    for {
      ch, ok := lx.peek()
      if !ok || ch == '\n' || ch == '\r' {
        break
      }
      lx.advance()
    }
    // consume newline (CRLF or single CR/LF) if present
    if ch, ok := lx.peek(); ok && (ch == '\n' || ch == '\r') {
      if lx.match('\r') {
        if next, ok2 := lx.peek(); ok2 && next == '\n' {
          lx.advance()
        }
      } else {
        lx.advance()
      }
      lx.bol = true
      return lx.make(TokNewline, "", startLine, startCol)
    }
    // EOF after comment
    return lx.make(TokEOF, "", lx.line, lx.col)
  }

  // Identifiers / keywords
  if ch, ok := lx.peek(); ok && (isIdentStart(ch)) {
    lex := lx.scanIdent()
    if kind, ok := keywordKind(lex); ok {
      return lx.make(kind, lex, startLine, startCol)
    }
    return lx.make(TokIdent, lex, startLine, startCol)
  }

  // Numbers: decimal, hex/bin, optional fraction+exponent
  if ch, ok := lx.peek(); ok && unicode.IsDigit(ch) {
    lex, kind := lx.scanNumberWithOptionalFractionAndExp()
    return lx.make(kind, lex, startLine, startCol)
  }

  // Strings (simple "..." with basic escapes)
  if ch, ok := lx.peek(); ok && ch == '"' {
    lex, closed := lx.scanString()
    if !closed {
      // Do not consume the newline here; let the normal flow emit NEWLINE next.
      return lx.make(TokErr, "unterminated string literal", startLine, startCol)
    }
    return lx.make(TokStr, lex, startLine, startCol)
  }

  // Multi-char operators first
  if lx.match(':') {
    if lx.match('=') {
      return lx.make(TokAssign, ":=", startLine, startCol)
    }
    return lx.make(TokColon, ":", startLine, startCol)
  }
  if lx.match('-') {
    // Prefer '->' then '-=' then bare '-'
    if lx.match('>') {
      return lx.make(TokArrow, "->", startLine, startCol)
    }
    if lx.match('=') {
      return lx.make(TokMinusEq, "-=", startLine, startCol)
    }
    return lx.make(TokMinus, "-", startLine, startCol)
  }
  if lx.match('=') {
    if lx.match('=') {
      return lx.make(TokEqEq, "==", startLine, startCol)
    }
    return lx.make(TokEq, "=", startLine, startCol)
  }
  if lx.match('!') {
    if lx.match('=') {
      return lx.make(TokNe, "!=", startLine, startCol)
    }
    return lx.make(TokBang, "!", startLine, startCol)
  }
  if lx.match('<') {
    if lx.match('=') {
      return lx.make(TokLe, "<=", startLine, startCol)
    }
    return lx.make(TokLt, "<", startLine, startCol)
  }
  if lx.match('>') {
    if lx.match('=') {
      return lx.make(TokGe, ">=", startLine, startCol)
    }
    return lx.make(TokGt, ">", startLine, startCol)
  }
  if lx.match('|') {
    if lx.match('>') {
      return lx.make(TokPipe, "|>", startLine, startCol)
    }
    // Unknown bare '|': Stage-0—emit TokPipe anyway
    return lx.make(TokPipe, "|", startLine, startCol)
  }

  // Single-char punctuation (with compound-assign checks)
  if lx.match('+') {
    if lx.match('=') {
      return lx.make(TokPlusEq, "+=", startLine, startCol)
    }
    return lx.make(TokPlus, "+", startLine, startCol)
  }
  if lx.match('*') {
    if lx.match('=') {
      return lx.make(TokStarEq, "*=", startLine, startCol)
    }
    return lx.make(TokStar, "*", startLine, startCol)
  }
  if lx.match('/') {
    if lx.match('=') {
      return lx.make(TokSlashEq, "/=", startLine, startCol)
    }
    return lx.make(TokSlash, "/", startLine, startCol)
  }
  if lx.match('%') {
    return lx.make(TokPercent, "%", startLine, startCol)
  }
  if lx.match('(') {
    return lx.make(TokLParen, "(", startLine, startCol)
  }
  if lx.match(')') {
    return lx.make(TokRParen, ")", startLine, startCol)
  }
  if lx.match('[') {
    return lx.make(TokLBrack, "[", startLine, startCol)
  }
  if lx.match(']') {
    return lx.make(TokRBrack, "]", startLine, startCol)
  }
  if lx.match('{') {
    return lx.make(TokLBrace, "{", startLine, startCol)
  }
  if lx.match('}') {
    return lx.make(TokRBrace, "}", startLine, startCol)
  }
  if lx.match('.') {
    return lx.make(TokDot, ".", startLine, startCol)
  }
  if lx.match(',') {
    return lx.make(TokComma, ",", startLine, startCol)
  }

  // Unknown character: skip it and continue (Stage-0 lenient)
  lx.advance()
  return lx.Next()
}
