package check

import (
	"fmt"
	"strconv"

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
		// Tuple slicing: t[i:j] -> requires compile-time known indices
		if tupType, ok := bt.(*types.Tuple); ok {
			// For now, require i and j to be integer literals for compile-time slicing
			iVal := 0
			jVal := len(tupType.Elems)

			if x.I != nil {
				if lit, ok := x.I.(*ast.IntLit); ok {
					iVal, _ = strconv.Atoi(lit.Text)
				} else {
					c.add(diagAt("DTE0004", x.Span, "tuple slice indices must be integer literals"))
					return nil
				}
			}
			if x.J != nil {
				if lit, ok := x.J.(*ast.IntLit); ok {
					jVal, _ = strconv.Atoi(lit.Text)
				} else {
					c.add(diagAt("DTE0004", x.Span, "tuple slice indices must be integer literals"))
					return nil
				}
			}

			// Validate indices
			if iVal < 0 || jVal > len(tupType.Elems) || iVal > jVal {
				c.add(diagAt("DTE0004", x.Span, "tuple slice indices out of bounds"))
				return nil
			}

			// Create new tuple type with sliced elements
			slicedElems := tupType.Elems[iVal:jVal]
			if len(slicedElems) == 0 {
				c.add(diagAt("DTE0004", x.Span, "empty tuple slices are not supported"))
				return nil
			}
			if len(slicedElems) == 1 {
				// Single element - just return the element type (no single-element tuples)
				c.info.Types[x] = slicedElems[0]
				return slicedElems[0]
			}
			slicedTuple := types.TupleOf(slicedElems...)
			c.info.Types[x] = slicedTuple
			return slicedTuple
		}
		return nil

	case *ast.ListComp:
		// Create a child scope for the comprehension to bind loop variables
		saved := c.scope
		c.scope = NewScope(c.scope)
		defer func() { c.scope = saved }()

		// Process each clause: check iterable, bind loop variable
		for _, clause := range x.Clauses {
			// 1. Type-check the iterable expression
			iterType := c.typ(clause.Iter)
			if iterType == nil {
				continue
			}

			// 2. Extract element type from iterable
			var elemType types.T
			switch it := iterType.(type) {
			case *types.List:
				elemType = it.Elem
			case *types.Set:
				elemType = it.Elem
			default:
				// For now, support list and set. Could extend to other iterables.
				c.add(diagAt("DTE0004", clause.Iter.SpanOf(), "cannot iterate over "+iterType.String()))
				continue
			}

			// 3. Bind the loop variable into scope
			if target, ok := clause.Target.(*ast.Ident); ok {
				c.scope.Define(&Symbol{
					Name: target.Name,
					Kind: SymVar,
					Type: elemType,
				})
				c.info.Types[target] = elemType
			}

			// 4. Type-check filter condition if present
			if clause.If != nil {
				condType := c.typ(clause.If)
				if condType != nil && !types.Equal(condType, types.Bool) {
					c.add(diagAt("DTE0004", clause.If.SpanOf(), "filter condition must be bool, got "+condType.String()))
				}
			}
		}

		// 5. Now check the element expression with loop variables in scope
		et := c.typ(x.Elem)
		if et != nil {
			t := types.ListOf(et)
			c.info.Types[x] = t
			return t
		}
		return nil

	case *ast.SetComp:
		// Create a child scope for the comprehension to bind loop variables
		saved := c.scope
		c.scope = NewScope(c.scope)
		defer func() { c.scope = saved }()

		// Process each clause: check iterable, bind loop variable
		for _, clause := range x.Clauses {
			iterType := c.typ(clause.Iter)
			if iterType == nil {
				continue
			}
			var elemType types.T
			switch it := iterType.(type) {
			case *types.List:
				elemType = it.Elem
			case *types.Set:
				elemType = it.Elem
			default:
				c.add(diagAt("DTE0004", clause.Iter.SpanOf(), "cannot iterate over "+iterType.String()))
				continue
			}
			if target, ok := clause.Target.(*ast.Ident); ok {
				c.scope.Define(&Symbol{Name: target.Name, Kind: SymVar, Type: elemType})
				c.info.Types[target] = elemType
			}
			if clause.If != nil {
				condType := c.typ(clause.If)
				if condType != nil && !types.Equal(condType, types.Bool) {
					c.add(diagAt("DTE0004", clause.If.SpanOf(), "filter condition must be bool"))
				}
			}
		}

		et := c.typ(x.Elem)
		if et != nil {
			t := types.SetOf(et)
			c.info.Types[x] = t
			return t
		}
		return nil

	case *ast.DictComp:
		// Create a child scope for the comprehension to bind loop variables
		saved := c.scope
		c.scope = NewScope(c.scope)
		defer func() { c.scope = saved }()

		// Process each clause: check iterable, bind loop variable
		for _, clause := range x.Clauses {
			iterType := c.typ(clause.Iter)
			if iterType == nil {
				continue
			}
			var elemType types.T
			switch it := iterType.(type) {
			case *types.List:
				elemType = it.Elem
			case *types.Set:
				elemType = it.Elem
			default:
				c.add(diagAt("DTE0004", clause.Iter.SpanOf(), "cannot iterate over "+iterType.String()))
				continue
			}
			if target, ok := clause.Target.(*ast.Ident); ok {
				c.scope.Define(&Symbol{Name: target.Name, Kind: SymVar, Type: elemType})
				c.info.Types[target] = elemType
			}
			if clause.If != nil {
				condType := c.typ(clause.If)
				if condType != nil && !types.Equal(condType, types.Bool) {
					c.add(diagAt("DTE0004", clause.If.SpanOf(), "filter condition must be bool"))
				}
			}
		}

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
			// Return list[none] which is assignable to any list[T]
			t := types.ListOf(types.None)
			c.info.Types[x] = t
			return t
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
		// Empty dict: {}
		if len(x.Keys) == 0 {
			// Return dict[none, none] which is assignable to any dict[K, V]
			t := types.DictOf(types.None, types.None)
			c.info.Types[x] = t
			return t
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
			t := types.SetOf(types.None)
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
	case *ast.DecimalLit:
		c.info.Types[e] = types.Decimal
		return types.Decimal
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

		// Handle unary operators with dunder support
		if x.Op == "await" {
			if ft, ok := t.(*types.Future); ok {
				c.info.Types[e] = ft.Elem
				return ft.Elem
			}
			c.add(diagAt("DTE0004", x.Span, "invalid unary 'await'"))
			return nil
		}

		// Boolean negation
		if x.Op == "!" || x.Op == "not" {
			// TODO: Support __bool__ or truthiness for custom types?
			// For now, require bool.
			if types.Equal(t, types.Bool) {
				c.info.Types[e] = types.Bool
				return types.Bool
			}
			c.add(diagAt("DTE0004", x.Span, "invalid unary '"+x.Op+"' on type '"+t.String()+"' (expected bool)"))
			return nil
		}

		// Arithmetic/Bitwise unary operators: -, +, ~
		// 1. Check for dunder methods on custom types
		if cls, ok := t.(*types.Class); ok {
			var method string
			switch x.Op {
			case "-":
				method = "__neg__"
			case "+":
				method = "__pos__"
			case "~":
				method = "__invert__"
			}

			if method != "" {
				// Lookup method (including inherited)
				curr := cls
				var m *types.Func
				for curr != nil {
					// Check Dunders map first
					if found, ok := curr.Dunders[method]; ok {
						m = found
						break
					}
					// Also check Methods map just in case (though checkClass separates them)
					if found, ok := curr.Methods[method]; ok {
						m = found
						break
					}
					curr = curr.Base
				}

				if m != nil {
					// Check return type of dunder
					c.info.Types[e] = m.Ret
					return m.Ret
				}
			}
		}

		// 2. Fallback to primitive types
		switch x.Op {
		case "-":
			if types.Equal(t, types.Int) || types.Equal(t, types.Float) {
				c.info.Types[e] = t
				return t
			}
		case "+":
			if types.Equal(t, types.Int) || types.Equal(t, types.Float) {
				c.info.Types[e] = t
				return t
			}
		case "~":
			if types.Equal(t, types.Int) {
				c.info.Types[e] = t
				return t
			}
		}

		c.add(diagAt("DTE0004", x.Span, "invalid unary '"+x.Op+"' on type '"+t.String()+"'"))
		return nil

	case *ast.BinaryExpr:
		return c.typBinary(x)

	case *ast.CallExpr:
		return c.typCall(x)

	case *ast.FieldExpr:
		return c.typFieldExpr(x)

	case *ast.IsExpr:
		// Type-check the LHS
		lhsType := c.typ(x.X)

		// Check for enum variant patterns: is Some(val), is Ok(v), is Err(e)
		// Also handles negated: is not Some(_), is not Ok(_), is not Err(_)
		if call, ok := x.Pattern.(*ast.CallExpr); ok {
			// Check if callee is Some/Ok/Err
			if callee, ok := call.Callee.(*ast.Ident); ok {
				variantName := callee.Name
				isEnumPattern := variantName == "Some" || variantName == "Ok" || variantName == "Err"

				if isEnumPattern {
					// This is a valid enum pattern - don't type-check as expression
					// Only create bindings if NOT negated
					if !x.Negated && len(call.Args) > 0 {
						// Get the payload type from the LHS type
						var payloadType types.T

						if variantName == "Some" && types.IsOption(lhsType) {
							payloadType = types.OptionSomeType(lhsType)
						} else if variantName == "Ok" && types.IsResult(lhsType) {
							payloadType = types.ResultOkType(lhsType)
						} else if variantName == "Err" && types.IsResult(lhsType) {
							payloadType = types.ResultErrType(lhsType)
						}

						if payloadType != nil {
							// Create bindings for pattern variables
							var bindings []MatchBinding
							for i, arg := range call.Args {
								if ident, ok := arg.(*ast.Ident); ok && ident.Name != "_" {
									binding := MatchBinding{
										Name:       ident.Name,
										Type:       payloadType,
										FieldIndex: i,
										Node:       ident,
									}
									bindings = append(bindings, binding)

									// Register binding in info
									sym := &Symbol{
										Name: ident.Name,
										Kind: SymVar,
										Type: payloadType,
										Node: ident,
									}
									c.info.Idents[ident] = sym
								}
							}

							// Store bindings for lowering
							if len(bindings) > 0 {
								if c.info.IsBindings == nil {
									c.info.IsBindings = make(map[*ast.IsExpr][]MatchBinding)
								}
								c.info.IsBindings[x] = bindings
							}
						}
					}
					// Don't fall through to type-check as expression
				} else {
					// Not an enum pattern - type check as expression
					c.typ(x.Pattern)
				}
			} else {
				// Callee is not an Ident - type check as expression
				c.typ(x.Pattern)
			}
		} else {
			// Regular pattern (not CallExpr) - type check it
			c.typ(x.Pattern)
		}

		// 'is' expression always returns bool
		c.info.Types[e] = types.Bool
		return types.Bool

	case *ast.CastExpr:
		// 'as' expression: expr as type (explicit type cast)
		fromType := c.typ(x.X)
		toType := c.resolveType(x.Type)

		// Validate the cast is allowed (numeric types only)
		fromNumeric := types.Equal(fromType, types.Int) || types.Equal(fromType, types.Float) ||
			types.Equal(fromType, types.Decimal) || types.Equal(fromType, types.Char) ||
			types.Equal(fromType, types.I8) || types.Equal(fromType, types.I16) ||
			types.Equal(fromType, types.I32) || types.Equal(fromType, types.I64) ||
			types.Equal(fromType, types.U8) || types.Equal(fromType, types.U16) ||
			types.Equal(fromType, types.U32) || types.Equal(fromType, types.U64) ||
			types.Equal(fromType, types.F32) || types.Equal(fromType, types.F64)

		toNumeric := types.Equal(toType, types.Int) || types.Equal(toType, types.Float) ||
			types.Equal(toType, types.Decimal) || types.Equal(toType, types.Char) ||
			types.Equal(toType, types.I8) || types.Equal(toType, types.I16) ||
			types.Equal(toType, types.I32) || types.Equal(toType, types.I64) ||
			types.Equal(toType, types.U8) || types.Equal(toType, types.U16) ||
			types.Equal(toType, types.U32) || types.Equal(toType, types.U64) ||
			types.Equal(toType, types.F32) || types.Equal(toType, types.F64)

		if !fromNumeric || !toNumeric {
			c.add(diagAt("DTE0004", e.SpanOf(), fmt.Sprintf("cannot cast %s to %s", fromType, toType)))
		}

		c.info.Types[e] = toType
		return toType

	case *ast.TupleLit:
		// Desi does not support single-element tuples - just use the value directly
		// But allow spread expressions that might expand to multiple elements
		hasSpreads := false
		for _, el := range x.Elems {
			if _, ok := el.(*ast.SpreadExpr); ok {
				hasSpreads = true
				break
			}
		}

		if len(x.Elems) == 1 && !hasSpreads {
			c.add(diagAt("DTE0050", x.Span, "single-element tuples are not supported; use the value directly"))
			// Still return the inner type so type checking can continue
			innerType := c.typ(x.Elems[0])
			c.info.Types[e] = innerType
			return innerType
		}

		var elems []types.T
		for _, el := range x.Elems {
			if spread, ok := el.(*ast.SpreadExpr); ok {
				// Spread expression: *expr - must be a tuple, flatten its elements
				spreadType := c.typ(spread.X)
				if spreadType == nil {
					continue
				}
				if tupT, ok := spreadType.(*types.Tuple); ok {
					// Flatten tuple elements
					elems = append(elems, tupT.Elems...)
					c.info.Types[spread] = spreadType
				} else {
					c.add(diagAt("DTE0004", spread.Span, "spread operator requires tuple type, got '"+spreadType.String()+"'"))
				}
			} else {
				elems = append(elems, c.typ(el))
			}
		}

		// Use NamedTupleOf for named tuple literals
		var t *types.Tuple
		if len(x.Names) > 0 {
			t = types.NamedTupleOf(x.Names, elems)
		} else {
			t = types.TupleOf(elems...)
		}
		c.info.Types[e] = t
		return t

	case *ast.IndexExpr:
		return c.typIndexExpr(x)

	case *ast.TryExpr:
		// ? operator unwraps Result[T, E] -> T or Option[T] -> T
		t := c.typ(x.X)
		if t == nil {
			c.add(diagAt("DTE0004", x.Span, "cannot use ? on expression with unknown type"))
			return nil
		}

		// Check for Generic enum type (Result[T,E] or Option[T])
		if g, ok := t.(*types.Generic); ok {
			if enum, ok := g.Base.(*types.Enum); ok {
				switch enum.Name {
				case "Result":
					// Result[T, E] -> T (first type argument is success type)
					if len(g.Args) >= 1 {
						c.info.Types[x] = g.Args[0]
						return g.Args[0]
					}
				case "Option":
					// Option[T] -> T (first type argument is wrapped type)
					if len(g.Args) >= 1 {
						c.info.Types[x] = g.Args[0]
						return g.Args[0]
					}
				}
				c.add(diagAt("DTE0004", x.Span, "? operator requires Result or Option type, got "+enum.Name))
				return nil
			}
		}

		// Check for unparameterized Enum named Result/Option
		if enum, ok := t.(*types.Enum); ok {
			if enum.Name == "Result" || enum.Name == "Option" {
				// For now, return types.Any for unparameterized Result/Option
				c.info.Types[x] = types.Any
				return types.Any
			}
		}

		c.add(diagAt("DTE0004", x.Span, "? operator can only be applied to Result or Option types, got "+t.String()))
		return nil

	case *ast.LambdaExpr:
		// Require explicit return type via lambda<RetType>
		var retType types.T
		if x.RetType != nil {
			if t, ok := types.FromName(x.RetType.Name); ok {
				retType = t
			} else {
				// Try resolving as custom type
				retType = c.resolveType(x.RetType)
				if retType == nil {
					c.add(diagAt("DTE0004", x.Span, "unknown lambda return type: "+x.RetType.Name))
					return nil
				}
			}
		} else {
			c.add(diagAt("DTE0004", x.Span, "lambda requires explicit return type: lambda<RetType>"))
			return nil
		}

		// Require typed params
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

		// Create child scope for lambda body to access parameters
		savedScope := c.scope
		c.scope = NewScope(savedScope)
		for i, p := range x.Params {
			c.scope.Define(&Symbol{
				Name: p.Name.Name,
				Kind: SymVar,
				Type: params[i],
			})
		}
		bt := c.typ(x.Body)
		c.scope = savedScope // restore

		// Validate body type matches declared return type
		if bt != nil && !types.Equal(bt, retType) {
			c.add(diagAt("DTE0004", x.Span, "lambda body type mismatch: expected "+retType.String()+", got "+bt.String()))
			return nil
		}

		// Store the full function type for internal use
		c.info.Types[e] = types.FuncOf(params, retType, false)
		// Return JUST the return type so let binding matches: let x: int = lambda<int>...
		return retType

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
		c.info.Idents[x] = sym
		return sym.Type
	}
	return nil
}

// typCall performs overload resolution for calls and wires move tracking.
// IMPORTANT: We only use named-arg canonicalization when the call actually
// contains named arguments. Purely positional calls follow the legacy path
// to preserve existing behaviors (len diagnostics, borrow/move, etc.).
