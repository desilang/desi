package token

// Operator table: multi-char first for greedy matching in a future scanner.
var operators = []struct {
	Lit   string
	Token Token
}{
	// 2-char and 3-char
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
}

// IsOperator reports whether a token is an operator (not punctuation).
func IsOperator(t Token) bool {
	switch t {
	case ASSIGN, DECLARE, PLUS, MINUS, STAR, SLASH, PERCENT,
		PLUS_EQ, MINUS_EQ, STAR_EQ, SLASH_EQ, PERCENT_EQ,
		EQEQ, NEQ, LT, LTE, GT, GTE, BANG, PIPE, PIPE_GT, ARROW, FAT_ARROW:
		return true
	default:
		return false
	}
}
