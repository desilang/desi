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
	ahead []lex.Item
}

func ParseFile(filename string, src []byte) (*ast.Module, []diag.Diagnostic) {
	sc := lex.NewScannerWithFile(src, filename)
	p := &Parser{sc: sc, file: filename}
	p.next()
	p.next()

	m := &ast.Module{File: filename, Span: spanPos(filename, p.cur)}
	for p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.EOF {
			break
		}

		switch p.cur.Tok {
		case token.AT:
			// Decorators may precede class/struct/enum/function declarations.
			decs := p.parseDecorators()
			p.skipNLs()
			switch {
			case p.cur.Tok == token.KW_class || (p.cur.Tok == token.KW_pub && p.peek.Tok == token.KW_class):
				if c := p.parseClassWithDecs(decs, false /*nested*/); c != nil {
					m.Decls = append(m.Decls, c)
				}
			case p.cur.Tok == token.KW_struct || (p.cur.Tok == token.KW_pub && p.peek.Tok == token.KW_struct):
				if s := p.parseStructWithDecs(decs); s != nil {
					m.Decls = append(m.Decls, s)
				}
			case p.cur.Tok == token.KW_enum || (p.cur.Tok == token.KW_pub && p.peek.Tok == token.KW_enum):
				if e := p.parseEnumWithDecs(decs); e != nil {
					m.Decls = append(m.Decls, e)
				}
			case p.cur.Tok == token.KW_trait || (p.cur.Tok == token.KW_pub && p.peek.Tok == token.KW_trait):
				if t := p.parseTrait(decs); t != nil {
					m.Decls = append(m.Decls, t)
				}
			case p.cur.Tok == token.KW_impl:
				if i := p.parseImpl(decs); i != nil {
					m.Decls = append(m.Decls, i)
				}
			default:
				if f := p.parseFuncWithDecs(decs); f != nil {
					m.Decls = append(m.Decls, f)
				}
			}
			continue

		case token.KW_class:
			if c := p.parseClassWithDecs(nil, false /*nested*/); c != nil {
				m.Decls = append(m.Decls, c)
			}
			continue

		case token.KW_struct:
			if s := p.parseStructWithDecs(nil); s != nil {
				m.Decls = append(m.Decls, s)
			}
			continue

		case token.KW_enum:
			if e := p.parseEnumWithDecs(nil); e != nil {
				m.Decls = append(m.Decls, e)
			}
			continue

		case token.KW_type:
			if t := p.parseTypeAlias(false); t != nil {
				m.Decls = append(m.Decls, t)
			}
			continue

		case token.KW_trait:
			if t := p.parseTrait(nil); t != nil {
				m.Decls = append(m.Decls, t)
			}
			continue

		case token.KW_impl:
			if i := p.parseImpl(nil); i != nil {
				m.Decls = append(m.Decls, i)
			}
			continue

		case token.KW_pub:
			// NEW: allow 'pub def' (and 'pub async def') at top-level
			switch p.peek.Tok {
			case token.KW_class:
				if c := p.parseClassWithDecs(nil, false /*nested*/); c != nil {
					m.Decls = append(m.Decls, c)
				}
				continue
			case token.KW_struct:
				if s := p.parseStructWithDecs(nil); s != nil {
					m.Decls = append(m.Decls, s)
				}
				continue
			case token.KW_enum:
				if e := p.parseEnumWithDecs(nil); e != nil {
					m.Decls = append(m.Decls, e)
				}
				continue
			case token.KW_trait:
				if t := p.parseTrait(nil); t != nil {
					m.Decls = append(m.Decls, t)
				}
				continue
			case token.KW_type:
				if t := p.parseTypeAlias(true); t != nil {
					m.Decls = append(m.Decls, t)
				}
				continue
			case token.KW_def, token.KW_async:
				if f := p.parseFuncWithDecs(nil); f != nil {
					m.Decls = append(m.Decls, f)
				}
				continue
			default:
				// let function parsing diagnose invalid pub usage elsewhere
			}

		case token.KW_def, token.KW_async:
			if f := p.parseFunc(); f != nil {
				m.Decls = append(m.Decls, f)
			}
			continue
		}

		// Fallback: hoist loose stmts into __top__ block.
		top := &ast.FuncDecl{
			Name: ast.Ident{Name: "__top__", Span: spanPos(filename, p.cur)},
			Body: &ast.Block{Span: spanPos(filename, p.cur)},
			Span: spanPos(filename, p.cur),
		}
		for p.cur.Tok != token.EOF &&
			p.cur.Tok != token.KW_def && p.cur.Tok != token.KW_async &&
			p.cur.Tok != token.KW_class && p.cur.Tok != token.KW_struct && p.cur.Tok != token.KW_enum &&
			p.cur.Tok != token.KW_trait && p.cur.Tok != token.KW_impl &&
			p.cur.Tok != token.KW_pub && p.cur.Tok != token.AT {
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

// --- small helpers (no diagnostics here; see diag_core.go) ---
func (p *Parser) next() {
	p.cur = p.peek
	if len(p.ahead) > 0 {
		p.peek = p.ahead[0]
		p.ahead = p.ahead[1:]
	} else {
		p.peek = p.sc.Next()
	}
}

func (p *Parser) accept(tok token.Token) bool {
	if p.cur.Tok == tok {
		p.next()
		return true
	}
	return false
}
func (p *Parser) expect(tok token.Token, label string) bool {
	if p.cur.Tok == tok {
		p.next()
		return true
	}
	p.errExpected(spanPos(p.file, p.cur), label)
	return false
}

func (p *Parser) expectClose(tok token.Token, label string, open diag.Span) bool {
	if p.cur.Tok == tok {
		p.next()
		return true
	}
	p.errUnclosed(open, label)
	return false
}

// expectTypeGT expects a '>' in type context, handling '>>' (RSHIFT) specially.
// For nested generics like Box<Box<int>>, the lexer produces '>>' as RSHIFT.
// This method treats RSHIFT as two '>' tokens, consuming one and pushing
// a synthetic '>' for the outer type parameter to consume.
func (p *Parser) expectTypeGT() bool {
	if p.cur.Tok == token.GT {
		p.next()
		return true
	}
	if p.cur.Tok == token.RSHIFT {
		// Consume '>>' but pretend we only consumed one '>'
		// Push a synthetic '>' token onto the ahead buffer
		syntheticGT := lex.Item{
			Tok:    token.GT,
			Lexeme: ">",
			Line:   p.cur.Line,
			Col:    p.cur.Col + 1, // shift column by 1
		}
		// Prepend to ahead buffer so it's consumed next
		p.ahead = append([]lex.Item{p.peek}, p.ahead...)
		p.peek = syntheticGT
		p.cur = lex.Item{} // mark as consumed
		p.next()           // advance to the synthetic >
		return true
	}
	p.errExpected(spanPos(p.file, p.cur), ">")
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

func spanPos(file string, it lex.Item) diag.Span {
	return diag.Span{
		File:  file,
		Start: diag.Pos{Line: it.Line, Col: it.Col},
		End:   diag.Pos{Line: it.Line, Col: it.Col},
	}
}

func joinTok(file string, a, b lex.Item) diag.Span {
	return diag.Span{
		File:  file,
		Start: diag.Pos{Line: a.Line, Col: a.Col},
		End:   diag.Pos{Line: b.Line, Col: b.Col},
	}
}
