package check

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
)

type Kind int

const (
	KindUnknown Kind = iota
	KindInt
	KindStr
	KindBool
)

func (k Kind) String() string {
	switch k {
	case KindInt:
		return "int"
	case KindStr:
		return "str"
	case KindBool:
		return "bool"
	default:
		return "unknown"
	}
}

type Info struct {
	// Vars holds the inferred kind of local variables in the current function.
	Vars map[string]Kind
}

// CheckFile walks the file (Stage-0: only def main) and infers simple kinds.
// It reports basic errors (assign to undeclared, mismatched assign).
func CheckFile(f *ast.File) (*Info, []error) {
	info := &Info{Vars: make(map[string]Kind)}
	var errs []error

	main := findMain(f)
	if main == nil {
		return info, nil
	}

	for _, s := range main.Body {
		switch st := s.(type) {
		case *ast.LetStmt:
			k := inferExprKind(st.Expr, info)
			info.Vars[st.Name] = k

		case *ast.AssignStmt:
			k := inferExprKind(st.Expr, info)
			if have, ok := info.Vars[st.Name]; ok {
				if have != KindUnknown && k != KindUnknown && have != k {
					errs = append(errs, fmt.Errorf("type mismatch on %q: have %s, got %s", st.Name, have, k))
				}
			} else {
				errs = append(errs, fmt.Errorf("assign to undeclared variable %q", st.Name))
			}

		case *ast.ReturnStmt:
			// Stage-0: we don't enforce the return type yet.

		case *ast.ExprStmt:
			// nothing to track
		}
	}
	return info, errs
}

func findMain(f *ast.File) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name == "main" {
			return fn
		}
	}
	return nil
}

// InferExprKind is exported for codegen; it infers the coarse kind for an expr.
func InferExprKind(info *Info, e ast.Expr) Kind { return inferExprKind(e, info) }

func inferExprKind(e ast.Expr, info *Info) Kind {
	switch v := e.(type) {
	case *ast.IntLit:
		return KindInt
	case *ast.StrLit:
		return KindStr
	case *ast.BoolLit:
		return KindBool
	case *ast.IdentExpr:
		if k, ok := info.Vars[v.Name]; ok {
			return k
		}
		return KindUnknown
	case *ast.UnaryExpr:
		k := inferExprKind(v.X, info)
		if k == KindInt {
			return KindInt
		}
		return KindUnknown
	case *ast.BinaryExpr:
		lk := inferExprKind(v.Left, info)
		rk := inferExprKind(v.Right, info)
		if lk == KindInt && rk == KindInt {
			return KindInt
		}
		// (future) string concat: if either is str and op is "+"
		if v.Op == "+" && (lk == KindStr || rk == KindStr) {
			return KindStr
		}
		return KindUnknown
	case *ast.CallExpr:
		// io.println returns void/unknown in Stage-0
		return KindUnknown
	case *ast.IndexExpr, *ast.FieldExpr:
		return KindUnknown
	default:
		return KindUnknown
	}
}
