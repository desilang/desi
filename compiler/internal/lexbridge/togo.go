package lexbridge

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/lexer"
)

// MapToGoKind converts one Desi (kind,text) into a concrete Go lexer.TokKind.
// Returns (TokKind, true) if mapped, otherwise (TokEOF, false).
func MapToGoKind(desiKind, text string) (lexer.TokKind, bool) {
	switch desiKind {
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
			return lexer.TokEOF, false
		}

	// Non-keyword kinds
	case "IDENT":
		return lexer.TokIdent, true
	case "INT":
		return lexer.TokInt, true
	case "STR":
		return lexer.TokStr, true
	case "NEWLINE":
		return lexer.TokNewline, true
	case "INDENT":
		return lexer.TokIndent, true
	case "DEDENT":
		return lexer.TokDedent, true

	// Punctuation and operators
	case "DOT":
		return lexer.TokDot, true
	case "LPAREN":
		return lexer.TokLParen, true
	case "RPAREN":
		return lexer.TokRParen, true
	case "LBRACK":
		return lexer.TokLBrack, true // reserved if Desi ever emits it
	case "RBRACK":
		return lexer.TokRBrack, true // reserved if Desi ever emits it
	case "COLON":
		return lexer.TokColon, true
	case "COMMA":
		return lexer.TokComma, true
	case "EQ":
		return lexer.TokEq, true
	case "ASSIGN":
		return lexer.TokAssign, true
	case "ARROW":
		return lexer.TokArrow, true
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
	case "PIPE":
		return lexer.TokPipe, true
	case "BANG":
		return lexer.TokBang, true // if Desi ever emits it
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
	case "EOF":
		return lexer.TokEOF, true
	default:
		return lexer.TokEOF, false
	}
}

// ToGoTokens converts a Desi token stream into a Go-lexer token slice.
// On unmapped tokens, it returns an error with the first offending index.
func ToGoTokens(desi []Token) ([]lexer.Token, error) {
	out := make([]lexer.Token, 0, len(desi))
	for i, t := range desi {
		gk, ok := MapToGoKind(t.Kind, t.Text)
		if !ok {
			return nil, fmt.Errorf("unmapped token at %d: kind=%q text=%q line=%d col=%d",
				i, t.Kind, t.Text, t.Line, t.Col)
		}
		out = append(out, lexer.Token{
			Kind: gk,
			Lex:  t.Text,
			Line: t.Line,
			Col:  t.Col,
		})
	}
	// Safety: ensure EOF exists (most streams should already end with EOF)
	if n := len(out); n == 0 || out[n-1].Kind != lexer.TokEOF {
		out = append(out, lexer.Token{Kind: lexer.TokEOF})
	}
	return out, nil
}
