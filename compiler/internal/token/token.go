package token

// Token is the enumeration of all lexical tokens in Desi (revised).
// This package defines only names/tables/helpers (no scanning).
type Token int

const (
	// Special/pseudo
	ILLEGAL Token = iota
	EOF
	NL     // logical newline (after layout processing)
	Indent // layout: indent
	Dedent // layout: dedent

	// Identifiers & literals
	IDENT

	// Numeric literals (distinct forms)
	INT_DEC // e.g., 123
	INT_HEX // 0xDEAD
	INT_BIN // 0b1011
	INT_OCT // 0o755

	FLOAT     // 12.34, 2., .5
	FLOAT_EXP // 1e9, 3.14e-2

	// Strings
	STR     // "..."
	LONGSTR // """..."""
	FSTR    // f"..."

	// Keywords
	KW_import
	KW_from
	KW_as
	KW_pub
	KW_def
	KW_async
	KW_class
	KW_struct
	KW_enum
	KW_trait
	KW_impl
	KW_type
	KW_let
	KW_mut
	KW_return
	KW_if
	KW_elif
	KW_else
	KW_while
	KW_for
	KW_in
	KW_using
	KW_defer
	KW_match
	KW_select
	KW_await
	KW_true
	KW_false
	KW_none
	KW_and
	KW_or
	KW_not
	KW_ref
	KW_inout
	KW_unsafe
	KW_break
	KW_continue

	// Delimiters / punctuators
	LPAREN // (
	RPAREN // )
	LBRACK // [
	RBRACK // ]
	LBRACE // {
	RBRACE // }
	COMMA  // ,
	COLON  // :
	DOT    // .
	AT     // @ (decorators)
	HASH   // # (participates in '#{' set opener)

	// Operators
	ASSIGN  // =
	DECLARE // :=
	PLUS    // +
	MINUS   // -
	STAR    // *
	SLASH   // /
	PERCENT // %

	PLUS_EQ    // +=
	MINUS_EQ   // -=
	STAR_EQ    // *=
	SLASH_EQ   // /=
	PERCENT_EQ // %=

	POW    // **
	POW_EQ // **=
	XOR    // ^
	XOR_EQ // ^=

	EQEQ // ==
	NEQ  // !=
	LT   // <
	LTE  // <=
	GT   // >
	GTE  // >=

	IN // 'in' (membership operator in expressions)

	BANG    // !
	PIPE    // |
	PIPE_GT // |>

	ARROW     // ->
	FAT_ARROW // =>
)

// Category classifies tokens into broad kinds (for scanning/parsing/pretty dumps).
type Category int

const (
	CatSpecial Category = iota
	CatLayout
	CatIdent
	CatLiteral
	CatKeyword
	CatOperator
	CatPunct
)

// TokenCategory returns a coarse category for t.
func TokenCategory(t Token) Category {
	switch t {
	case ILLEGAL, EOF:
		return CatSpecial
	case NL, Indent, Dedent:
		return CatLayout
	case IDENT:
		return CatIdent
	case INT_DEC, INT_HEX, INT_BIN, INT_OCT, FLOAT, FLOAT_EXP, STR, LONGSTR, FSTR:
		return CatLiteral
	case KW_import, KW_from, KW_as, KW_pub, KW_def, KW_async, KW_class, KW_struct, KW_enum, KW_trait, KW_impl, KW_type,
		KW_let, KW_mut, KW_return, KW_if, KW_elif, KW_else, KW_while, KW_for, KW_in, KW_using, KW_defer,
		KW_match, KW_select, KW_await, KW_true, KW_false, KW_none, KW_and, KW_or, KW_not, KW_ref, KW_inout,
		KW_unsafe, KW_break, KW_continue:
		return CatKeyword
	case ASSIGN, DECLARE, PLUS, MINUS, STAR, SLASH, PERCENT, PLUS_EQ, MINUS_EQ, STAR_EQ, SLASH_EQ, PERCENT_EQ,
		POW, POW_EQ, XOR, XOR_EQ, EQEQ, NEQ, LT, LTE, GT, GTE, IN, BANG, PIPE, PIPE_GT, ARROW, FAT_ARROW:
		return CatOperator
	case LPAREN, RPAREN, LBRACK, RBRACK, LBRACE, RBRACE, COMMA, COLON, DOT, AT, HASH:
		return CatPunct
	default:
		return CatSpecial
	}
}
