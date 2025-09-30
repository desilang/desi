package parser

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) parseDottedIdent() (string, error) {
	var parts []string
	t, err := p.expectIdent("module/name")
	if err != nil {
		return "", err
	}
	parts = append(parts, t.Lex)
	for p.accept(lexer.TokDot) {
		t, err := p.expectIdent("module/name")
		if err != nil {
			return "", err
		}
		parts = append(parts, t.Lex)
	}
	return strings.Join(parts, "."), nil
}

func (p *Parser) parseTypeUntil(stoppers ...lexer.TokKind) (string, error) {
	stop := make(map[lexer.TokKind]bool)
	for _, k := range stoppers {
		stop[k] = true
	}
	var b strings.Builder
	depthParen, depthBrack := 0, 0
	for {
		if depthParen == 0 && depthBrack == 0 && stop[p.tok.Kind] {
			break
		}
		switch p.tok.Kind {
		case lexer.TokEOF, lexer.TokNewline, lexer.TokColon:
			return strings.TrimSpace(b.String()), nil
		case lexer.TokLParen:
			depthParen++
		case lexer.TokRParen:
			if depthParen > 0 {
				depthParen--
			}
		case lexer.TokLBrack:
			depthBrack++
		case lexer.TokRBrack:
			if depthBrack > 0 {
				depthBrack--
			}
		}
		if p.tok.Lex != "" {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(p.tok.Lex)
		} else {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(p.tok.Kind.String())
		}
		p.next()
	}
	return strings.TrimSpace(b.String()), nil
}

/*** ------------------ helpers (friendlier ident errors) ------------------ ***/

// expectIdent is like expect(TokIdent) but produces a nicer message if we
// encounter a keyword where an identifier is required.
func (p *Parser) expectIdent(context string) (lexer.Token, error) {
	if p.at(lexer.TokIdent) {
		t := p.tok
		p.next()
		return t, nil
	}
	t := p.tok
	if isKeywordToken(t.Kind) {
		return lexer.Token{}, fmt.Errorf("DPE0002: keyword %q cannot be used as an identifier for %s at %d:%d",
			t.Lex, context, t.Line, t.Col)
	}
	// Fallback to the normal expect error (keeps original formatting/code).
	return p.expect(lexer.TokIdent)
}

func isKeywordToken(k lexer.TokKind) bool {
	switch k {
	case lexer.TokPackage,
		lexer.TokImport,
		lexer.TokFrom,
		lexer.TokAs,
		lexer.TokDef,
		lexer.TokType,
		lexer.TokStruct,
		lexer.TokEnum,
		lexer.TokMut,
		lexer.TokLet,
		lexer.TokIf,
		lexer.TokElif,
		lexer.TokElse,
		lexer.TokWhile,
		lexer.TokReturn,
		lexer.TokMatch:
		return true
	default:
		return false
	}
}
