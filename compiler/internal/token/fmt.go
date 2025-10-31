package token

import "strconv"

// String() returns a symbolic, stable name (great for dumps/tests).
// Lit() returns the canonical source spelling for tokens that have a fixed literal.
// Some tokens (IDENT, INT_*, FLOAT_*, STR, layout) have no single literal → Lit() == "".

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

	// Keywords: show source spellings in String()
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
	KW_ref:    "ref",
	KW_inout:  "inout",
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

	// Punct/ops: symbolic names for String()
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

// Canonical literal spellings when they exist.
// Empty string means "no single literal" for that token kind.
var tokenLits = map[Token]string{
	// Keywords (source spellings)
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
	KW_ref:    "ref",
	KW_inout:  "inout",
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

	// Punct
	LPAREN: "(",
	RPAREN: ")",
	LBRACK: "[",
	RBRACK: "]",
	LBRACE: "{",
	RBRACE: "}",
	COMMA:  ",",
	COLON:  ":",
	DOT:    ".",
	AT:     "@",
	HASH:   "#",

	// Operators
	ASSIGN:     "=",
	DECLARE:    ":=",
	PLUS:       "+",
	MINUS:      "-",
	STAR:       "*",
	SLASH:      "/",
	PERCENT:    "%",
	PLUS_EQ:    "+=",
	MINUS_EQ:   "-=",
	STAR_EQ:    "*=",
	SLASH_EQ:   "/=",
	PERCENT_EQ: "%=",
	POW:        "**",
	POW_EQ:     "**=",
	XOR:        "^",
	XOR_EQ:     "^=",
	EQEQ:       "==",
	NEQ:        "!=",
	LT:         "<",
	LTE:        "<=",
	GT:         ">",
	GTE:        ">=",
	BANG:       "!",
	PIPE:       "|",
	PIPE_GT:    "|>",
	ARROW:      "->",
	FAT_ARROW:  "=>",
}

func (t Token) String() string {
	if int(t) >= 0 && int(t) < len(tokenNames) && tokenNames[t] != "" {
		return tokenNames[t]
	}
	return "Token(" + strconv.Itoa(int(t)) + ")"
}

// Lit returns the canonical source literal (if any) for this token kind.
// Returns "" for tokens that don't have a fixed literal form (e.g., IDENT, INT_DEC, STR, layout).
func (t Token) Lit() string {
	if s, ok := tokenLits[t]; ok {
		return s
	}
	return ""
}
