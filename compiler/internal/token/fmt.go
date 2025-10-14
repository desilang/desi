package token

import "strconv"

var tokenNames = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	NL:      "NL",
	Indent:  "Indent",
	Dedent:  "Dedent",

	IDENT: "IDENT",

	INT_DEC:   "INT_DEC",
	INT_HEX:   "INT_HEX",
	INT_BIN:   "INT_BIN",
	INT_OCT:   "INT_OCT",
	FLOAT:     "FLOAT",
	FLOAT_EXP: "FLOAT_EXP",

	STR:     "STR",
	LONGSTR: "LONGSTR",
	FSTR:    "FSTR",

	// Keywords print as their spellings
	KW_import: "import",
	KW_from:   "from",
	KW_as:     "as",
	KW_pub:    "pub",
	KW_def:    "def",
	KW_async:  "async",
	KW_class:  "class",
	KW_struct: "struct",
	KW_enum:   "enum",
	KW_type:   "type",
	KW_let:    "let",
	KW_mut:    "mut",
	KW_return: "return",
	KW_if:     "if",
	KW_elif:   "elif",
	KW_else:   "else",
	KW_while:  "while",
	KW_for:    "for",
	KW_in:     "in",
	KW_using:  "using",
	KW_defer:  "defer",
	KW_match:  "match",
	KW_select: "select",
	KW_await:  "await",
	KW_true:   "true",
	KW_false:  "false",
	KW_none:   "none",
	KW_and:    "and",
	KW_or:     "or",
	KW_not:    "not",

	// Punct: symbolic names
	LPAREN: "LPAREN",
	RPAREN: "RPAREN",
	LBRACK: "LBRACK",
	RBRACK: "RBRACK",
	LBRACE: "LBRACE",
	RBRACE: "RBRACE",
	COMMA:  "COMMA",
	COLON:  "COLON",
	DOT:    "DOT",
	AT:     "AT",
	HASH:   "HASH",

	// Operators: symbolic names
	ASSIGN:     "ASSIGN",
	DECLARE:    "DECLARE",
	PLUS:       "PLUS",
	MINUS:      "MINUS",
	STAR:       "STAR",
	SLASH:      "SLASH",
	PERCENT:    "PERCENT",
	PLUS_EQ:    "PLUS_EQ",
	MINUS_EQ:   "MINUS_EQ",
	STAR_EQ:    "STAR_EQ",
	SLASH_EQ:   "SLASH_EQ",
	PERCENT_EQ: "PERCENT_EQ",
	POW:        "POW",
	POW_EQ:     "POW_EQ",
	XOR:        "XOR",
	XOR_EQ:     "XOR_EQ",
	EQEQ:       "EQEQ",
	NEQ:        "NEQ",
	LT:         "LT",
	LTE:        "LTE",
	GT:         "GT",
	GTE:        "GTE",
	BANG:       "BANG",
	PIPE:       "PIPE",
	PIPE_GT:    "PIPE_GT",
	ARROW:      "ARROW",
	FAT_ARROW:  "FAT_ARROW",
}

// String returns a stable, human-friendly name for the token.
func (t Token) String() string {
	if int(t) >= 0 && int(t) < len(tokenNames) && tokenNames[t] != "" {
		return tokenNames[t]
	}
	return "Token(" + strconv.Itoa(int(t)) + ")"
}
