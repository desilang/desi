package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

// posFrom converts a lexer.Token into an ast.Pos using the token's start location.
func posFrom(t lexer.Token) ast.Pos {
	return ast.Pos{Line: t.Line, Col: t.Col}
}

// endPosFrom returns a position just past the token start (good enough for stage-0).
func endPosFrom(t lexer.Token) ast.Pos {
	return ast.Pos{Line: t.Line, Col: t.Col + 1}
}

// spanTok builds a Span from two tokens (inclusive start, inclusive end).
func spanTok(start, end lexer.Token) ast.Span {
	return ast.Span{
		Start: posFrom(start),
		End:   endPosFrom(end),
	}
}

// spanFrom builds a Span from explicit start and end positions.
func spanFrom(start ast.Pos, end ast.Pos) ast.Span {
	return ast.Span{Start: start, End: end}
}

// ---- helpers to read spans from expressions (for composing larger nodes) ----

func exprSpan(e ast.Expr) ast.Span {
	switch v := e.(type) {
	case *ast.IdentExpr:
		return v.Span
	case *ast.IntLit:
		return v.Span
	case *ast.StrLit:
		return v.Span
	case *ast.BoolLit:
		return v.Span
	case *ast.CallExpr:
		return v.Span
	case *ast.IndexExpr:
		return v.Span
	case *ast.FieldExpr:
		return v.Span
	case *ast.UnaryExpr:
		return v.Span
	case *ast.BinaryExpr:
		return v.Span
	default:
		return ast.Span{}
	}
}

func exprStart(e ast.Expr) ast.Pos { return exprSpan(e).Start }
func exprEnd(e ast.Expr) ast.Pos   { return exprSpan(e).End }
