package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

// pos/span helpers -----------------------------------------------------------

func posFrom(t lexer.Token) ast.Pos { return ast.Pos{Line: t.Line, Col: t.Col} }
func endPosFrom(t lexer.Token) ast.Pos {
	return ast.Pos{Line: t.Line, Col: t.Col + 1}
}
func spanTok(start, end lexer.Token) ast.Span {
	return ast.Span{Start: posFrom(start), End: endPosFrom(end)}
}
func spanFrom(start, end ast.Pos) ast.Span { return ast.Span{Start: start, End: end} }

// expr spans -----------------------------------------------------------------

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

// stmt spans -----------------------------------------------------------------

func stmtSpan(s ast.Stmt) ast.Span {
	switch v := s.(type) {
	case *ast.LetStmt:
		return v.Span
	case *ast.AssignStmt:
		return v.Span
	case *ast.ReturnStmt:
		return v.Span
	case *ast.ExprStmt:
		return v.Span
	case *ast.IfStmt:
		return v.Span
	case *ast.WhileStmt:
		return v.Span
	case *ast.DeferStmt:
		return v.Span
	default:
		return ast.Span{}
	}
}
func stmtEnd(s ast.Stmt) ast.Pos { return stmtSpan(s).End }

// block helpers --------------------------------------------------------------

func blockEnd(stmts []ast.Stmt) ast.Pos {
	if len(stmts) == 0 {
		return ast.Pos{} // unknown
	}
	return stmtEnd(stmts[len(stmts)-1])
}
