package lexer

import "strconv"

// TokKind enumerates token kinds produced by the lexer.
// Stage-0 subset; we'll add more as grammar lands.
type TokKind int

const (
  // Special
  TokEOF     TokKind = iota
  TokNewline         // logical newline
  TokIndent          // indent block
  TokDedent          // dedent block

  // Literals/identifiers
  TokIdent
  TokInt
  TokFloat
  TokStr

  // Keywords (Stage-0)
  TokLet
  TokMut
  TokDef
  TokReturn
  TokIf
  TokElif
  TokElse
  TokWhile
  TokFor
  TokIn
  TokMatch
  TokStruct
  TokEnum
  TokPackage
  TokImport
  TokAs

  // Operators/punctuation
  TokEq      // =
  TokAssign  // :=
  TokPlus    // +
  TokMinus   // -
  TokStar    // *
  TokSlash   // /
  TokPercent // %
  TokLParen  // (
  TokRParen  // )
  TokLBrack  // [
  TokRBrack  // ]
  TokDot     // .
  TokColon   // :
  TokComma   // ,
  TokArrow   // ->

  TokPipe // |>
  TokBang // !
  TokLt   // <
  TokLe   // <=
  TokGt   // >
  TokGe   // >=
  TokEqEq // ==
  TokNe   // !=

  // Boolean & logical words
  TokTrue
  TokFalse
  TokAnd
  TokOr
  TokNot
  TokDefer // NEW
)

// Token is a single lexeme with source position.
type Token struct {
  Kind TokKind
  Lex  string
  Line int
  Col  int
}

func (k TokKind) String() string {
  switch k {
  case TokEOF:
    return "EOF"
  case TokNewline:
    return "NEWLINE"
  case TokIndent:
    return "INDENT"
  case TokDedent:
    return "DEDENT"
  case TokIdent:
    return "IDENT"
  case TokInt:
    return "INT"
  case TokFloat:
    return "FLOAT"
  case TokStr:
    return "STR"
  case TokLet:
    return "let"
  case TokMut:
    return "mut"
  case TokDef:
    return "def"
  case TokReturn:
    return "return"
  case TokIf:
    return "if"
  case TokElif:
    return "elif"
  case TokElse:
    return "else"
  case TokWhile:
    return "while"
  case TokFor:
    return "for"
  case TokIn:
    return "in"
  case TokMatch:
    return "match"
  case TokStruct:
    return "struct"
  case TokEnum:
    return "enum"
  case TokPackage:
    return "package"
  case TokImport:
    return "import"
  case TokAs:
    return "as"
  case TokEq:
    return "="
  case TokAssign:
    return ":="
  case TokPlus:
    return "+"
  case TokMinus:
    return "-"
  case TokStar:
    return "*"
  case TokSlash:
    return "/"
  case TokPercent:
    return "%"
  case TokLParen:
    return "("
  case TokRParen:
    return ")"
  case TokLBrack:
    return "["
  case TokRBrack:
    return "]"
  case TokDot:
    return "."
  case TokColon:
    return ":"
  case TokComma:
    return ","
  case TokArrow:
    return "->"
  case TokPipe:
    return "|>"
  case TokBang:
    return "!"
  case TokLt:
    return "<"
  case TokLe:
    return "<="
  case TokGt:
    return ">"
  case TokGe:
    return ">="
  case TokEqEq:
    return "=="
  case TokNe:
    return "!="
  case TokTrue:
    return "true"
  case TokFalse:
    return "false"
  case TokAnd:
    return "and"
  case TokOr:
    return "or"
  case TokNot:
    return "not"
  case TokDefer:
    return "defer"
  default:
    return "TokKind(" + strconv.Itoa(int(k)) + ")"
  }
}
