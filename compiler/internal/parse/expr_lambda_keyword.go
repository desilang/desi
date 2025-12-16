package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseLambdaKeyword parses Python-style lambda with explicit return type:
//
//	lambda<RetType> x: ParamType, y: ParamType: body_expr
//	lambda<int> x: int: x * 2
//	lambda<none> x: int: print(x)
//
// Called when current token is KW_lambda.
func (p *Parser) parseLambdaKeyword() ast.Expr {
	start := spanPos(p.file, p.cur)
	p.next() // consume 'lambda'

	// Parse return type: lambda<RetType>
	var retType *ast.TypeName
	if p.cur.Tok == token.LT {
		p.next() // consume '<'
		retType = p.parseTypeName()
		if !p.expect(token.GT, ">") {
			return &ast.Ident{Name: "<error>", Span: start}
		}
	} else {
		p.errExpected(spanPos(p.file, p.cur), "return type in lambda<Type>")
		return &ast.Ident{Name: "<error>", Span: start}
	}

	var params []ast.LambdaParam

	// Parse parameter list: name: Type, name: Type
	// until we hit a colon that's NOT followed by a type identifier
	for p.cur.Tok != token.EOF {
		// Check if we're at the final colon (before the body)
		if p.cur.Tok == token.COLON {
			// If next token looks like an expression start, this is the body colon
			if !p.isTypeName(p.peek.Tok, p.peek.Lexeme) {
				break
			}
			// Edge case of no params: `lambda<int>: expr`
			if len(params) == 0 {
				break
			}
		}

		// Expect parameter name
		if p.cur.Tok != token.IDENT {
			p.errExpected(spanPos(p.file, p.cur), "parameter name")
			break
		}
		name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		paramStart := spanPos(p.file, p.cur)
		p.next()

		// Parameter type is required: name: Type
		var ty *ast.TypeName
		if p.cur.Tok == token.COLON {
			// Look ahead - if after COLON we have a type identifier, consume it
			if p.isTypeName(p.peek.Tok, p.peek.Lexeme) {
				p.next() // consume ':'
				ty = p.parseTypeName()
			} else {
				// This colon is the body separator - param has no type annotation
				params = append(params, ast.LambdaParam{
					Name: name,
					Type: ty,
					Span: paramStart,
				})
				break
			}
		}

		params = append(params, ast.LambdaParam{
			Name: name,
			Type: ty,
			Span: paramStart,
		})

		// Check for comma (more params) or break
		if p.cur.Tok == token.COMMA {
			p.next() // consume ','
			continue
		}

		// No comma - we're done with params
		break
	}

	// Expect ':' before body
	if !p.expect(token.COLON, ":") {
		return &ast.Ident{Name: "<error>", Span: start}
	}

	// Parse body expression
	body := p.parseExpr()

	return &ast.LambdaExpr{
		Params:  params,
		RetType: retType,
		Body:    body,
		Span:    ast.JoinSpan(start, body.SpanOf()),
	}
}

// isTypeName checks if a token could be a type name.
func (p *Parser) isTypeName(tok token.Token, lexeme string) bool {
	if tok == token.IDENT {
		// Check for common type names
		switch lexeme {
		case "int", "str", "bool", "float", "none",
			"i8", "i16", "i32", "i64", "i128",
			"u8", "u16", "u32", "u64", "u128",
			"f32", "f64", "isize", "usize",
			"list", "dict", "set", "tuple":
			return true
		}
		// Capitalized names are likely types (PascalCase convention)
		if len(lexeme) > 0 && lexeme[0] >= 'A' && lexeme[0] <= 'Z' {
			return true
		}
	}
	return false
}
