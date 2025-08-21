package parser

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

type Parser struct {
	lx  *lexer.Lexer
	tok lexer.Token
}

func New(src string) *Parser {
	p := &Parser{lx: lexer.New(src)}
	p.next()
	return p
}

func (p *Parser) next()                   { p.tok = p.lx.Next() }
func (p *Parser) at(k lexer.TokKind) bool { return p.tok.Kind == k }
func (p *Parser) accept(k lexer.TokKind) bool {
	if p.at(k) {
		p.next()
		return true
	}
	return false
}
func (p *Parser) expect(k lexer.TokKind) (lexer.Token, error) {
	if !p.at(k) {
		return p.tok, fmt.Errorf("expected %v, got %v at %d:%d", k, p.tok.Kind, p.tok.Line, p.tok.Col)
	}
	t := p.tok
	p.next()
	return t, nil
}
func (p *Parser) skipNewlines() {
	for p.accept(lexer.TokNewline) {
	}
}

func (p *Parser) ParseFile() (*ast.File, error) {
	f := &ast.File{}
	p.skipNewlines()

	// package (optional)
	if p.accept(lexer.TokPackage) {
		name, err := p.parseDottedIdent()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		f.Pkg = &ast.PackageDecl{Name: name}
		p.skipNewlines()
	}

	// imports
	for p.accept(lexer.TokImport) {
		path, err := p.parseDottedIdent()
		if err != nil {
			return nil, err
		}
		// Optional: grouped form or alias left for later
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		f.Imports = append(f.Imports, ast.ImportDecl{Path: path})
		p.skipNewlines()
	}

	// decls
	for !p.at(lexer.TokEOF) {
		switch {
		case p.accept(lexer.TokDef):
			fn, err := p.parseFuncDecl()
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, fn)
		default:
			// Skip unexpected tokens to next newline
			for !p.at(lexer.TokNewline) && !p.at(lexer.TokEOF) {
				p.next()
			}
			p.skipNewlines()
		}
	}
	return f, nil
}

func (p *Parser) parseDottedIdent() (string, error) {
	var parts []string
	t, err := p.expect(lexer.TokIdent)
	if err != nil {
		return "", err
	}
	parts = append(parts, t.Lex)
	for p.accept(lexer.TokDot) {
		t, err := p.expect(lexer.TokIdent)
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
		// stop if current token is a stopper and we're not nested
		if depthParen == 0 && depthBrack == 0 && stop[p.tok.Kind] {
			break
		}
		switch p.tok.Kind {
		case lexer.TokEOF, lexer.TokNewline, lexer.TokColon: // safety
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

func (p *Parser) parseFuncDecl() (*ast.FuncDecl, error) {
	// def <name> "(" params? ")" "->" type ":" NEWLINE INDENT stmts DEDENT
	nameTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}

	var params []ast.Param
	if !p.accept(lexer.TokRParen) {
		for {
			id, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokColon); err != nil {
				return nil, err
			}
			ty, err := p.parseTypeUntil(lexer.TokComma, lexer.TokRParen)
			if err != nil {
				return nil, err
			}
			params = append(params, ast.Param{Name: id.Lex, Type: ty})
			if p.accept(lexer.TokComma) {
				continue
			}
			_, err = p.expect(lexer.TokRParen)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	// return type
	if _, err := p.expect(lexer.TokArrow); err != nil {
		return nil, err
	}
	ret, err := p.parseTypeUntil(lexer.TokColon)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokColon); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokNewline); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokIndent); err != nil {
		return nil, err
	}

	var body []ast.Stmt
	for !p.at(lexer.TokDedent) && !p.at(lexer.TokEOF) {
		p.skipNewlines()
		if p.at(lexer.TokDedent) || p.at(lexer.TokEOF) {
			break
		}
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		body = append(body, s)
	}

	if _, err := p.expect(lexer.TokDedent); err != nil {
		return nil, err
	}

	return &ast.FuncDecl{
		Name:   nameTok.Lex,
		Params: params,
		Ret:    ret,
		Body:   body,
	}, nil
}

func (p *Parser) parseStmt() (ast.Stmt, error) {
	switch {
	case p.accept(lexer.TokLet):
		mut := p.accept(lexer.TokMut)
		id, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokEq); err != nil {
			return nil, err
		}
		expr := p.readExprToEOL()
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.LetStmt{Mutable: mut, Name: id.Lex, Expr: expr}, nil
	case p.at(lexer.TokIdent):
		// Could be assignment or expr stmt
		save := p.tok
		p.next()
		if p.at(lexer.TokAssign) {
			// name := expr
			p.next()
			expr := p.readExprToEOL()
			if _, err := p.expect(lexer.TokNewline); err != nil {
				return nil, err
			}
			return &ast.AssignStmt{Name: save.Lex, Expr: expr}, nil
		}
		// fallback: expr starts with that ident
		head := tokenText(save)
		rest := p.readExprToEOL()
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.ExprStmt{Expr: strings.TrimSpace(head + " " + rest)}, nil
	case p.accept(lexer.TokReturn):
		// return [expr]
		if p.at(lexer.TokNewline) {
			p.next()
			return &ast.ReturnStmt{Expr: ""}, nil
		}
		expr := p.readExprToEOL()
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.ReturnStmt{Expr: expr}, nil
	default:
		// generic expr stmt
		expr := p.readExprToEOL()
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.ExprStmt{Expr: expr}, nil
	}
}

func (p *Parser) readExprToEOL() string {
	var b strings.Builder
	depthParen, depthBrack := 0, 0
	for {
		if p.at(lexer.TokEOF) || (p.at(lexer.TokNewline) && depthParen == 0 && depthBrack == 0) {
			break
		}
		switch p.tok.Kind {
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
	return strings.TrimSpace(b.String())
}

func tokenText(t lexer.Token) string {
	if t.Lex != "" {
		return t.Lex
	}
	return t.Kind.String()
}
