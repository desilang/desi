package lexbridge

import (
  "errors"
  "fmt"
  "strings"

  "github.com/desilang/desi/compiler/internal/lexer"
)

// desiSource replays a pre-mapped slice of Go tokens (lexer.TokKind)
// produced from the Desi NDJSON stream.
type desiSource struct {
  toks []lexer.Token
  i    int
}

func (s *desiSource) Next() lexer.Token {
  if s.i >= len(s.toks) {
    // Safety: once drained, always return EOF
    return lexer.Token{Kind: lexer.TokEOF}
  }
  t := s.toks[s.i]
  s.i++
  return t
}

// NewSourceFromFile builds & runs the Desi lexer bridge on <file> and returns a
// lexer.Source that yields Go-lexer-compatible tokens to the parser.
func NewSourceFromFile(file string) (lexer.Source, error) {
  raw, err := BuildAndRunRaw(file, false, false)
  if err != nil {
    return nil, err
  }
  nd := ConvertRawToNDJSON(raw, true)

  rows, perr := ParseNDJSON(strings.NewReader(nd))
  if perr != nil {
    // Non-fatal: proceed with whatever we parsed; caller will see parser errors if any.
  }

  var mapped []lexer.Token
  var lexers []string

  for _, r := range rows {
    if r.Kind == "ERR" {
      lexers = append(lexers, fmt.Sprintf("LEXERR line=%d col=%d msg=%q", r.Line, r.Col, r.Text))
      continue
    }
    gk, ok := mapDesiToTokKind(r.Kind, r.Text)
    if !ok {
      return nil, fmt.Errorf("desi-adapter: unmapped token kind=%q text=%q at %d:%d", r.Kind, r.Text, r.Line, r.Col)
    }

    lexeme := r.Text
    // Our Go lexer returns string literals INCLUDING quotes; Desi stream gives unquoted text.
    if gk == lexer.TokStr {
      lexeme = quoteCLike(r.Text)
    }

    mapped = append(mapped, lexer.Token{
      Kind: gk,
      Lex:  lexeme,
      Line: r.Line,
      Col:  r.Col,
    })
  }

  if len(lexers) > 0 {
    return nil, errors.New(strings.Join(lexers, "\n"))
  }

  // Inject a NEWLINE before any DEDENT if the previous token wasn't a NEWLINE.
  // This handles blocks where the last statement line has no trailing newline.
  if len(mapped) > 0 {
    fixed := make([]lexer.Token, 0, len(mapped)+4)
    for _, t := range mapped {
      if t.Kind == lexer.TokDedent {
        if len(fixed) > 0 && fixed[len(fixed)-1].Kind != lexer.TokNewline {
          fixed = append(fixed, lexer.Token{
            Kind: lexer.TokNewline,
            Line: t.Line,
            Col:  t.Col,
          })
        }
      }
      fixed = append(fixed, t)
    }
    mapped = fixed
  }

  // Ensure trailing EOF (Desi emits one, but double-check).
  if n := len(mapped); n == 0 || mapped[n-1].Kind != lexer.TokEOF {
    mapped = append(mapped, lexer.Token{Kind: lexer.TokEOF})
  }

  return &desiSource{toks: mapped}, nil
}

// mapDesiToTokKind converts the Desi stream's (Kind,Text) into our Go lexer TokKind.
func mapDesiToTokKind(kind, text string) (lexer.TokKind, bool) {
  switch kind {
  // structural
  case "EOF":
    return lexer.TokEOF, true
  case "NEWLINE":
    return lexer.TokNewline, true
  case "INDENT":
    return lexer.TokIndent, true
  case "DEDENT":
    return lexer.TokDedent, true

  // identifiers & literals
  case "IDENT":
    return lexer.TokIdent, true
  case "INT":
    return lexer.TokInt, true
  case "STR":
    return lexer.TokStr, true

  // keywords are emitted as kind=KW, text=<word>
  case "KW":
    switch text {
    case "let":
      return lexer.TokLet, true
    case "mut":
      return lexer.TokMut, true
    case "def":
      return lexer.TokDef, true
    case "return":
      return lexer.TokReturn, true
    case "if":
      return lexer.TokIf, true
    case "elif":
      return lexer.TokElif, true
    case "else":
      return lexer.TokElse, true
    case "while":
      return lexer.TokWhile, true
    case "for":
      return lexer.TokFor, true
    case "in":
      return lexer.TokIn, true
    case "match":
      return lexer.TokMatch, true
    case "struct":
      return lexer.TokStruct, true
    case "enum":
      return lexer.TokEnum, true
    case "package":
      return lexer.TokPackage, true
    case "import":
      return lexer.TokImport, true
    case "as":
      return lexer.TokAs, true
    case "true":
      return lexer.TokTrue, true
    case "false":
      return lexer.TokFalse, true
    case "and":
      return lexer.TokAnd, true
    case "or":
      return lexer.TokOr, true
    case "not":
      return lexer.TokNot, true
    case "defer":
      return lexer.TokDefer, true
    default:
      return 0, false
    }

  // punctuation / operators
  case "EQ":
    return lexer.TokEq, true
  case "ASSIGN":
    return lexer.TokAssign, true
  case "PLUS":
    return lexer.TokPlus, true
  case "MINUS":
    return lexer.TokMinus, true
  case "STAR":
    return lexer.TokStar, true
  case "SLASH":
    return lexer.TokSlash, true
  case "PERCENT":
    return lexer.TokPercent, true
  case "LPAREN":
    return lexer.TokLParen, true
  case "RPAREN":
    return lexer.TokRParen, true
  case "LBRACK":
    return lexer.TokLBrack, true
  case "RBRACK":
    return lexer.TokRBrack, true
  case "DOT":
    return lexer.TokDot, true
  case "COLON":
    return lexer.TokColon, true
  case "COMMA":
    return lexer.TokComma, true
  case "ARROW":
    return lexer.TokArrow, true
  case "PIPE":
    return lexer.TokPipe, true
  case "BANG":
    return lexer.TokBang, true
  case "LT":
    return lexer.TokLt, true
  case "LE":
    return lexer.TokLe, true
  case "GT":
    return lexer.TokGt, true
  case "GE":
    return lexer.TokGe, true
  case "EQEQ":
    return lexer.TokEqEq, true
  case "NE":
    return lexer.TokNe, true
  default:
    return 0, false
  }
}

// quoteCLike returns a double-quoted literal with escapes suitable for our pipeline.
func quoteCLike(s string) string {
  var b strings.Builder
  b.WriteByte('"')
  for _, r := range s {
    switch r {
    case '\\':
      b.WriteString(`\\`)
    case '"':
      b.WriteString(`\"`)
    case '\n':
      b.WriteString(`\n`)
    case '\r':
      b.WriteString(`\r`)
    case '\t':
      b.WriteString(`\t`)
    default:
      if r < 0x20 {
        o1 := ((r >> 6) & 7) + '0'
        o2 := ((r >> 3) & 7) + '0'
        o3 := (r & 7) + '0'
        b.WriteByte('\\')
        b.WriteByte(byte(o1))
        b.WriteByte(byte(o2))
        b.WriteByte(byte(o3))
      } else {
        b.WriteRune(r)
      }
    }
  }
  b.WriteByte('"')
  return b.String()
}
