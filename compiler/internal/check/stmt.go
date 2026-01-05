package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// NOTE: unchanged cases elided for brevity in your view; this is a full function.
// Paste the whole thing, replacing your existing checkStmt entirely.
func (c *checker) checkStmt(s ast.Stmt) {
	switch st := s.(type) {
	case *ast.LetStmt:
		// If there's an explicit type annotation, use it as expected type for RHS
		var expectedType types.T
		var t types.T

		if st.Type != nil {
			expectedType = c.resolveType(st.Type)
			// Even if type annotation doesn't resolve, we still check RHS
			// (important for lambdas which have func type)

			// Save and set expected type for bidirectional checking (if resolved)
			savedExpected := c.expected
			if expectedType != nil {
				c.expected = expectedType
			}
			rhs := c.typ(st.Value)
			c.expected = savedExpected

			// Use resolved type annotation if available, otherwise use RHS type
			if expectedType != nil {
				t = expectedType
				if rhs != nil && !types.Assignable(t, rhs) {
					c.add(diagAt("DTE0004", st.Span, "cannot assign '"+rhs.String()+"' to '"+t.String()+"'"))
				}

				// Propagate expected type to empty collection literals
				// This ensures lowering sees the concrete type (e.g., list[Person]) instead of list[none]
				if rhs != nil {
					// List: [] -> list[none]
					if l, ok := rhs.(*types.List); ok && l.Elem == types.None {
						if _, ok := t.(*types.List); ok {
							c.info.Types[st.Value] = t
						}
					}
					// Set: set() -> set[none]
					if s, ok := rhs.(*types.Set); ok && s.Elem == types.None {
						if _, ok := t.(*types.Set); ok {
							c.info.Types[st.Value] = t
						}
					}
					// Dict: {} -> dict[none, none]
					if d, ok := rhs.(*types.Dict); ok && d.Key == types.None && d.Val == types.None {
						if _, ok := t.(*types.Dict); ok {
							c.info.Types[st.Value] = t
						}
					}
				}
			} else {
				// Type annotation didn't resolve, use RHS type
				t = rhs
			}
		} else {
			// No type annotation: infer from RHS
			rhs := c.typ(st.Value)
			if rhs == nil {
				c.add(diagAt("DTE0004", st.Span, "missing type annotation and no value for variable '"+st.Name.Name+"'"))
			}
			t = rhs
		}

		// Special handling for lambdas: store the Func type (from info.Types) so variable is callable.
		// The return type was validated earlier via type annotation check.
		symType := t
		if _, ok := st.Value.(*ast.LambdaExpr); ok {
			if ft := c.info.Types[st.Value]; ft != nil {
				symType = ft // Use the Func type, not the return type
			}
		}

		// Handle tuple destructuring: let (a, b, c) = tuple_expr or let (first, *rest) = tuple_expr
		if len(st.Pattern) > 0 {
			// Unwrap TypeAlias for destructuring
			destType := symType
			if alias, ok := destType.(*types.TypeAlias); ok {
				destType = alias.Target
			}

			// Verify RHS is a tuple with matching element count
			tupType, ok := destType.(*types.Tuple)
			if !ok {
				c.add(diagAt("DTE0004", st.Span, "cannot destructure non-tuple type '"+symType.String()+"'"))
				return
			}

			// Check arity with rest pattern support
			hasRest := st.RestIndex >= 0
			if hasRest {
				// With rest: need at least (len(pattern) - 1) elements in tuple
				// e.g., let (first, *rest) = (1, 2, 3) - need at least 1 non-rest element
				minRequired := len(st.Pattern) - 1
				if len(tupType.Elems) < minRequired {
					c.add(diagAt("DTE0004", st.Span, "tuple destructuring arity mismatch: expected at least "+
						string(rune('0'+minRequired))+" elements for rest pattern, got "+string(rune('0'+len(tupType.Elems)))))
					return
				}
			} else {
				// Exact match required
				if len(tupType.Elems) != len(st.Pattern) {
					c.add(diagAt("DTE0004", st.Span, "tuple destructuring arity mismatch: expected "+
						string(rune('0'+len(tupType.Elems)))+" elements, got "+string(rune('0'+len(st.Pattern)))))
					return
				}
			}

			// Define a symbol for each pattern element
			for i, ident := range st.Pattern {
				if ident.Name == "_" {
					// Ignore pattern - don't bind
					continue
				}

				var elemType types.T
				if hasRest && i == st.RestIndex {
					// This is the rest pattern - get remaining elements as tuple
					// Calculate which tuple elements go to *rest
					beforeRest := st.RestIndex
					afterRest := len(st.Pattern) - (st.RestIndex + 1)
					restStart := beforeRest
					restEnd := len(tupType.Elems) - afterRest

					restElems := tupType.Elems[restStart:restEnd]
					if len(restElems) == 0 {
						// Empty rest - create empty tuple type
						elemType = types.TupleOf()
					} else {
						elemType = types.TupleOf(restElems...)
					}
				} else {
					// Regular element - map to correct tuple index
					var tupleIdx int
					if hasRest && i > st.RestIndex {
						// After rest: count from end
						afterRestPos := len(st.Pattern) - i // position from end (1-based)
						tupleIdx = len(tupType.Elems) - afterRestPos
					} else {
						// Before rest (or no rest): direct index
						tupleIdx = i
					}
					if tupleIdx >= 0 && tupleIdx < len(tupType.Elems) {
						elemType = tupType.Elems[tupleIdx]
					}
				}

				sym := &Symbol{
					Name:      ident.Name,
					Kind:      SymVar,
					Type:      elemType,
					Node:      st,
					IsMutable: st.Mutable,
				}
				_ = c.scope.Define(sym)
				c.info.Idents[&st.Pattern[i]] = sym
				c.info.Types[&st.Pattern[i]] = elemType
			}
		} else {
			// Single variable binding (normal let)
			sym := &Symbol{
				Name:      st.Name.Name,
				Kind:      SymVar,
				Type:      symType,
				Node:      st,
				IsMutable: st.Mutable, // Track let vs let mut
			}
			_ = c.scope.Define(sym)

			// Enrich Info
			c.info.Idents[&st.Name] = sym
			if t != nil {
				c.info.Types[&st.Name] = symType
			}
		}

	case *ast.AssignStmt:
		// width must match
		if len(st.LHS) != len(st.RHS) {
			c.add(diagAt("DTE0002", st.Span, "arity mismatch in assignment"))
			return
		}
		for i := range st.LHS {
			lt := st.LHS[i]
			rt := st.RHS[i]

			// Handle different LHS types
			switch lhs := lt.(type) {
			case *ast.Ident:
				// Original identifier assignment logic
				valT := c.typ(rt)
				sym := c.scope.Lookup(lhs.Name)
				if sym == nil {
					c.add(diagAt("DTE0001", lhs.Span, "undefined name: "+lhs.Name))
					continue
				}
				c.info.Idents[lhs] = sym
				if sym.Type != nil {
					c.info.Types[lhs] = sym.Type
				}
				if !types.Assignable(sym.Type, valT) {
					c.add(diagAt("DTE0004", st.Span, "cannot assign '"+valT.String()+"' to '"+sym.Type.String()+"'"))
				}

			case *ast.FieldExpr:
				// Field assignment: obj.field = value OR ClassName.static_field = value
				valT := c.typ(rt)

				// Check if this is a type access (ClassName.FIELD) for static fields
				isTypeAccess := false
				if ident, ok := lhs.X.(*ast.Ident); ok {
					if sym := c.scope.Lookup(ident.Name); sym != nil && sym.Kind == SymType {
						isTypeAccess = true
					}
				}

				objType := c.typ(lhs.X)

				// Check if object type is a class (or generic class)
				var cls *types.Class
				var subst map[string]types.T

				if c, ok := objType.(*types.Class); ok {
					cls = c
				} else if gen, ok := objType.(*types.Generic); ok {
					if c, ok := gen.Base.(*types.Class); ok {
						cls = c
						// Build substitution map for generic class
						subst = make(map[string]types.T)
						if len(cls.TypeParams) == len(gen.Args) {
							for i, tp := range cls.TypeParams {
								subst[tp.Name] = gen.Args[i]
							}
						}
					}
				}

				if cls == nil {
					c.add(diagAt("DTE0004", lhs.Span, "cannot assign to field of non-class type"))
					continue
				}

				fieldName := lhs.Name.Name

				// If it's type access, check for static fields
				if isTypeAccess {
					var staticField *types.ClassStaticField
					curr := cls
					for curr != nil {
						if sf, found := curr.StaticFields[fieldName]; found {
							staticField = sf
							break
						}
						curr = curr.Base
					}

					if staticField != nil {
						// Check if mutable
						if !staticField.IsMut {
							c.add(diagAt("DTE0004", lhs.Name.Span, "cannot assign to immutable static field '"+fieldName+"'"))
							continue
						}

						// Visibility check
						if !staticField.IsPub {
							allowed := false
							if selfSym := c.scope.Lookup("self"); selfSym != nil {
								if selfType, ok := selfSym.Type.(*types.Class); ok {
									if types.IsSubclass(selfType, cls) {
										allowed = true
									}
								}
							}
							if !allowed {
								c.add(diagAt("DTE0010", lhs.Name.Span, "static field '"+fieldName+"' is private"))
								continue
							}
						}

						// Type check
						if !types.Assignable(staticField.Type, valT) {
							c.add(diagAt("DTE0004", st.Span, "cannot assign '"+valT.String()+"' to static field of type '"+staticField.Type.String()+"'"))
						}
						continue
					}
				}

				// Find the instance field
				var field *types.Field
				curr := cls
				for curr != nil {
					for i := range curr.Fields {
						if curr.Fields[i].Name == fieldName {
							field = &curr.Fields[i]
							break
						}
					}
					if field != nil {
						break
					}
					curr = curr.Base
				}

				if field == nil {
					c.add(diagAt("DTE0001", lhs.Name.Span, "no such field: "+fieldName))
					continue
				}

				// Visibility check
				if !field.IsPub {
					// Check if we're inside a method of the same class
					allowed := false
					if selfSym := c.scope.Lookup("self"); selfSym != nil {
						if selfType, ok := selfSym.Type.(*types.Class); ok {
							if types.Equal(selfType, cls) {
								allowed = true
							}
						}
					}
					if !allowed {
						c.add(diagAt("DTE0010", lhs.Name.Span, "field '"+fieldName+"' is private"))
						continue
					}
				}

				// Mutability check: only allow assignment to mutable fields (or in __new__)
				if !field.IsMut && c.curFuncName != "__new__" {
					c.add(diagAt("DCL0004", lhs.Name.Span, "cannot assign to immutable field '"+fieldName+"'"))
					continue
				}

				// Reassignment operator check: outside __new__, field assignment requires ':='
				if c.curFuncName != "__new__" && !st.IsReassign {
					c.add(diagAt("DTE0013", lhs.Span, "field mutation requires ':=' not '=' (e.g., obj.field := value)"))
					continue
				}

				// Type check (apply substitution for generic classes)
				fieldType := field.Type
				if subst != nil {
					fieldType = substitute(fieldType, subst)
				}
				if !types.Assignable(fieldType, valT) {
					dstStr := "?"
					if fieldType != nil {
						dstStr = fieldType.String()
					}
					srcStr := "?"
					if valT != nil {
						srcStr = valT.String()
					}
					c.add(diagAt("DTE0004", st.Span, "cannot assign '"+srcStr+"' to field of type '"+dstStr+"'"))
				}

			case *ast.IndexExpr:
				// Index assignment: obj[idx] := value (mutation)
				// Check that := was used, not =
				if !st.IsReassign {
					c.add(diagAt("DTE0013", st.Span, "index mutation requires ':=' not '=' (e.g., list[0] := value)"))
					continue
				}

				objType := c.typ(lhs.X)
				idxType := c.typ(lhs.Idx)
				valT := c.typ(rt)

				// Check if objType is a custom class with __setitem__
				if cls, ok := objType.(*types.Class); ok {
					if setitem, found := cls.Dunders["__setitem__"]; found {
						// __setitem__(self, index, value) -> none
						if len(setitem.Params) == 3 {
							// Check index type
							if !types.Assignable(setitem.Params[1], idxType) {
								c.add(diagAt("DTE0004", lhs.Idx.SpanOf(), "index type mismatch for __setitem__"))
								continue
							}
							// Check value type
							if !types.Assignable(setitem.Params[2], valT) {
								c.add(diagAt("DTE0004", st.Span, "value type mismatch for __setitem__"))
								continue
							}
							// Valid __setitem__ call
							continue
						}
					}
				}

				// For built-in types (lists, dicts), check mutability
				// and validate element types
				if _, ok := objType.(*types.List); ok {
					// Check if the list variable is mutable
					if ident, ok := lhs.X.(*ast.Ident); ok {
						sym := c.scope.Lookup(ident.Name)
						if sym != nil && !sym.IsMutable {
							c.add(diagAt("DCL0004", ident.Span, "cannot assign to element of immutable list '"+ident.Name+"' (use 'let mut' instead of 'let')"))
							continue
						}
					}
				}
				if _, ok := objType.(*types.Dict); ok {
					// Check if the dict variable is mutable
					if ident, ok := lhs.X.(*ast.Ident); ok {
						sym := c.scope.Lookup(ident.Name)
						if sym != nil && !sym.IsMutable {
							c.add(diagAt("DCL0004", ident.Span, "cannot assign to element of immutable dict '"+ident.Name+"' (use 'let mut' instead of 'let')"))
							continue
						}
					}
				}

			default:
				// Unsupported LHS
				_ = c.typ(lt)
				_ = c.typ(rt)
			}
		}

	case *ast.AugAssignStmt:
		lt := c.typ(st.Left)
		rt := c.typ(st.Right)
		op := st.Op
		// Treat as binary op type check on the underlying op (e.g., "+=" -> "+").
		be := &ast.BinaryExpr{Op: op[:len(op)-1], Lhs: st.Left, Rhs: st.Right, Span: st.Span}
		_ = c.typBinary(be)

		if lt != nil && rt != nil {
			// We expect the binary result type to be the same as LHS for a valid AugAssign.
			if resT, ok := beResultType(be.Op, lt, rt); !ok || !types.Equal(lt, resT) {
				c.add(diagAt("DTE0004", st.Span, "invalid augmented assignment"))
			}
		}

	case *ast.PassStmt:
		// pass is a no-op - nothing to type check

	case *ast.ReturnStmt:
		if st.Value == nil {
			// returning none is always fine if declared none
			if c.curFuncRet != nil && !types.Equal(c.curFuncRet, types.None) {
				c.add(diagAt("DTE0005", st.Span, "missing return value"))
			}
			return
		}
		rt := c.typ(st.Value)
		if c.curFuncRet != nil && !types.Equal(rt, c.curFuncRet) {
			c.add(diagAt("DTE0004", st.Span, "wrong return type: expected "+c.curFuncRet.String()))
		}

	case *ast.ExprStmt:
		_ = c.typ(st.Expr)

	case *ast.MatchExpr:
		_ = c.checkMatchExpr(st)

	case *ast.SelectStmt:
		c.checkSelectStmt(st)

	case *ast.IfStmt:
		condType := c.typ(st.Cond)

		// Check if condition is a class type with __bool__ dunder
		if condType != nil {
			if cls, ok := condType.(*types.Class); ok {
				if _, hasBool := cls.Dunders["__bool__"]; hasBool {
					// Track this expression for lowering to emit __bool__ call
					c.info.BoolConversions[st.Cond] = cls
				} else if !types.Equal(condType, types.Bool) {
					c.add(diagAt("DTE0004", st.Cond.SpanOf(), "condition must be bool or implement __bool__"))
				}
			}
		}

		// If condition is IsExpr with bindings, push scope with bound variables
		if st.Then != nil {
			// Check for IsExpr condition with bindings
			if isExpr, ok := st.Cond.(*ast.IsExpr); ok && !isExpr.Negated {
				if bindings := c.info.IsBindings[isExpr]; len(bindings) > 0 {
					// Push scope with bindings for then block
					c.scope = NewScope(c.scope)
					for _, b := range bindings {
						c.scope.Define(&Symbol{
							Name: b.Name,
							Kind: SymVar,
							Type: b.Type,
							Node: b.Node,
						})
					}
					c.checkBlock(st.Then)
					c.scope = c.scope.parent // Pop scope
				} else {
					c.checkBlock(st.Then)
				}
			} else {
				c.checkBlock(st.Then)
			}
		}
		for _, arm := range st.Elifs {
			_ = c.typ(arm.Cond)
			if arm.Body != nil {
				c.checkBlock(arm.Body)
			}
		}
		if st.Else != nil {
			c.checkBlock(st.Else)
		}

	case *ast.WhileStmt:
		_ = c.typ(st.Cond)
		if st.Body != nil {
			c.checkBlock(st.Body)
		}

	case *ast.ForStmt:
		// Get the iterable type
		iterType := c.typ(st.Iter)

		// Check if source collection is mutable (needed for mutable loop targets)
		isSourceMutable := false
		if id, ok := st.Iter.(*ast.Ident); ok {
			if sym := c.scope.Lookup(id.Name); sym != nil {
				isSourceMutable = sym.IsMutable
			}
		} else if callExpr, ok := st.Iter.(*ast.CallExpr); ok {
			// For dict.items() or enumerate(), check if the source is mutable
			if fieldExpr, ok := callExpr.Callee.(*ast.FieldExpr); ok {
				if id, ok := fieldExpr.X.(*ast.Ident); ok {
					if sym := c.scope.Lookup(id.Name); sym != nil {
						isSourceMutable = sym.IsMutable
					}
				}
			} else if id, ok := callExpr.Callee.(*ast.Ident); ok {
				// For enumerate(items), check the inner iterable
				if id.Name == "enumerate" && len(callExpr.Args) > 0 {
					if innerIdent, ok := callExpr.Args[0].(*ast.Ident); ok {
						if sym := c.scope.Lookup(innerIdent.Name); sym != nil {
							isSourceMutable = sym.IsMutable
						}
					}
				}
			}
		}

		// Check if this is enumerate() iteration
		isEnumerate := false
		var enumerateIterType types.T
		if callExpr, ok := st.Iter.(*ast.CallExpr); ok {
			if id, ok := callExpr.Callee.(*ast.Ident); ok && id.Name == "enumerate" {
				if len(callExpr.Args) == 1 {
					isEnumerate = true
					// Get the type of the inner iterable
					enumerateIterType = c.typ(callExpr.Args[0])
				}
			}
		}

		// Check if this is dict.items() iteration
		isDictItems := false
		var dictType *types.Dict
		if callExpr, ok := st.Iter.(*ast.CallExpr); ok {
			if fieldExpr, ok := callExpr.Callee.(*ast.FieldExpr); ok {
				if fieldExpr.Name.Name == "items" {
					if dt, ok := c.info.Types[fieldExpr.X].(*types.Dict); ok {
						isDictItems = true
						dictType = dt
					}
				}
			}
		}

		// Check if this is zip(a, b) iteration
		isZip := false
		var zipElemType1, zipElemType2 types.T
		if callExpr, ok := st.Iter.(*ast.CallExpr); ok {
			if id, ok := callExpr.Callee.(*ast.Ident); ok && id.Name == "zip" {
				if len(callExpr.Args) == 2 {
					isZip = true
					// Get element types from both iterables
					iter1Type := c.typ(callExpr.Args[0])
					iter2Type := c.typ(callExpr.Args[1])
					if list1, ok := iter1Type.(*types.List); ok {
						zipElemType1 = list1.Elem
					}
					if list2, ok := iter2Type.(*types.List); ok {
						zipElemType2 = list2.Elem
					}
				}
			}
		}

		// Handle enumerate() with two targets: (index, element)
		if isEnumerate && len(st.Targets) == 2 {
			// Get element type from inner iterable
			var elemType types.T
			if enumerateIterType != nil {
				switch it := enumerateIterType.(type) {
				case *types.List:
					elemType = it.Elem
				case *types.Set:
					elemType = it.Elem
				default:
					elemType = types.Int // Fallback
				}
			}

			// First target: index (always int)
			if st.Targets[0].Name != nil {
				var idxType types.T = types.Int
				if st.Targets[0].Type != nil {
					idxType = c.resolveType(st.Targets[0].Type)
				}
				sym := &Symbol{
					Name:      st.Targets[0].Name.Name,
					Kind:      SymVar,
					Type:      idxType,
					IsMutable: false, // Index is always immutable
				}
				_ = c.scope.Define(sym)
				c.info.Idents[st.Targets[0].Name] = sym
				c.info.Types[st.Targets[0].Name] = idxType
			}

			// Second target: element (can be mutable if source is mutable)
			if st.Targets[1].IsMut && !isSourceMutable {
				c.add(diagAt("DTE0004", st.Targets[1].Name.Span,
					"cannot mutate elements of immutable collection (use 'let mut' to declare the collection)"))
			}
			if st.Targets[1].Name != nil {
				valType := elemType
				if st.Targets[1].Type != nil {
					valType = c.resolveType(st.Targets[1].Type)
				}
				sym := &Symbol{
					Name:      st.Targets[1].Name.Name,
					Kind:      SymVar,
					Type:      valType,
					IsMutable: st.Targets[1].IsMut && isSourceMutable,
				}
				_ = c.scope.Define(sym)
				c.info.Idents[st.Targets[1].Name] = sym
				if valType != nil {
					c.info.Types[st.Targets[1].Name] = valType
				}
			}
		} else if isDictItems && len(st.Targets) == 2 && dictType != nil {
			// First target: key type (keys are always immutable)
			if st.Targets[0].IsMut {
				c.add(diagAt("DTE0004", st.Targets[0].Name.Span,
					"dict keys cannot be mutable during iteration"))
			}
			keyType := dictType.Key
			if st.Targets[0].Type != nil {
				keyType = c.resolveType(st.Targets[0].Type)
			}
			if st.Targets[0].Name != nil {
				sym := &Symbol{
					Name:      st.Targets[0].Name.Name,
					Kind:      SymVar,
					Type:      keyType,
					IsMutable: false, // Keys are always immutable
				}
				_ = c.scope.Define(sym)
				c.info.Idents[st.Targets[0].Name] = sym
				if keyType != nil {
					c.info.Types[st.Targets[0].Name] = keyType
				}
			}

			// Second target: value type (can be mutable if source is mutable)
			if st.Targets[1].IsMut && !isSourceMutable {
				c.add(diagAt("DTE0004", st.Targets[1].Name.Span,
					"cannot mutate elements of immutable collection (use 'let mut' to declare the dict)"))
			}
			valType := dictType.Val
			if st.Targets[1].Type != nil {
				valType = c.resolveType(st.Targets[1].Type)
			}
			if st.Targets[1].Name != nil {
				sym := &Symbol{
					Name:      st.Targets[1].Name.Name,
					Kind:      SymVar,
					Type:      valType,
					IsMutable: st.Targets[1].IsMut && isSourceMutable,
				}
				_ = c.scope.Define(sym)
				c.info.Idents[st.Targets[1].Name] = sym
				if valType != nil {
					c.info.Types[st.Targets[1].Name] = valType
				}
			}
		} else if isZip && len(st.Targets) == 2 {
			// First target: element from first list
			if st.Targets[0].Name != nil {
				elemType1 := zipElemType1
				if st.Targets[0].Type != nil {
					elemType1 = c.resolveType(st.Targets[0].Type)
				}
				sym := &Symbol{
					Name:      st.Targets[0].Name.Name,
					Kind:      SymVar,
					Type:      elemType1,
					IsMutable: st.Targets[0].IsMut && isSourceMutable,
				}
				_ = c.scope.Define(sym)
				c.info.Idents[st.Targets[0].Name] = sym
				if elemType1 != nil {
					c.info.Types[st.Targets[0].Name] = elemType1
				}
			}

			// Second target: element from second list
			if st.Targets[1].Name != nil {
				elemType2 := zipElemType2
				if st.Targets[1].Type != nil {
					elemType2 = c.resolveType(st.Targets[1].Type)
				}
				sym := &Symbol{
					Name:      st.Targets[1].Name.Name,
					Kind:      SymVar,
					Type:      elemType2,
					IsMutable: st.Targets[1].IsMut && isSourceMutable,
				}
				_ = c.scope.Define(sym)
				c.info.Idents[st.Targets[1].Name] = sym
				if elemType2 != nil {
					c.info.Types[st.Targets[1].Name] = elemType2
				}
			}
		} else {
			// Standard list/set/tuple iteration
			var elemType types.T
			if iterType != nil {
				switch it := iterType.(type) {
				case *types.List:
					elemType = it.Elem
				case *types.Set:
					elemType = it.Elem
				case *types.Tuple:
					// Homogeneous tuple iteration: all elements must be same type
					if len(it.Elems) == 0 {
						c.add(diagAt("DTE0004", st.Iter.SpanOf(), "cannot iterate over empty tuple"))
						return
					}
					firstType := it.Elems[0]
					isHomogeneous := true
					for i := 1; i < len(it.Elems); i++ {
						if !types.Equal(it.Elems[i], firstType) {
							isHomogeneous = false
							break
						}
					}
					if !isHomogeneous {
						c.add(diagAt("DTE0004", st.Iter.SpanOf(),
							"tuple iteration requires all elements to be the same type"))
						return
					}
					elemType = firstType
				default:
					// For range() and other iterables, assume int for now
					elemType = types.Int
				}
			}

			// Bind loop variables from Targets
			if len(st.Targets) > 0 {
				for _, tgt := range st.Targets {
					// Validate mutable iteration
					if tgt.IsMut && !isSourceMutable {
						c.add(diagAt("DTE0004", tgt.Name.Span,
							"cannot mutate elements of immutable collection (use 'let mut' to declare the collection)"))
					}

					var varType types.T
					if tgt.Type != nil {
						// Explicit type annotation
						varType = c.resolveType(tgt.Type)
						// Validate against element type
						if elemType != nil && varType != nil && !types.Assignable(varType, elemType) {
							c.add(diagAt("DTE0004", tgt.Name.Span, "loop variable type '"+varType.String()+"' does not match element type '"+elemType.String()+"'"))
						}
					} else {
						// Infer from collection element type
						varType = elemType
					}

					if tgt.Name != nil {
						sym := &Symbol{
							Name:      tgt.Name.Name,
							Kind:      SymVar,
							Type:      varType,
							IsMutable: tgt.IsMut && isSourceMutable,
						}
						_ = c.scope.Define(sym)
						c.info.Idents[tgt.Name] = sym
						if varType != nil {
							c.info.Types[tgt.Name] = varType
						}
					}
				}
			}
		}

		if st.Body != nil {
			c.checkBlock(st.Body)
		}

	case *ast.DeferStmt:
		if st.Call != nil {
			_ = c.typ(st.Call)
		}

	case *ast.UsingStmt:
		// Bind the using identifier to the scope as Arena type
		if id, ok := st.Bind.(*ast.Ident); ok {
			sym := &Symbol{
				Name: id.Name,
				Kind: SymVar,
				Type: types.ArenaOf(),
			}
			_ = c.scope.Define(sym)
			c.info.Idents[id] = sym
		}
		if st.Init != nil {
			_ = c.typ(st.Init)
		}
		if st.Body != nil {
			c.checkBlock(st.Body)
		}

	case *ast.UnsafeBlock:
		// Enter unsafe context for the duration of the block.
		c.unsafeDepth++
		if st.Body != nil {
			c.checkBlock(st.Body)
		}
		c.unsafeDepth--

	// NEW (M5): imports
	case *ast.ImportStmt:
		// Track stdlib module imports for dead-code elimination
		// Stdlib modules: log, json, http, etc.
		if len(st.Path) == 1 {
			modName := st.Path[0]
			if StdlibModules[modName] {
				c.info.StdlibImports[modName] = true
			}
		}
		// nothing else to type; resolver handles validity; lints post-check
	case *ast.FromImportStmt:
		// nothing to type; resolver handles validity; lints post-check

	case *ast.SpawnStmt:
		// Type check the spawn body block
		// Also verify captured variables implement Send trait
		if st.Body != nil {
			// Collect free variables used in the spawn block
			// that are defined in the outer scope
			captured := c.collectCapturedVariables(st.Body)

			// Check each captured variable for Send safety
			for varName, sym := range captured {
				if sym.Type != nil && !types.IsSend(sym.Type) {
					c.add(diagAt("DTE0020", st.Span,
						"cannot spawn with captured variable '"+varName+"' of non-Send type '"+sym.Type.String()+"'"))
				}
			}

			c.checkBlock(st.Body)
		}
	}

}

// beResultType returns (resultType, ok) for a binary operator (op) applied to (lt, rt).
// Phase-1: exact same-type numeric ops; string+string for "+"; boolean on logical ops & equality for same primitives.
func beResultType(op string, lt, rt types.T) (types.T, bool) {
	switch op {
	case "+": // addition OR string concatenation
		// numeric + numeric (same type)
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		if types.Equal(lt, types.Float) && types.Equal(rt, types.Float) {
			return types.Float, true
		}
		// string + string -> string
		if types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
			return types.Str, true
		}
		return nil, false

	case "-", "*", "/", "%":
		// numeric only; same-type
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		if types.Equal(lt, types.Float) && types.Equal(rt, types.Float) {
			return types.Float, true
		}
		return nil, false

	case "|", "&", "^":
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		return nil, false

	case "<<", ">>":
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		return nil, false

	case "==", "!=":
		// Equality only on same primitives (int/float/bool/str) for this phase.
		if types.Equal(lt, rt) && (isPrimitive(lt)) {
			return types.Bool, true
		}
		return nil, false

	case "<", "<=", ">", ">=":
		// Numeric comparisons only; same-type
		if (types.Equal(lt, types.Int) && types.Equal(rt, types.Int)) ||
			(types.Equal(lt, types.Float) && types.Equal(rt, types.Float)) {
			return types.Bool, true
		}
		return nil, false

	case "and", "or":
		if types.Equal(lt, types.Bool) && types.Equal(rt, types.Bool) {
			return types.Bool, true
		}
		return nil, false

	case "|>":
		// Pipeline typed elsewhere
		return nil, false

	default:
		return nil, false
	}
}

// helper: primitive means the simple builtins we handle here.
func isPrimitive(t types.T) bool {
	return types.Equal(t, types.Int) ||
		types.Equal(t, types.Float) ||
		types.Equal(t, types.Bool) ||
		types.Equal(t, types.Str)
}

// collectCapturedVariables walks an AST block and finds identifiers that reference
// variables from outer scopes (free variables / captures).
// Returns a map of variable name -> Symbol for captured variables.
func (c *checker) collectCapturedVariables(block *ast.Block) map[string]*Symbol {
	captured := make(map[string]*Symbol)
	defined := make(map[string]bool)

	// Walk all statements in the block
	var walkExpr func(e ast.Expr)
	var walkStmt func(s ast.Stmt)

	walkExpr = func(e ast.Expr) {
		if e == nil {
			return
		}
		switch x := e.(type) {
		case *ast.Ident:
			// If not locally defined and exists in outer scope, it's captured
			if !defined[x.Name] {
				if sym := c.scope.Lookup(x.Name); sym != nil && sym.Kind == SymVar {
					captured[x.Name] = sym
				}
			}
		case *ast.BinaryExpr:
			walkExpr(x.Lhs)
			walkExpr(x.Rhs)
		case *ast.UnaryExpr:
			walkExpr(x.X)
		case *ast.CallExpr:
			walkExpr(x.Callee)
			for _, arg := range x.Args {
				walkExpr(arg)
			}
			for _, arg := range x.ArgNodes {
				walkExpr(arg.Expr)
			}
		case *ast.FieldExpr:
			walkExpr(x.X)
		case *ast.IndexExpr:
			walkExpr(x.X)
			walkExpr(x.Idx)
		case *ast.ListLit:
			for _, el := range x.Elems {
				walkExpr(el)
			}
		case *ast.DictLit:
			for _, k := range x.Keys {
				walkExpr(k)
			}
			for _, v := range x.Values {
				walkExpr(v)
			}
		case *ast.TupleLit:
			for _, el := range x.Elems {
				walkExpr(el)
			}
		}
	}

	walkStmt = func(s ast.Stmt) {
		if s == nil {
			return
		}
		switch x := s.(type) {
		case *ast.LetStmt:
			// First walk RHS to find captures, then mark name as defined
			walkExpr(x.Value)
			defined[x.Name.Name] = true
			for _, p := range x.Pattern {
				defined[p.Name] = true
			}
		case *ast.AssignStmt:
			for _, lhs := range x.LHS {
				walkExpr(lhs)
			}
			for _, rhs := range x.RHS {
				walkExpr(rhs)
			}
		case *ast.ExprStmt:
			walkExpr(x.Expr)
		case *ast.ReturnStmt:
			walkExpr(x.Value)
		case *ast.IfStmt:
			walkExpr(x.Cond)
			if x.Then != nil {
				for _, stmt := range x.Then.Stmts {
					walkStmt(stmt)
				}
			}
			if x.Else != nil {
				for _, stmt := range x.Else.Stmts {
					walkStmt(stmt)
				}
			}
		case *ast.WhileStmt:
			walkExpr(x.Cond)
			if x.Body != nil {
				for _, stmt := range x.Body.Stmts {
					walkStmt(stmt)
				}
			}
		case *ast.ForStmt:
			walkExpr(x.Iter)
			for _, tgt := range x.Targets {
				if tgt.Name != nil {
					defined[tgt.Name.Name] = true
				}
			}
			if x.Body != nil {
				for _, stmt := range x.Body.Stmts {
					walkStmt(stmt)
				}
			}
		}
	}

	// Walk all statements in the block
	if block != nil {
		for _, stmt := range block.Stmts {
			walkStmt(stmt)
		}
	}

	return captured
}
