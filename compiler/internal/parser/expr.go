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

	case p.at(lexer.TokAwait):
		awTok := p.tok
		if !p.features.Async {
			// If the feature is gated off, treat as error.
			return nil, ErrAwaitRequiresExpr(awTok)
		}
		p.next() // consume 'await'

		// Provide a precise diagnostic if no expression follows.
		if isAwaitExprStopper(p.tok.Kind) {
			return nil, ErrAwaitRequiresExpr(awTok)
		}

		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.AwaitExpr{Expr: x, Span: spanFrom(posFrom(awTok), exprEnd(x))}, nil

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
	if p.at(lexer.TokFloat) {
		t := p.tok
		p.next()
		return p.parsePostfix(&ast.FloatLit{Value: t.Lex, Span: spanTok(t, t)})
	}
	// Registry-backed parser error
	return nil, ErrUnexpectedToken("expression", p.tok)
}

func (p *Parser) parsePostfix(base ast.Expr) (ast.Expr, error) {
	e := base
	for {
		// Struct literal: <Ident> { field: expr, ... }
		if id, ok := e.(*ast.IdentExpr); ok && p.at(lexer.TokLBrace) {

			p.next() // eat '{'
			var fields []ast.StructLitField
			var rb lexer.Token

			// empty literal?
			if p.at(lexer.TokRBrace) {
				rb = p.tok
				p.next()
				e = &ast.StructLit{
					Name:   id.Name,
					Fields: nil,
					Span:   spanFrom(exprStart(e), endPosFrom(rb)),
				}
				continue
			}

			for {
				// key
				keyTok, err := p.expect(lexer.TokIdent)
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
				fields = append(fields, ast.StructLitField{
					Name:  keyTok.Lex,
					Value: val,
					Span:  spanFrom(posFrom(keyTok), exprEnd(val)),
				})

				// comma or closing brace (allow trailing comma)
				if p.accept(lexer.TokComma) {
					if p.at(lexer.TokRBrace) {
						rb = p.tok
						p.next()
						break
					}
					continue
				}
				rb, err = p.expect(lexer.TokRBrace)
				if err != nil {
					return nil, err
				}
				break
			}

			e = &ast.StructLit{
				Name:   id.Name,
				Fields: fields,
				Span:   spanFrom(exprStart(base), endPosFrom(rb)),
			}
			continue
		}

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

// --- helpers for await ---

func isAwaitExprStopper(k lexer.TokKind) bool {
	// If any of these appear immediately after 'await', there's no valid expression.
	switch k {
	case lexer.TokEOF, lexer.TokNewline,
		lexer.TokRParen, lexer.TokRBrack, lexer.TokRBrace,
		lexer.TokComma, lexer.TokColon, lexer.TokArrow:
		return true
	default:
		return false
	}
}
