package token

// Operator table: multi-char first for greedy matching in a future scanner.
var operators = []struct {
	Lit   string
	Token Token
}{
	// 3-char first
	{"**=", POW_EQ},
	{"<<=", LSHIFT_EQ},
	{">>=", RSHIFT_EQ},

	// 2-char
	{":=", DECLARE},
	{"==", EQEQ},
	{"!=", NEQ},
	{"<=", LTE},
	{">=", GTE},
	{"+=", PLUS_EQ},
	{"-=", MINUS_EQ},
	{"*=", STAR_EQ},
	{"/=", SLASH_EQ},
	{"%=", PERCENT_EQ},
	{"^=", XOR_EQ},
	{"&=", AMP_EQ},
	{"|=", PIPE_EQ},
	{"<<", LSHIFT},
	{">>", RSHIFT},
	{"**", POW},
	{"|>", PIPE_GT},
	{"->", ARROW},
	{"=>", FAT_ARROW},

	// 1-char
	{"=", ASSIGN},
	{"+", PLUS},
	{"-", MINUS},
	{"*", STAR},
	{"/", SLASH},
	{"%", PERCENT},
	{"<", LT},
	{">", GT},
	{"!", BANG},
	{"|", PIPE},
	{"^", XOR},
	{"&", AMP},
	{"~", TILDE},
	{"(", LPAREN},
	{")", RPAREN},
	{"[", LBRACK},
	{"]", RBRACK},
	{"{", LBRACE},
	{"}", RBRACE},
	{",", COMMA},
	{":", COLON},
	{".", DOT},
	{"@", AT},
	{"#", HASH},
	{"?", QUESTION},
}

// IsOperator reports whether a token is an operator (not punctuation).
func IsOperator(t Token) bool {
	switch t {
	case ASSIGN, DECLARE, PLUS, MINUS, STAR, SLASH, PERCENT,
		PLUS_EQ, MINUS_EQ, STAR_EQ, SLASH_EQ, PERCENT_EQ,
		POW, POW_EQ, XOR, XOR_EQ,
		EQEQ, NEQ, LT, LTE, GT, GTE, BANG, PIPE, PIPE_GT, ARROW, FAT_ARROW,
		TILDE, AMP, AMP_EQ, PIPE_EQ, LSHIFT, LSHIFT_EQ, RSHIFT, RSHIFT_EQ,
		QUESTION:
		return true
	default:
		return false
	}
}

// ---------- Exports for lexers ----------

// OperatorLits returns operator/punctuator literals in greedy order (longest first).
func OperatorLits() []string {
	out := make([]string, len(operators))
	for i, p := range operators {
		out[i] = p.Lit
	}
	return out
}

// OperatorByLit returns the token for a literal (if any).
func OperatorByLit(lit string) (Token, bool) {
	for _, p := range operators {
		if p.Lit == lit {
			return p.Token, true
		}
	}
	return ILLEGAL, false
}
