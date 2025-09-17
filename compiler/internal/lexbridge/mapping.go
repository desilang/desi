package lexbridge

import "github.com/desilang/desi/compiler/internal/lexer"

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
