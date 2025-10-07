package parser

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** lookup + tiny utils ***/

func lookupIDTitle(domain, key, fallbackID, fallbackTitle string) (string, string) {
	if info, ok := diag.LookupFull(domain, key); ok {
		id := info.Entry.ID
		if id == "" {
			id = fallbackID
		}
		title := info.Entry.Title
		if title == "" {
			title = fallbackTitle
		}
		return id, title
	}
	return fallbackID, fallbackTitle
}

func tokSpan(t lexer.Token) diag.Span {
	p := diag.Pos{Line: t.Line, Col: t.Col}
	return diag.Span{Start: p, End: p}
}

/*** pretty helpers ***/

func prettyKind(k lexer.TokKind) string {
	switch k {
	case lexer.TokIdent:
		return "identifier"
	case lexer.TokInt:
		return "integer literal"
	case lexer.TokFloat:
		return "float literal"
	case lexer.TokStr:
		return "string literal"
	case lexer.TokTrue, lexer.TokFalse:
		return "boolean literal"
	case lexer.TokRParen:
		return "')'"
	case lexer.TokLParen:
		return "'('"
	case lexer.TokRBrack:
		return "']'"
	case lexer.TokLBrack:
		return "'['"
	case lexer.TokRBrace:
		return "'}'"
	case lexer.TokLBrace:
		return "'{'"
	case lexer.TokComma:
		return "','"
	case lexer.TokColon:
		return "':'"
	case lexer.TokDot:
		return "'.'"
	case lexer.TokArrow:
		return "'->'"
	case lexer.TokEq:
		return "'='"
	case lexer.TokAssign:
		return "':='"
	case lexer.TokPlus:
		return "'+'"
	case lexer.TokMinus:
		return "'-'"
	case lexer.TokStar:
		return "'*'"
	case lexer.TokSlash:
		return "'/'"
	case lexer.TokPercent:
		return "'%'"
	case lexer.TokPipe:
		return "'|>'"
	case lexer.TokBang:
		return "'!'"
	case lexer.TokLt:
		return "'<'"
	case lexer.TokLe:
		return "'<='"
	case lexer.TokGt:
		return "'>'"
	case lexer.TokGe:
		return "'>='"
	case lexer.TokEqEq:
		return "'=='"
	case lexer.TokNe:
		return "'!='"
	case lexer.TokNewline:
		return "newline"
	case lexer.TokEOF:
		return "end of file"
	default:
		return k.String()
	}
}

func prettyGot(tok lexer.Token) string {
	switch tok.Kind {
	case lexer.TokIdent:
		return fmt.Sprintf("identifier %q", tok.Lex)
	case lexer.TokInt:
		return fmt.Sprintf("integer literal %q", tok.Lex)
	case lexer.TokFloat:
		return fmt.Sprintf("float literal %q", tok.Lex)
	case lexer.TokStr:
		return fmt.Sprintf("string literal %q", tok.Lex)
	case lexer.TokTrue, lexer.TokFalse:
		return fmt.Sprintf("boolean literal %q", tok.Lex)
	default:
		return prettyKind(tok.Kind)
	}
}

/*** generic constructor (parser domain) ***/

// ParserErrorAtf builds a parser-domain error by key. The registry title becomes
// the FIRST %s in format (same pattern as TypeErrorf/TypeErrorAtf).
func ParserErrorAtf(at lexer.Token, key, fallbackID, fallbackTitle, format string, args ...any) error {
	id, title := lookupIDTitle("parser", key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf(format, append([]any{title}, args...)...)
	return diag.Diagnostic{
		Domain:  "parser",
		Key:     key,
		Level:   diag.LevelError,
		Code:    id,
		Message: msg,
		Span:    tokSpan(at),
	}
}

/*** constructors (DPE…) implemented via the helper ***/

// DPE0001: unexpected token
func ErrUnexpectedToken(context string, tok lexer.Token) error {
	return ParserErrorAtf(tok, "unexpected_token", "DPE0001", "unexpected token",
		"unexpected token in %s: %s", context, prettyGot(tok))
}

// DPE0002: expected a different token
func ErrExpectedToken(context string, expected lexer.TokKind, got lexer.Token) error {
	return ParserErrorAtf(got, "expected_token", "DPE0002", "expected a different token",
		"expected %s, found %s", prettyKind(expected), prettyGot(got))
}

// DPE0003: unclosed delimiter
func ErrUnclosedDelimiter(context, open, want string, atTok lexer.Token) error {
	return ParserErrorAtf(atTok, "unclosed_delimiter", "DPE0003", "unclosed delimiter",
		"unclosed %s; expected matching %s in %s", open, want, context)
}

// DPE0004: trailing or extra token
func ErrTrailingOrExtraToken(context string, tok lexer.Token) error {
	return ParserErrorAtf(tok, "trailing_or_extra_token", "DPE0004", "trailing or extra token",
		"trailing or extra %s in %s", prettyGot(tok), context)
}

// DPE0005: invalid assignment target
func ErrInvalidAssignmentTarget(context string, tok lexer.Token) error {
	return ParserErrorAtf(tok, "invalid_assignment_target", "DPE0005", "invalid assignment target",
		"invalid assignment target in %s: %s", context, prettyGot(tok))
}

// DPE1001: async only before 'def'
func ErrAsyncOnlyBeforeDef(got lexer.Token) error {
	return ParserErrorAtf(got, "async_before_def", "DPE1001", "async only valid before 'def'",
		"%s")
}

// DPE1002: await requires an expression
func ErrAwaitRequiresExpr(at lexer.Token) error {
	return ParserErrorAtf(at, "await_requires_expr", "DPE1002", "await requires an expression",
		"%s")
}

// DPE0100: unexpected token after 'pub'
func ErrAfterPubUnexpected(got lexer.Token) error {
	return ParserErrorAtf(got, "unexpected_after_pub", "DPE0100", "unexpected token after 'pub'",
		"unexpected token after 'pub': %s", got.Kind.String())
}

// DPE0101: wildcard '_' cannot have a payload
func ErrWildcardHasPayload(at lexer.Token) error {
	return ParserErrorAtf(at, "wildcard_has_payload", "DPE0101", "wildcard '_' cannot have a payload",
		"%s")
}

/*** lexer passthrough — domain=lexer; message is lexer’s own text ***/

func ErrLexerError(tok lexer.Token) error {
	id, _ := lookupIDTitle("lexer", "generic_lexer_error", "DLE0001", "lexer error")
	return diag.Diagnostic{
		Domain:  "lexer",
		Key:     "generic_lexer_error",
		Level:   diag.LevelError,
		Code:    id,
		Message: tok.Lex, // surface the lexer’s own message
		Span:    tokSpan(tok),
	}
}
