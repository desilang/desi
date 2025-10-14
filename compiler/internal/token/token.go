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
	INT
	FLOAT
	STR     // "..."
	LONGSTR // """..."""
	FSTR    // f"..."

	// Keywords (Rev-6)
	KW_import
	KW_from
	KW_as
	KW_pub
	KW_def
	KW_async
	KW_class
	KW_struct
	KW_enum
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
	ASSIGN     // =
	DECLARE    // := (statement assignment)
	PLUS       // +
	MINUS      // -
	STAR       // *
	SLASH      // /
	PERCENT    // %
	PLUS_EQ    // +=
	MINUS_EQ   // -=
	STAR_EQ    // *=
	SLASH_EQ   // /=
	PERCENT_EQ // %=
	EQEQ       // ==
	NEQ        // !=
	LT         // <
	LTE        // <=
	GT         // >
	GTE        // >=
	BANG       // !
	PIPE       // |   (type unions)
	PIPE_GT    // |>  (pipeline)
	ARROW      // ->  (return type)
	FAT_ARROW  // =>  (lambda)
)

// Category labels for formatting/debug UI.
type Category int

const (
	CatSpecial Category = iota
	CatLayout
	CatIdent
	CatLiteral
	CatKeyword
	CatBuiltinType
	CatOperator
	CatPunct
)

// TokenCategory provides a coarse grouping useful in dumps/formatters.
func TokenCategory(t Token) Category {
	switch t {
	case ILLEGAL, EOF:
		return CatSpecial
	case NL, Indent, Dedent:
		return CatLayout
	case IDENT:
		return CatIdent
	case INT, FLOAT, STR, LONGSTR, FSTR:
		return CatLiteral
	case KW_import, KW_from, KW_as, KW_pub, KW_def, KW_async, KW_class, KW_struct, KW_enum, KW_type,
		KW_let, KW_mut, KW_return, KW_if, KW_elif, KW_else, KW_while, KW_for, KW_in, KW_using, KW_defer,
		KW_match, KW_select, KW_await, KW_true, KW_false, KW_none, KW_and, KW_or, KW_not:
		return CatKeyword
	case ASSIGN, DECLARE, PLUS, MINUS, STAR, SLASH, PERCENT, PLUS_EQ, MINUS_EQ, STAR_EQ, SLASH_EQ, PERCENT_EQ,
		EQEQ, NEQ, LT, LTE, GT, GTE, BANG, PIPE, PIPE_GT, ARROW, FAT_ARROW:
		return CatOperator
	case LPAREN, RPAREN, LBRACK, RBRACK, LBRACE, RBRACE, COMMA, COLON, DOT, AT, HASH:
		return CatPunct
	default:
		return CatSpecial
	}
}
