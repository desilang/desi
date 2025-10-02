package lexer

// keywordKind maps identifiers to keyword tokens.
func keywordKind(s string) (TokKind, bool) {
	switch s {
	case "let":
		return TokLet, true
	case "mut":
		return TokMut, true
	case "def":
		return TokDef, true
	case "return":
		return TokReturn, true
	case "if":
		return TokIf, true
	case "elif":
		return TokElif, true
	case "else":
		return TokElse, true
	case "while":
		return TokWhile, true
	case "for":
		return TokFor, true
	case "in":
		return TokIn, true
	case "match":
		return TokMatch, true
	case "struct":
		return TokStruct, true
	case "enum":
		return TokEnum, true
	case "package":
		return TokPackage, true
	case "import":
		return TokImport, true
	case "from":
		return TokFrom, true
	case "as":
		return TokAs, true
	case "type":
		return TokType, true
	case "pub":
		return TokPub, true
	case "async":
		return TokAsync, true
	case "await":
		return TokAwait, true
	case "true":
		return TokTrue, true
	case "false":
		return TokFalse, true
	case "and":
		return TokAnd, true
	case "or":
		return TokOr, true
	case "not":
		return TokNot, true
	case "defer":
		return TokDefer, true
	// Reserved-but-forbidden word:
	case "void":
		// We expose a distinct token so later stages can emit a nice diagnostic:
		// "‘void’ is not a Desi type; use ‘none’".
		return TokVoid, true
	default:
		return 0, false
	}
}
