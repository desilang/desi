package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lex"
	"github.com/desilang/desi/compiler/internal/token"
)

type Parser struct {
	sc    *lex.Scanner
	file  string
	cur   lex.Item
	peek  lex.Item
	diags []diag.Diagnostic
}

// ParseFile is the public entry point.
func ParseFile(filename string, src []byte) (*ast.Module, []diag.Diagnostic) {
	sc := lex.NewScannerWithFile(src, filename)
	p := &Parser{sc: sc, file: filename}
	p.next() // fill cur
	p.next() // fill peek

	m := &ast.Module{File: filename, Span: spanPos(filename, p.cur)}
	// Top-level: handle decorators + funcs; if loose stmts exist (examples), gather them under __top__.
	for p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.EOF {
			break
		}
		// Function can start with decorators, 'async', or 'def'
		if p.cur.Tok == token.AT || p.cur.Tok == token.KW_def || p.cur.Tok == token.KW_async {
			if d := p.parseFunc(); d != nil {
				m.Decls = append(m.Decls, d)
			}
			continue
		}

		top := &ast.FuncDecl{
			Name: ast.Ident{Name: "__top__", Span: spanPos(filename, p.cur)},
			Body: &ast.Block{Span: spanPos(filename, p.cur)},
		}
		for p.cur.Tok != token.EOF && p.cur.Tok != token.KW_def && p.cur.Tok != token.KW_async && p.cur.Tok != token.AT {
			p.skipNLs()
			if p.cur.Tok == token.EOF || p.cur.Tok == token.KW_def || p.cur.Tok == token.KW_async || p.cur.Tok == token.AT {
				break
			}
			if s := p.parseStmt(); s != nil {
				top.Body.Stmts = append(top.Body.Stmts, s)
			} else {
				p.syncStmt()
			}
			p.skipNLs()
		}
		m.Decls = append(m.Decls, top)
	}
	m.Span = ast.JoinSpan(m.Span, spanPos(filename, p.cur))
	return m, p.diags
}

/* ---------- scanner glue & small helpers ---------- */

func (p *Parser) next()                 { p.cur, p.peek = p.peek, p.sc.Next() }
func (p *Parser) at(t token.Token) bool { return p.cur.Tok == t }
func (p *Parser) accept(t token.Token) bool {
	if p.at(t) {
		p.next()
		return true
	}
	return false
}
func (p *Parser) expect(t token.Token, label string) bool {
	if p.accept(t) {
		return true
	}
	p.errExpected(spanPos(p.file, p.cur), label)
	return false
}

// expectClose emits DPE0003 (unclosed delimiter) tied to the span of the opener.
func (p *Parser) expectClose(closeTok token.Token, label string, open diag.Span) bool {
	if p.accept(closeTok) {
		return true
	}
	p.errUnclosed(open, label)
	return false
}

func (p *Parser) skipNLs() {
	for p.cur.Tok == token.NL {
		p.next()
	}
}
func (p *Parser) syncStmt() {
	for p.cur.Tok != token.NL && p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.next()
	}
	if p.cur.Tok == token.NL {
		p.next()
	}
}

/* ---------- span helpers ---------- */

func spanPos(file string, it lex.Item) diag.Span {
	return diag.Span{
		File:  file,
		Start: diag.Pos{Line: it.Line, Col: it.Col},
		End:   diag.Pos{Line: it.Line, Col: it.Col},
	}
}
func lastSpan(n ast.Node, fallback diag.Span) diag.Span {
	if n == nil {
		return fallback
	}
	return n.SpanOf()
}
func joinTok(file string, a, b lex.Item) diag.Span {
	return diag.Span{
		File:  file,
		Start: diag.Pos{Line: a.Line, Col: a.Col},
		End:   diag.Pos{Line: b.Line, Col: b.Col},
	}
}
