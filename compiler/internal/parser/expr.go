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
	case p.at(lexer.TokMinus):
		op := p.tok
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "-", X: x, Span: spanFrom(posFrom(op), exprEnd(x))}, nil

	case p.at(lexer.TokBang):
		op := p.tok
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "!", X: x, Span: spanFrom(posFrom(op), exprEnd(x))}, nil

	case p.at(lexer.TokNot):
		op := p.tok
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "not", X: x, Span: spanFrom(posFrom(op), exprEnd(x))}, nil

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
		// Postfix handles calls, indexes, fields **and** struct literals via "{"
		return p.parsePostfix(&ast.IdentExpr{Name: t.Lex, Span: spanTok(t, t)})
	}
	if p.at(lexer.TokInt) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.IntLit{Value: t.Lex, Span: spanTok(t, t)})
	}
	if p.at(lexer.TokStr) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.StrLit{Value: t.Lex, Span: spanTok(t, t)})
	}
	if p.at(lexer.TokTrue) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.BoolLit{Value: true, Span: spanTok(t, t)})
	}
	if p.at(lexer.TokFalse) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.BoolLit{Value: false, Span: spanTok(t, t)})
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
	// Registry-backed parser error
	return nil, ErrUnexpectedToken("expression", p.tok)
}

func (p *Parser) parsePostfix(base ast.Expr) (ast.Expr, error) {
	e := base
	for {
		switch {
		case p.accept(lexer.TokLParen):
			// arguments with optional trailing comma
			var args []ast.Expr
			var rparen lexer.Token

			if p.at(lexer.TokRParen) {
				rparen = p.tok
				p.next()
			} else {
				for {
					if p.at(lexer.TokRParen) {
						rparen = p.tok
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
					rtok, err := p.expect(lexer.TokRParen)
					if err != nil {
						return nil, err
					}
					rparen = rtok
					break
				}
			}
			e = &ast.CallExpr{
				Callee: e,
				Args:   args,
				Span:   spanFrom(exprStart(e), endPosFrom(rparen)),
			}

		case p.accept(lexer.TokLBrack):
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			rb, err := p.expect(lexer.TokRBrack)
			if err != nil {
				return nil, err
			}
			e = &ast.IndexExpr{
				Seq:   e,
				Index: idx,
				Span:  spanFrom(exprStart(e), endPosFrom(rb)),
			}

		case p.accept(lexer.TokDot):
			id, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			e = &ast.FieldExpr{
				X:    e,
				Name: id.Lex,
				Span: spanFrom(exprStart(e), endPosFrom(id)),
			}

		// NEW: Struct literal after an Ident/type name:  User{ field: expr, ... }
		case p.accept(lexer.TokLBrace):
			// Only valid when base is IdentExpr (a type name)
			_, isIdent := e.(*ast.IdentExpr)
			if !isIdent {
				return nil, ErrUnexpectedToken("struct literal", p.tok)
			}
			var inits []ast.StructFieldInit
			var lastTok lexer.Token

			if p.at(lexer.TokRBrace) {
				lastTok = p.tok
				p.next()
			} else {
				for {
					// field name
					fn, err := p.expect(lexer.TokIdent)
					if err != nil {
						return nil, err
					}
					if _, err := p.expect(lexer.TokColon); err != nil {
						return nil, err
					}
					val, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					inits = append(inits, ast.StructFieldInit{
						Name:  fn.Lex,
						Value: val,
						Span:  spanFrom(posFrom(fn), exprEnd(val)),
					})
					if p.accept(lexer.TokComma) {
						// allow trailing comma
						if p.at(lexer.TokRBrace) {
							lastTok = p.tok
							p.next()
							break
						}
						continue
					}
					rb, err := p.expect(lexer.TokRBrace)
					if err != nil {
						return nil, err
					}
					lastTok = rb
					break
				}
			}

			// Replace base Ident + literal with a StructLitExpr
			name := baseIdentName(e)
			if name == "" {
				return nil, ErrUnexpectedToken("struct literal type", p.tok)
			}
			e = &ast.StructLitExpr{
				Name:  name,
				Inits: inits,
				Span:  spanFrom(exprStart(e), endPosFrom(lastTok)),
			}

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

		leftStart := exprStart(left)
		left = &ast.BinaryExpr{
			Op:    opTok.Kind.String(),
			Left:  left,
			Right: right,
			Span:  spanFrom(leftStart, exprEnd(right)),
		}
	}
}

/*** small helper ***/
func baseIdentName(e ast.Expr) string {
	if id, ok := e.(*ast.IdentExpr); ok {
		return id.Name
	}
	return ""
}
