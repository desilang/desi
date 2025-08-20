package lexer

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
)

// Token is a single lexeme with source position.
type Token struct {
	Kind TokKind
	Lex  string
	Line int
	Col  int
}
