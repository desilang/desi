package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseBlock parses an Indent/Dedent-delimited block, then post-processes
// the first statement: if it is a triple-quoted string literal (LONGSTR),
// we convert it into a DocStringStmt node. Later, decl parsers (e.g., functions)
// may *attach* that docstring to the decl metadata and remove it from the body.
func (p *Parser) parseBlock() *ast.Block {
	start := spanPos(p.file, p.cur) // Indent
	_ = p.expect(token.Indent, "indent")

	var stmts []ast.Stmt
	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}
		stmts = append(stmts, p.parseStmt())
	}
	_ = p.expect(token.Dedent, "dedent")

	// Block span: conservative join from first stmt to last stmt (or empty block)
	if len(stmts) == 0 {
		return &ast.Block{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	}
	return &ast.Block{Stmts: stmts, Span: ast.JoinSpan(stmts[0].SpanOf(), stmts[len(stmts)-1].SpanOf())}
}

// parseStmt dispatches statement forms. One-line forms (if/while/for) parse
// their SimpleStmt inline without requiring Indent/Dedent.
func (p *Parser) parseStmt() ast.Stmt {
	switch p.cur.Tok {
	case token.KW_let:
		return p.parseLet()
	case token.KW_return:
		return p.parseReturn()
	case token.KW_if:
		return p.parseIf()
	case token.KW_while:
		return p.parseWhile()
	case token.KW_for:
		return p.parseFor()
	case token.KW_using:
		return p.parseUsing()
	case token.KW_defer:
		return p.parseDefer()
	case token.KW_match:
		return p.parseMatch()
	case token.KW_import: // NEW
		return p.parseImport()
	case token.KW_from: // NEW
		return p.parseFromImport()
	default:
		e := p.parseExpr()
		if s := p.maybeMakeAssign(e); s != nil {
			return s
		}
		// Expression statement: require newline (tolerate EOF/Dedent)
		span := ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))
		if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		return &ast.ExprStmt{Expr: e, Span: span}
	}
}

func (p *Parser) parseLet() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'let'
	p.next()                        // consume 'let'

	lhs := p.parseExpr()
	// (Assign target validation is deferred to the checker; parser only rejects
	// clearly invalid LHS shapes via maybeMakeAssign / errInvalidAssignTarget.)
	if !p.expect(token.ASSIGN, "=") {
		p.syncStmt()
		return nil
	}

	val := p.parseExpr()
	span := ast.JoinSpan(start, lastSpan(val, spanPos(p.file, p.cur)))
	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}
	return &ast.AssignStmt{
		Kind:  ast.AssignLet,
		Left:  []ast.Expr{lhs},
		Right: []ast.Expr{val},
		Span:  span,
	}
}

func (p *Parser) parseReturn() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'return'
	p.next()                        // consume 'return'

	var e ast.Expr
	if p.cur.Tok != token.NL && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		e = p.parseExpr()
	}
	span := ast.JoinSpan(start, lastSpan(e, spanPos(p.file, p.cur)))
	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}
	return &ast.ReturnStmt{Result: e, Span: span}
}
