package c

import (
	"bytes"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/term"
)

func emitStmt(b *bytes.Buffer, indent int, s ast.Stmt, e *env) {
	ind := spaces(indent)
	switch st := s.(type) {
	case *ast.LetStmt:
		n := _min(len(st.Binds), len(st.Values))
		for i := 0; i < n; i++ {
			name := st.Binds[i].Name
			ce, kind := cExprFor(st.Values[i], e)
			if kind == "" {
				kind = "int"
			}
			// store textual type for variable
			if strings.TrimSpace(st.Binds[i].Type) != "" {
				e.vars[name] = strings.TrimSpace(st.Binds[i].Type)
				term.Wprintf(b, "%s%s %s = %s;\n", ind, cTypeFromText(e.vars[name], e.info), name, ce)
			} else {
				// infer from kind
				if kind == "str" {
					e.vars[name] = "str"
				} else {
					e.vars[name] = "int"
				}
				term.Wprintf(b, "%s%s %s = %s;\n", ind, cTypeFromText(e.vars[name], e.info), name, ce)
			}
		}
		for i := n; i < len(st.Binds); i++ {
			name := st.Binds[i].Name
			typ := strings.TrimSpace(st.Binds[i].Type)
			if typ == "" {
				typ = "int"
			}
			e.vars[name] = typ
			def := "0"
			if isStrText(typ) {
				def = "\"\""
			}
			term.Wprintf(b, "%s%s %s = %s;\n", ind, cTypeFromText(typ, e.info), name, def)
		}

	case *ast.AssignStmt:
		// ---- Preferred path: LHS as expressions ----
		if len(st.LHS) > 0 {
			n := _min(len(st.LHS), len(st.Exprs))

			// Fast path: single assignment — emit directly, no temporaries.
			if n == 1 {
				lhsStr, ok := cLValueFor(st.LHS[0], e)
				if !ok {
					// Unsupported LHS kind in Stage-1: evaluate RHS for side-effects and drop.
					rx, _ := cExprFor(st.Exprs[0], e)
					term.Wprintf(b, "%s/* unsupported LHS */ (void)(%s);\n", ind, rx)
					break
				}
				rx, _ := cExprFor(st.Exprs[0], e)
				term.Wprintf(b, "%s%s = %s;\n", ind, lhsStr, rx)
				break
			}

			// Parallel assignment → evaluate RHS first to preserve order.
			type tmpRec struct {
				name string
				kind string
			}
			var tmps []tmpRec
			for i := 0; i < n; i++ {
				cx, k := cExprFor(st.Exprs[i], e)
				if k == "" {
					k = "int"
				}
				t := e.newTemp()
				term.Wprintf(b, "%s%s %s = %s;\n", ind, cType(k), t, cx)
				tmps = append(tmps, tmpRec{name: t, kind: k})
			}
			for i := 0; i < n; i++ {
				lhsStr, ok := cLValueFor(st.LHS[i], e)
				if !ok {
					term.Wprintf(b, "%s/* unsupported LHS in parallel assign */;\n", ind)
					continue
				}
				term.Wprintf(b, "%s%s = %s;\n", ind, lhsStr, tmps[i].name)
			}
			break
		}

		// ---- Legacy fallback: Names []string ----
		n := _min(len(st.Names), len(st.Exprs))

		// Single assignment (legacy) — emit directly, no temporaries, no C-side warnings.
		if n == 1 {
			lhs := st.Names[0]
			cx, _ := cExprFor(st.Exprs[0], e)
			term.Wprintf(b, "%s%s = %s;\n", ind, lhs, cx)
			break
		}

		// Parallel assignment (legacy)
		type tmpRec struct {
			name string
			kind string
		}
		var tmps []tmpRec
		for i := 0; i < n; i++ {
			cx, k := cExprFor(st.Exprs[i], e)
			if k == "" {
				k = "int"
			}
			t := e.newTemp()
			term.Wprintf(b, "%s%s %s = %s;\n", ind, cType(k), t, cx)
			tmps = append(tmps, tmpRec{name: t, kind: k})
		}
		for i := 0; i < n; i++ {
			lhs := st.Names[i]
			term.Wprintf(b, "%s%s = %s;\n", ind, lhs, tmps[i].name)
		}

	case *ast.ExprStmt:
		emitCallOrExpr(b, indent, st.Expr, e)

	case *ast.ReturnStmt:
		if len(e.defers) > 0 {
			emitDefers(b, indent, e)
		}
		if st.Expr == nil {
			if e.retKind == "void" {
				term.Wprintf(b, "%sreturn;\n", ind)
			} else {
				term.Wprintf(b, "%sreturn 0;\n", ind)
			}
			return
		}
		cExpr, kind := cExprFor(st.Expr, e)
		if e.retKind == "int" && kind != "int" {
			term.Wprintf(b, "%s/* non-int return; force 0 */\n", ind)
			term.Wprintf(b, "%sreturn 0;\n", ind)
			return
		}
		if e.retKind == "str" && kind != "str" {
			term.Wprintf(b, "%s/* non-str return; force \"\" */\n", ind)
			term.Wprintf(b, "%sreturn \"\";\n", ind)
			return
		}
		term.Wprintf(b, "%sreturn %s;\n", ind, cExpr)

	case *ast.IfStmt:
		cond, _ := cExprFor(st.Cond, e)
		term.Wprintf(b, "%sif (%s) {\n", ind, stripOuterParens(cond))
		for _, s2 := range st.Then {
			emitStmt(b, indent+2, s2, e)
		}
		term.Wprintf(b, "%s}", ind)
		for _, el := range st.Elifs {
			ec, _ := cExprFor(el.Cond, e)
			term.Wprintf(b, " else if (%s) {\n", stripOuterParens(ec))
			for _, s2 := range el.Body {
				emitStmt(b, indent+2, s2, e)
			}
			term.Wprintf(b, "%s}", ind)
		}
		if st.Else != nil {
			term.Wprintf(b, " else {\n")
			for _, s2 := range st.Else {
				emitStmt(b, indent+2, s2, e)
			}
			term.Wprintf(b, "%s}\n", ind)
		} else {
			term.Wprintf(b, "\n")
		}

	case *ast.WhileStmt:
		cond, _ := cExprFor(st.Cond, e)
		term.Wprintf(b, "%swhile (%s) {\n", ind, stripOuterParens(cond))
		for _, s2 := range st.Body {
			emitStmt(b, indent+2, s2, e)
		}
		term.Wprintf(b, "%s}\n", ind)

	case *ast.DeferStmt:
		e.defers = append(e.defers, st.Call)
		term.Wprintf(b, "%s/* defer scheduled */\n", ind)

	default:
		term.Wprintf(b, "%s/* stmt not lowered */\n", ind)
	}
}

func emitCallOrExpr(b *bytes.Buffer, indent int, expr ast.Expr, e *env) {
	ind := spaces(indent)
	// io.println(...): special-case to printf
	if call, ok := expr.(*ast.CallExpr); ok && isIoPrintln(call) {
		emitPrintln(b, indent, call, e)
		return
	}
	cx, _ := cExprFor(expr, e)
	term.Wprintf(b, "%s(void)(%s);\n", ind, cx)
}

func emitDefers(b *bytes.Buffer, indent int, e *env) {
	ind := spaces(indent)
	for i := len(e.defers) - 1; i >= 0; i-- {
		call := e.defers[i]
		if ce, ok := call.(*ast.CallExpr); ok && isIoPrintln(ce) {
			emitPrintln(b, indent, ce, e)
			continue
		}
		cx, _ := cExprFor(call, e)
		term.Wprintf(b, "%s/* defer */ (void)(%s);\n", ind, cx)
	}
}

func isIoPrintln(c *ast.CallExpr) bool {
	if fe, ok := c.Callee.(*ast.FieldExpr); ok && fe.Name == "println" {
		if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "io" {
			return true
		}
	}
	return false
}

// Variadic println: io.println(a, b, c, ...)
func emitPrintln(b *bytes.Buffer, indent int, call *ast.CallExpr, e *env) {
	ind := spaces(indent)
	term.Wprintf(b, "%sprintf(", ind)
	term.Wprintf(b, "%s", buildPrintfArgs(call.Args, e))
	term.Wprintf(b, ");\n")
}

func buildPrintfArgs(args []ast.Expr, e *env) string {
	var fmt strings.Builder
	var argv []string
	fmt.WriteString("\"")
	if len(args) == 0 {
		fmt.WriteString("\\n\"")
		return fmt.String()
	}
	for _, a := range args {
		ce, kind := cExprFor(a, e)
		if kind == "str" {
			fmt.WriteString("%s")
		} else {
			fmt.WriteString("%d")
		}
		argv = append(argv, ce)
	}
	fmt.WriteString("\\n\"")
	if len(argv) > 0 {
		return fmt.String() + ", " + strings.Join(argv, ", ")
	}
	return fmt.String()
}

// small helper to avoid importing math
func _min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
