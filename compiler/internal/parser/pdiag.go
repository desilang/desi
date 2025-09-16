package parser

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lexer"
)

// parseError carries a catalog code and a rendered message.
type parseError struct {
	code   string
	title  string
	domain string
	key    string
}

func (e parseError) Error() string {
	if e.code != "" {
		return e.code + ": " + e.title
	}
	return e.title
}

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

// ErrUnexpectedToken => DPE0001
func ErrUnexpectedToken(context string, tok lexer.Token) error {
	id, title := lookupIDTitle("parser", "unexpected_token", "DPE0001", "unexpected token")
	msg := fmt.Sprintf("%s in %s at %d:%d: got %s", title, context, tok.Line, tok.Col, tok.Kind.String())
	return parseError{code: id, title: msg, domain: "parser", key: "unexpected_token"}
}

// ErrExpectedToken => DPE0002
func ErrExpectedToken(context string, expected lexer.TokKind, got lexer.Token) error {
	id, title := lookupIDTitle("parser", "expected_token", "DPE0002", "expected a different token")
	msg := fmt.Sprintf("%s in %s at %d:%d: expected %s, got %s",
		title, context, got.Line, got.Col, expected.String(), got.Kind.String())
	return parseError{code: id, title: msg, domain: "parser", key: "expected_token"}
}

// ErrUnclosedDelimiter => DPE0003
func ErrUnclosedDelimiter(context string, open string, want string, atTok lexer.Token) error {
	id, title := lookupIDTitle("parser", "unclosed_delimiter", "DPE0003", "unclosed delimiter")
	msg := fmt.Sprintf("%s in %s near %d:%d: opened with %s but did not find %s",
		title, context, atTok.Line, atTok.Col, open, want)
	return parseError{code: id, title: msg, domain: "parser", key: "unclosed_delimiter"}
}

// ErrTrailingOrExtraToken => DPE0004
func ErrTrailingOrExtraToken(context string, tok lexer.Token) error {
	id, title := lookupIDTitle("parser", "trailing_or_extra_token", "DPE0004", "trailing or extra token")
	msg := fmt.Sprintf("%s in %s at %d:%d: %s", title, context, tok.Line, tok.Col, tok.Kind.String())
	return parseError{code: id, title: msg, domain: "parser", key: "trailing_or_extra_token"}
}

// ErrInvalidAssignmentTarget => DPE0005
func ErrInvalidAssignmentTarget(context string, tok lexer.Token) error {
	id, title := lookupIDTitle("parser", "invalid_assignment_target", "DPE0005", "invalid assignment target")
	msg := fmt.Sprintf("%s in %s at %d:%d: token %s", title, context, tok.Line, tok.Col, tok.Kind.String())
	return parseError{code: id, title: msg, domain: "parser", key: "invalid_assignment_target"}
}
