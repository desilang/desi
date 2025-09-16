package parser

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** Expressions (Pratt parser) ***/

func (p *Parser) parseExpr() (ast.Expr, error) {
	// Surface lexer errors immediately inside expressions
	if p.at(lexer.TokErr) {
		t := p.tok
		return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
	}
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	return p.parseBinaryRHS(1, left)
}

func (p *Parser) parseExprWithLHS(lhs ast.Expr) (ast.Expr, error) {
	if p.at(lexer.TokErr) {
		t := p.tok
		return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
	}
	post, err := p.parsePostfix(lhs)
	if err != nil {
		return nil, err
	}
	return p.parseBinaryRHS(1, post)
}

func (p *Parser) parseUnary() (ast.Expr, error) {
	// Surface lexer errors if a unary operator position contains TokErr
	if p.at(lexer.TokErr) {
		t := p.tok
		return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
	}
	switch {
	case p.accept(lexer.TokMinus):
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "-", X: x}, nil
	case p.accept(lexer.TokBang):
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "!", X: x}, nil
	case p.accept(lexer.TokNot):
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "not", X: x}, nil
	default:
		return p.parsePrimary()
	}
}

func (p *Parser) parsePrimary() (ast.Expr, error) {
	// Surface lexer errors at primary positions (e.g., unterminated string)
	if p.at(lexer.TokErr) {
		t := p.tok
		return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
	}
	if p.at(lexer.TokIdent) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.IdentExpr{Name: t.Lex})
	}
	if p.at(lexer.TokInt) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.IntLit{Value: t.Lex})
	}
	if p.at(lexer.TokStr) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.StrLit{Value: t.Lex})
	}
	if p.at(lexer.TokTrue) {
		p.next()
		return p.parsePostfix(&ast.BoolLit{Value: true})
	}
	if p.at(lexer.TokFalse) {
		p.next()
		return p.parsePostfix(&ast.BoolLit{Value: false})
	}
	if p.accept(lexer.TokLParen) {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokRParen); err != nil {
			return nil, err
		}
		return p.parsePostfix(e)
	}
	return nil, fmt.Errorf("unexpected token in expression: %v at %d:%d", p.tok.Kind, p.tok.Line, p.tok.Col)
}

func (p *Parser) parsePostfix(base ast.Expr) (ast.Expr, error) {
	e := base
	for {
		switch {
		case p.accept(lexer.TokLParen):
			// arguments with optional trailing comma
			var args []ast.Expr
			if !p.accept(lexer.TokRParen) {
				for {
					if p.at(lexer.TokRParen) {
						p.next()
						break
					}
					// Allow TokErr to bubble as a cleaner message (e.g., unterminated string)
					if p.at(lexer.TokErr) {
						t := p.tok
						return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
					}
					a, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.accept(lexer.TokComma) {
						continue
					}
					if _, err := p.expect(lexer.TokRParen); err != nil {
						return nil, err
					}
					break
				}
			}
			e = &ast.CallExpr{Callee: e, Args: args}
		case p.accept(lexer.TokLBrack):
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokRBrack); err != nil {
				return nil, err
			}
			e = &ast.IndexExpr{Seq: e, Index: idx}
		case p.accept(lexer.TokDot):
			id, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			e = &ast.FieldExpr{X: e, Name: id.Lex}
		default:
			return e, nil
		}
	}
}

func (p *Parser) parseBinaryRHS(minPrec int, left ast.Expr) (ast.Expr, error) {
	for {
		prec, ok := binPrec(p.tok.Kind)
		if !ok || prec < minPrec {
			return left, nil
		}
		opTok := p.tok
		p.next()

		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}

		for {
			nextPrec, ok := binPrec(p.tok.Kind)
			if !ok || nextPrec <= prec {
				break
			}
			right, err = p.parseBinaryRHS(prec+1, right)
			if err != nil {
				return nil, err
			}
		}

		left = &ast.BinaryExpr{
			Op:    opTok.Kind.String(),
			Left:  left,
			Right: right,
		}
	}
}
