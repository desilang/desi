package check

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) typ(e ast.Expr) types.T {
	switch x := e.(type) {
	case *ast.SliceExpr:
		bt := c.typ(x.X)
		if x.I != nil {
			_ = c.typ(x.I)
		}
		if x.J != nil {
			_ = c.typ(x.J)
		}
		if x.K != nil {
			_ = c.typ(x.K)
		}
		if types.Equal(bt, types.Str) {
			c.info.Types[x] = types.Str
			return types.Str
		}
		return nil

	case *ast.ListComp:
		et := c.typ(x.Elem)
		if et != nil {
			t := types.ListOf(et)
			c.info.Types[x] = t
			return t
		}
		return nil

	case *ast.SetComp:
		et := c.typ(x.Elem)
		if et != nil {
			t := types.SetOf(et)
			c.info.Types[x] = t
			return t
		}
		return nil

	case *ast.DictComp:
		kt := c.typ(x.Key)
		vt := c.typ(x.Val)
		if kt != nil && vt != nil {
			t := types.DictOf(kt, vt)
			c.info.Types[x] = t
			return t
		}
		return nil

	case *ast.MatchExpr:
		return c.checkMatchExpr(x)

	case *ast.ListLit:
		if len(x.Elems) == 0 {
			// Empty list: []
			// If we have an expected type from context (e.g. let x: list[int] = []), use it.
			// But 'typ' doesn't take context.
			// For now, we'll return list[void] or error if we can't infer.
			// Actually, let's return a special "unknown list" type or just list[void]
			// and let the assignment checker handle it?
			// Better: return list[void] (bottom type) which is compatible with any list?
			// Or just error for now saying explicit type needed.
			// In the test case: let nums: list[int] = [1, 2, 3]
			// The assignment checker will check compatibility.
			// But for [] we don't know the type.
			// Let's assume list[int] for now or error.
			// Real solution: Bidirectional type checking or context.
			// For now: Error if empty without context (which we don't have here).
			// But wait, if we are in an assignment, we might check it later.
			// Let's return types.ListOf(types.Void) for empty list?
			// Or maybe types.ListOf(types.Any)?
			// Let's try to infer from elements first.
			c.add(diagAt("DTE0005", x.Span, "empty list literals require type annotation (not yet supported)"))
			return nil
		}

		// Infer element type from first element
		elemType := c.typ(x.Elems[0])
		if elemType == nil {
			return nil
		}

		// Verify all elements have the same type
		for i, elem := range x.Elems {
			if i == 0 {
				continue
			}
			t := c.typ(elem)
			if t == nil {
				continue
			}
			if !types.Equal(t, elemType) {
				c.add(diagAt("DTE0105", elem.SpanOf(), fmt.Sprintf("list element type mismatch: expected %s, got %s", elemType, t)))
			}
		}

		t := types.ListOf(elemType)
		c.info.Types[x] = t
		return t

	case *ast.DictLit:
		// Empty dict requires type annotation (deferred for now)
		if len(x.Keys) == 0 {
			// For now, return error - require type annotation for empty dicts
			c.add(diagAt("DTE0004", x.Span, "empty dict literals require type annotation (not yet supported)"))
			return nil
		}

		// Infer key type from first key
		kt := c.typ(x.Keys[0])
		if kt == nil {
			return nil
		}

		// Infer value type from first value
		vt := c.typ(x.Values[0])
		if vt == nil {
			return nil
		}

		// Validate all keys have the same type
		for _, key := range x.Keys {
			keyType := c.typ(key)
			if keyType == nil {
				continue
			}
			if !types.Equal(keyType, kt) {
				c.add(diagAt("DTE0104", key.SpanOf(), "dict key type mismatch"))
			}
		}

		// Validate all values have the same type
		for _, val := range x.Values {
			valType := c.typ(val)
			if valType == nil {
				continue
			}
			if !types.Equal(valType, vt) {
				c.add(diagAt("DTE0104", val.SpanOf(), "dict value type mismatch"))
			}
		}

		// Return dict[K, V] type
		t := types.DictOf(kt, vt)
		c.info.Types[x] = t
		return t

	case *ast.SetLit:
		// Empty set literal {} is ambiguous with empty dict, but parser handles {} as empty dict.
		// So SetLit here implies non-empty or we might have explicit syntax later.
		// Actually, parser returns DictLit for empty {}.
		if len(x.Elems) == 0 {
			t := types.SetOf(types.Any)
			c.info.Types[x] = t
			return t
		}

		// Infer element type from first element
		et := c.typ(x.Elems[0])
		if et == nil {
			return nil
		}

		// Validate all elements have the same type
		for _, elem := range x.Elems {
			elemType := c.typ(elem)
			if elemType == nil {
				continue
			}
			if !types.Equal(elemType, et) {
				c.add(diagAt("DTE0104", elem.SpanOf(), "set element type mismatch"))
			}
		}

		t := types.SetOf(et)
		c.info.Types[x] = t
		return t

	case *ast.IntLit:
		c.info.Types[e] = types.Int
		return types.Int
	case *ast.FloatLit:
		c.info.Types[e] = types.Float
		return types.Float
	case *ast.BoolLit:
		c.info.Types[e] = types.Bool
		return types.Bool
	case *ast.StrLit:
		c.info.Types[e] = types.Str
		return types.Str
	case *ast.FString:
		for _, part := range x.Parts {
			_ = c.typ(part)
		}
		c.info.Types[e] = types.Str
		return types.Str
	case *ast.NoneLit:
		c.info.Types[e] = types.None
		return types.None

	case *ast.Ident:
		return c.typIdent(x)

	case *ast.UnaryExpr:
		t := c.typ(x.X)
		if x.Op == "await" {
			if ft, ok := t.(*types.Future); ok {
				c.info.Types[e] = ft.Elem
				return ft.Elem
			}
			c.add(diagAt("DTE0004", x.Span, "invalid unary 'await'"))
			return nil
		}
		if x.Op == "-" {
			if types.Equal(t, types.Int) || types.Equal(t, types.Float) {
				c.info.Types[e] = t
				return t
			}
		}
		if x.Op == "!" || x.Op == "not" {
			if types.Equal(t, types.Bool) {
				c.info.Types[e] = types.Bool
				return types.Bool
			}
		}
		c.add(diagAt("DTE0004", x.Span, "invalid unary '"+x.Op+"'"))
		return nil

	case *ast.BinaryExpr:
		return c.typBinary(x)

	case *ast.CallExpr:
		return c.typCall(x)

	case *ast.FieldExpr:
		return c.typFieldExpr(x)
	case *ast.TupleLit:
		var elems []types.T
		for _, el := range x.Elems {
			elems = append(elems, c.typ(el))
		}
		t := types.TupleOf(elems...)
		c.info.Types[e] = t
		return t

	case *ast.IndexExpr:
		return c.typIndexExpr(x)

	case *ast.LambdaExpr:
		// require typed params in M4
		params := make([]types.T, len(x.Params))
		for i, p := range x.Params {
			if p.Type == nil {
				c.add(diagAt("DTE0004", x.Span, "lambda parameters must be typed"))
				return nil
			}
			if t, ok := types.FromName(p.Type.Name); ok {
				params[i] = t
			} else {
				c.add(diagAt("DTE0004", x.Span, "unknown lambda param type: "+p.Type.Name))
				return nil
			}
		}
		bt := c.typ(x.Body)
		c.info.Types[e] = types.FuncOf(params, bt, false)
		return c.info.Types[e]

	default:
		return nil
	}
}

// typIdent handles identifier expressions, including DBR0004 (use after move).
func (c *checker) typIdent(x *ast.Ident) types.T {
	if sp, ok := c.moved.movedAt(x.Name); ok {
		c.issueUseAfterMove(x.Span, sp)
	}
	if sym := c.scope.Lookup(x.Name); sym != nil {
		c.info.Types[x] = sym.Type
		return sym.Type
	}
	return nil
}

// typCall performs overload resolution for calls and wires move tracking.
// IMPORTANT: We only use named-arg canonicalization when the call actually
// contains named arguments. Purely positional calls follow the legacy path
// to preserve existing behaviors (len diagnostics, borrow/move, etc.).
