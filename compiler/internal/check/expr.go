package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
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
		// Not modeled yet
		return nil
	case *ast.IndexExpr:
		// Not modeled yet
		return nil

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
		c.info.Types[e] = types.FuncOf(params, bt)
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

func (c *checker) typBinary(x *ast.BinaryExpr) types.T {
	op := x.Op

	// Helpers: classify numeric families via canonical String() spellings.
	intInfo := func(t types.T) (signed bool, width int, ok bool) {
		if t == nil {
			return false, 0, false
		}
		switch t.String() {
		case "int":
			return true, 0, true // width==0 marks unsized 'int'
		case "isize":
			return true, 64, true
		case "usize":
			return false, 64, true
		case "i8":
			return true, 8, true
		case "i16":
			return true, 16, true
		case "i32":
			return true, 32, true
		case "i64":
			return true, 64, true
		case "i128":
			return true, 128, true
		case "u8":
			return false, 8, true
		case "u16":
			return false, 16, true
		case "u32":
			return false, 32, true
		case "u64":
			return false, 64, true
		case "u128":
			return false, 128, true
		default:
			return false, 0, false
		}
	}
	floatInfo := func(t types.T) (width int, ok bool) {
		if t == nil {
			return 0, false
		}
		switch t.String() {
		case "f32":
			return 32, true
		case "float", "f64": // 'float' is our f64 alias
			return 64, true
		default:
			return 0, false
		}
	}

	switch op {
	case "+", "-", "*", "/", "%", "**":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)

		// String ergonomics: allow str + (int|float|bool|str) => str
		if op == "+" && (types.Equal(lt, types.Str) || types.Equal(rt, types.Str)) {
			if types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
				c.info.Types[x] = types.Str
				return types.Str
			}
			other := rt
			if types.Equal(lt, types.Str) {
				other = rt
			} else {
				other = lt
			}
			if types.Equal(other, types.Int) || types.Equal(other, types.Float) ||
				types.Equal(other, types.Bool) || types.Equal(other, types.Str) {
				c.info.Types[x] = types.Str
				return types.Str
			}
		}

		// Integers: same signedness & same width, with one exception:
		//   M9 allowance: (isize|usize) with unsized int ==> OK, result keeps pointer-sized type.
		if ls, lw, lok := intInfo(lt); lok {
			if rs, rw, rok := intInfo(rt); rok {
				allowIntPtrMix := (lw == 0 && (rw == 64)) || (rw == 0 && (lw == 64))
				if !allowIntPtrMix {
					if ls != rs {
						c.add(diagAt("DNT0002", x.Span, "")) // signed/unsigned mismatch
						return nil
					}
					if lw != rw {
						c.add(diagAt("DNT0001", x.Span, "")) // width mismatch
						return nil
					}
					// OK: same family — result type is the left (equal to right)
					c.info.Types[x] = lt
					return lt
				}
				// allow int <op> (isize|usize)
				if lw == 0 && rw == 64 {
					c.info.Types[x] = rt
					return rt
				}
				if rw == 0 && lw == 64 {
					c.info.Types[x] = lt
					return lt
				}
			}
		}

		// Floats: require same width (f32 with f32; f64/float with f64/float)
		if lw, lok := floatInfo(lt); lok {
			if rw, rok := floatInfo(rt); rok {
				if lw != rw {
					c.add(diagAt("DNT0001", x.Span, ""))
					return nil
				}
				if lw == 32 {
					c.info.Types[x] = types.F32
					return types.F32
				}
				c.info.Types[x] = types.Float // f64 alias
				return types.Float
			}
		}

		// Any other combination is invalid for now.
		c.add(diagAt("DTE0004", x.Span, "invalid operands for '"+op+"'"))
		return nil

	case "|", "&", "^":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		// Keep legacy behavior: bitwise ops require plain 'int'
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			c.info.Types[x] = types.Int
			return types.Int
		}
		c.add(diagAt("DTE0004", x.Span, "bitwise operators require int operands"))
		return nil

	case "<<", ">>":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		// Keep legacy behavior: shifts require plain 'int'
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			c.info.Types[x] = types.Int
			return types.Int
		}
		c.add(diagAt("DTE0004", x.Span, "bitwise operators require int operands"))
		return nil

	case "|>":
		// pipeline: lhs |> f(a,b)  ==>  f(lhs, a, b)
		call, ok := x.Rhs.(*ast.CallExpr)
		if !ok {
			c.add(diagAt("DTE0103", x.Span, "pipeline expects a call on the right-hand side"))
			return nil
		}
		id, ok := call.Callee.(*ast.Ident)
		if !ok || id == nil {
			c.add(diagAt("DTE0103", x.Span, "pipeline target must be an identifier"))
			return nil
		}

		lhsT := c.typ(x.Lhs)
		set := c.info.Funcs[id.Name]
		if set == nil || len(set.Cands) == 0 {
			c.add(diagAt("DTE0001", id.Span, "pipeline target undefined function: "+id.Name))
			return nil
		}

		base := callArgs(call)
		hasNamed := false
		for _, a := range base {
			if a.Name != nil {
				hasNamed = true
				break
			}
		}

		if !hasNamed {
			// Legacy positional: prepend lhs, then resolve
			args := make([]types.T, 0, 1+len(base))
			args = append(args, lhsT)
			for _, a := range base {
				args = append(args, c.typ(a.Expr))
			}
			arityCands := filterByArity(set.Cands, len(args))
			if len(arityCands) == 0 {
				c.add(diagAt("DTE0046", x.Span, "pipeline arity mismatch"))
				return nil
			}
			exact := filterExactByTypes(arityCands, args)
			switch len(exact) {
			case 1:
				chosen := exact[0]
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				ret := chosen.Type.Ret
				c.info.Types[x] = ret
				return ret
			case 0:
				c.add(diagAt("DTE0101", x.Span, "pipeline has no matching overload for call to "+id.Name))
				return nil
			default:
				c.add(diagAt("DTE0102", x.Span, "pipeline ambiguous overload for call to "+id.Name))
				return nil
			}
		}

		// Named-args pipeline: synthesize [lhs]+args and run per-candidate mapping
		synth := make([]ast.CallArg, 0, 1+len(base))
		synth = append(synth, ast.CallArg{Expr: &ast.Ident{Name: "<pipe>", Span: x.Lhs.SpanOf()}})
		synth = append(synth, base...)

		var exact []*FuncCand
		for _, cand := range set.Cands {
			if cand.Type == nil || len(cand.Type.Params) == 0 {
				continue
			}
			vec, ok := c.canonicalizeForCandidate(cand, synth)
			if !ok {
				continue
			}
			vec[0] = lhsT // force first param to be lhsT
			if typesMatchExactly(cand.Type.Params, vec) {
				exact = append(exact, cand)
			}
		}
		switch len(exact) {
		case 1:
			chosen := exact[0]
			if chosen.Extern && c.unsafeDepth == 0 {
				c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
			}
			ret := chosen.Type.Ret
			c.info.Types[x] = ret
			return ret
		case 0:
			c.add(diagAt("DTE0101", x.Span, "pipeline has no matching overload for call to "+id.Name))
			return nil
		default:
			c.add(diagAt("DTE0102", x.Span, "pipeline ambiguous overload for call to "+id.Name))
			return nil
		}

	case "<", "<=", ">", ">=",
		"==", "!=":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)

		// Integers: same signedness & width, with the same M9 exception as above.
		if ls, lw, lok := intInfo(lt); lok {
			if rs, rw, rok := intInfo(rt); rok {
				allowIntPtrMix := (lw == 0 && (rw == 64)) || (rw == 0 && (lw == 64))
				if !allowIntPtrMix {
					if ls != rs {
						c.add(diagAt("DNT0002", x.Span, ""))
						return nil
					}
					if lw != rw {
						c.add(diagAt("DNT0001", x.Span, ""))
						return nil
					}
					c.info.Types[x] = types.Bool
					return types.Bool
				}
				c.info.Types[x] = types.Bool
				return types.Bool
			}
		}

		// Floats: same width
		if lw, lok := floatInfo(lt); lok {
			if rw, rok := floatInfo(rt); rok {
				if lw != rw {
					c.add(diagAt("DNT0001", x.Span, ""))
					return nil
				}
				c.info.Types[x] = types.Bool
				return types.Bool
			}
		}

		// Fallback: identical non-numeric types comparable
		if types.Equal(lt, rt) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "incomparable operands for '"+op+"'"))
		return nil
	default:
		return nil
	}
}

// typCall performs overload resolution for calls and wires move tracking.
// IMPORTANT: We only use named-arg canonicalization when the call actually
// contains named arguments. Purely positional calls follow the legacy path
// to preserve existing behaviors (len diagnostics, borrow/move, etc.).
func (c *checker) typCall(call *ast.CallExpr) types.T {
	// --- Case 0: direct call of a lambda: (lambda ...)(args)
	if l, ok := call.Callee.(*ast.LambdaExpr); ok {
		_ = c.typ(l)
		// Lambdas are positional-only in this milestone.
		if len(call.Args) != len(l.Params) {
			c.add(diagAt("DTE0046", l.Span, "arity mismatch: wrong number of arguments"))
			return nil
		}
		for i := range l.Params {
			var pt types.T
			if l.Params[i].Type != nil {
				if t, ok := types.FromName(l.Params[i].Type.Name); ok {
					pt = t
				}
			}
			at := c.typ(call.Args[i])
			if pt == nil || at == nil || !types.Equal(pt, at) {
				c.add(diagAt("DTE0104", call.Span, "argument type mismatch"))
				return nil
			}
		}
		if ft, ok := c.info.Types[l].(*types.Func); ok {
			c.info.Types[call] = ft.Ret
			return ft.Ret
		}
		return nil
	}

	// Common helpers
	argsNodes := callArgs(call)
	hasNamed := false
	for _, a := range argsNodes {
		if a.Name != nil {
			hasNamed = true
			break
		}
	}

	// --- Case 1: module-qualified call: mod.fn(...)
	if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		if set, base, isImport := c.moduleQualifiedOverloadSet(fe); isImport {
			if !hasNamed {
				// Legacy positional path
				args := make([]types.T, len(argsNodes))
				for i, a := range argsNodes {
					args[i] = c.typ(a.Expr)
				}
				if set == nil || len(set.Cands) == 0 {
					c.add(diagAt("DME0003", fe.Name.Span, base.Name+" has no exported '"+fe.Name.Name+"'"))
					return nil
				}
				arityCands := filterByArity(set.Cands, len(args))
				if len(arityCands) == 0 {
					c.add(diagAt("DTE0046", fe.Name.Span, "arity mismatch: wrong number of arguments"))
					return nil
				}
				exact := filterExactByTypes(arityCands, args)
				switch len(exact) {
				case 1:
					chosen := exact[0]
					if chosen.Extern && c.unsafeDepth == 0 {
						c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
					}
					// Synthesize positional Args for borrow/move enforcement
					tmp := *call
					tmp.Args = make([]ast.Expr, len(argsNodes))
					for i, a := range argsNodes {
						tmp.Args[i] = a.Expr
					}
					c.markMovesFromCall(chosen, &tmp, args)
					c.enforceCallsiteBorrow(chosen, &tmp)

					ret := chosen.Type.Ret
					if chosen.Decl != nil && chosen.Decl.Async {
						ret = types.FutureOf(ret)
					}
					c.info.Types[call] = ret
					return ret
				case 0:
					c.add(diagAt("DTE0101", fe.Name.Span, "no matching overload"))
					return nil
				default:
					c.add(diagAt("DTE0102", fe.Name.Span, "ambiguous overload"))
					return nil
				}
			}

			// Named-args path (per-candidate mapping)
			var exact []*FuncCand
			for _, cand := range set.Cands {
				vec, ok := c.canonicalizeForCandidate(cand, argsNodes)
				if !ok {
					continue
				}
				if typesMatchExactly(cand.Type.Params, vec) {
					exact = append(exact, cand)
				}
			}
			switch len(exact) {
			case 1:
				chosen := exact[0]
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				// Build positional Args vector aligned to params for borrow/move
				tmp := *call
				vecE := make([]ast.Expr, len(chosen.Type.Params))
				// Map names -> indices
				pnames := c.paramNamesForCand(chosen)
				name2idx := map[string]int{}
				for i, nm := range pnames {
					if nm != "" {
						name2idx[nm] = i
					}
				}
				// Fill leading positionals
				pos := 0
				for _, an := range argsNodes {
					if an.Name == nil {
						if pos < len(vecE) {
							vecE[pos] = an.Expr
							pos++
						}
					}
				}
				// Fill named
				for _, an := range argsNodes {
					if an.Name != nil {
						if idx, ok := name2idx[an.Name.Name]; ok {
							vecE[idx] = an.Expr
						}
					}
				}
				tmp.Args = vecE

				// Enforce borrow (no move tracking here: IR for these is handled via exports)
				c.enforceCallsiteBorrow(chosen, &tmp)
				ret := chosen.Type.Ret
				if chosen.Decl != nil && chosen.Decl.Async {
					ret = types.FutureOf(ret)
				}
				c.info.Types[call] = ret
				return ret
			case 0:
				if !hasCands(set) {
					c.add(diagAt("DME0003", fe.Name.Span, base.Name+" has no exported '"+fe.Name.Name+"'"))
					return nil
				}
				c.add(diagAt("DTE0101", fe.Name.Span, "no matching overload"))
				return nil
			default:
				c.add(diagAt("DTE0102", fe.Name.Span, "ambiguous overload"))
				return nil
			}
		}
		_ = c.typ(fe.X)
		return nil
	}

	// --- Case 2: plain identifier call: f(...)
	if id, ok := call.Callee.(*ast.Ident); ok {
		set := c.info.Funcs[id.Name]
		sym := c.scope.Lookup(id.Name)
		isCallableSym := sym != nil && sym.Kind == SymFunc
		callable := isCallableSym || (set != nil && len(set.Cands) > 0)
		if !callable {
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
				return nil
			}
			c.add(diagAt("DTE0105", id.Span, "value is not callable"))
			return nil
		}

		if !hasNamed {
			// Legacy positional path
			args := make([]types.T, len(argsNodes))
			for i, a := range argsNodes {
				args[i] = c.typ(a.Expr)
			}
			if set == nil || len(set.Cands) == 0 {
				c.add(diagAt("DTE0105", call.Callee.SpanOf(), "value is not callable"))
				return nil
			}
			arityCands := filterByArity(set.Cands, len(args))
			if len(arityCands) == 0 {
				c.add(diagAt("DTE0046", id.Span, "arity mismatch: wrong number of arguments"))
				return nil
			}
			exact := filterExactByTypes(arityCands, args)
			switch len(exact) {
			case 1:
				chosen := exact[0]
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				// Synthesize positional Args for borrow/move enforcement
				tmp := *call
				tmp.Args = make([]ast.Expr, len(argsNodes))
				for i, a := range argsNodes {
					tmp.Args[i] = a.Expr
				}
				c.markMovesFromCall(chosen, &tmp, args)
				c.enforceCallsiteBorrow(chosen, &tmp)

				ret := chosen.Type.Ret
				if chosen.Decl != nil && chosen.Decl.Async {
					ret = types.FutureOf(ret)
				}
				c.info.Types[call] = ret
				return ret
			case 0:
				c.add(diagAt("DTE0101", id.Span, "no matching overload"))
				return nil
			default:
				c.add(diagAt("DTE0102", id.Span, "ambiguous overload"))
				return nil
			}
		}

		// Named-args path
		var exact []*FuncCand
		for _, cand := range set.Cands {
			vec, ok := c.canonicalizeForCandidate(cand, argsNodes)
			if !ok {
				continue
			}
			if typesMatchExactly(cand.Type.Params, vec) {
				exact = append(exact, cand)
			}
		}
		switch len(exact) {
		case 1:
			chosen := exact[0]
			if chosen.Extern && c.unsafeDepth == 0 {
				c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
			}
			// Build positional Args vector aligned to params for borrow/move
			tmp := *call
			vecE := make([]ast.Expr, len(chosen.Type.Params))
			// Map names -> indices
			pnames := c.paramNamesForCand(chosen)
			name2idx := map[string]int{}
			for i, nm := range pnames {
				if nm != "" {
					name2idx[nm] = i
				}
			}
			// Fill leading positionals
			pos := 0
			for _, an := range argsNodes {
				if an.Name == nil {
					if pos < len(vecE) {
						vecE[pos] = an.Expr
						pos++
					}
				}
			}
			// Fill named
			for _, an := range argsNodes {
				if an.Name != nil {
					if idx, ok := name2idx[an.Name.Name]; ok {
						vecE[idx] = an.Expr
					}
				}
			}
			tmp.Args = vecE

			// Types for move tracking (aligned)
			vecT := make([]types.T, len(vecE))
			for i := range vecE {
				vecT[i] = c.typ(vecE[i])
			}

			c.markMovesFromCall(chosen, &tmp, vecT)
			c.enforceCallsiteBorrow(chosen, &tmp)

			ret := chosen.Type.Ret
			if chosen.Decl != nil && chosen.Decl.Async {
				ret = types.FutureOf(ret)
			}
			c.info.Types[call] = ret
			return ret
		case 0:
			c.add(diagAt("DTE0101", id.Span, "no matching overload"))
			return nil
		default:
			c.add(diagAt("DTE0102", id.Span, "ambiguous overload"))
			return nil
		}
	}

	// --- Fallback: callee is some other expression (e.g., (1)()).
	_ = c.typ(call.Callee)
	for _, a := range argsNodes {
		_ = c.typ(a.Expr)
	}
	c.add(diagAt("DTE0105", call.Callee.SpanOf(), "value is not callable"))
	return nil
}

/* ----------------------------- named args core ---------------------------- */

func (c *checker) resolveCallAgainstSet(call *ast.CallExpr, set *OverloadSet, nameSpan, calleeSpan diag.Span) types.T {
	// DCA0003: positional after named (call-site rule, independent of overload).
	seenNamed := false
	for _, a := range call.ArgNodes {
		isNamed := a.Name != nil
		if isNamed {
			seenNamed = true
		} else if seenNamed {
			c.add(diagAt("DCA0003", a.Expr.SpanOf(), "positional argument after named arguments"))
			return nil
		}
	}

	// Try candidates: map names to indices per-candidate, then reuse your existing machinery.
	type mapped struct {
		cand    *FuncCand
		vecExpr []ast.Expr
		vecType []types.T
	}
	var okMapped []mapped
	var firstUnknown *ast.Ident // for DCA0001 if every candidate fails due to unknown names

	for _, cand := range set.Cands {
		pnames := c.paramNamesForCand(cand)
		n := len(cand.Type.Params)
		vecE := make([]ast.Expr, n)
		fill := 0

		// Track duplicates per candidate.
		seen := map[string]bool{}

		// Merge leading unnamed (positional) until we see first named.
		pos := 0
		i := 0
		for i < len(call.ArgNodes) && call.ArgNodes[i].Name == nil {
			if pos >= n {
				// too many args for this candidate — let existing arity filtering handle it later
				break
			}
			vecE[pos] = call.ArgNodes[i].Expr
			fill++
			pos++
			i++
		}
		// Named section
		valid := true
		for ; i < len(call.ArgNodes); i++ {
			an := call.ArgNodes[i]
			if an.Name == nil {
				// shouldn't happen due to DCA0003 earlier, but guard anyway
				valid = false
				break
			}
			name := an.Name.Name
			if seen[name] {
				c.add(diagAt("DCA0002", an.Name.Span, "duplicate named argument: "+name))
				valid = false
				break
			}
			idx := indexOfName(pnames, name)
			if idx < 0 {
				// remember one unknown to possibly report later
				if firstUnknown == nil {
					firstUnknown = an.Name
				}
				valid = false
				break
			}
			if vecE[idx] != nil {
				// same param filled twice via name/position — treat as duplicate
				c.add(diagAt("DCA0002", an.Name.Span, "duplicate named argument: "+name))
				valid = false
				break
			}
			seen[name] = true
			vecE[idx] = an.Expr
			fill++
		}
		if !valid {
			continue
		}
		// Ensure vector is fully populated for this candidate.
		if fill != n {
			// let existing arity/type machinery handle this; skip for now
			continue
		}
		// Compute types vector
		vecT := make([]types.T, n)
		for j := 0; j < n; j++ {
			vecT[j] = c.typ(vecE[j])
		}
		okMapped = append(okMapped, mapped{cand: cand, vecExpr: vecE, vecType: vecT})
	}

	// If no candidate mapped, decide the best diagnostic.
	if len(okMapped) == 0 {
		if firstUnknown != nil {
			c.add(diagAt("DCA0001", firstUnknown.Span, "unknown named argument: "+firstUnknown.Name))
			return nil
		}
		// Otherwise fall back to a generic arity/overload error (reuse legacy paths).
		// Build plain arg types to drive legacy filters.
		rawArgs := make([]types.T, len(call.ArgNodes))
		for i, a := range call.ArgNodes {
			rawArgs[i] = c.typ(a.Expr)
		}
		arityCands := filterByArity(set.Cands, len(rawArgs))
		if len(arityCands) == 0 {
			c.add(diagAt("DTE0046", nameSpan, "arity mismatch: wrong number of arguments"))
			return nil
		}
		exact := filterExactByTypes(arityCands, rawArgs)
		if len(exact) == 0 {
			c.add(diagAt("DTE0101", nameSpan, "no matching overload"))
			return nil
		}
		c.add(diagAt("DTE0102", nameSpan, "ambiguous overload"))
		return nil
	}

	// Now run exact-type matching across mapped candidates.
	best := make([]*FuncCand, 0, len(okMapped))
	idxOf := func(c *FuncCand) int {
		for i, m := range okMapped {
			if m.cand == c {
				return i
			}
		}
		return -1
	}
	for _, m := range okMapped {
		if typesMatchExactly(m.cand.Type.Params, m.vecType) {
			best = append(best, m.cand)
		}
	}
	switch len(best) {
	case 0:
		c.add(diagAt("DTE0101", nameSpan, "no matching overload"))
		return nil
	case 1:
		chosen := best[0]
		mi := idxOf(chosen)
		vecE := okMapped[mi].vecExpr
		vecT := okMapped[mi].vecType

		// Synthetic call to reuse borrow/move logic that expects CallExpr.Args
		tmp := *call
		tmp.Args = make([]ast.Expr, len(vecE))
		copy(tmp.Args, vecE)

		// unsafe gate for extern
		if chosen.Extern && c.unsafeDepth == 0 {
			c.add(diagAt("DFI0003", calleeSpan, ""))
		}
		// moves + borrow enforcement use the positional vector
		c.markMovesFromCall(chosen, &tmp, vecT)
		c.enforceCallsiteBorrow(chosen, &tmp)

		ret := chosen.Type.Ret
		if chosen.Decl != nil && chosen.Decl.Async {
			ret = types.FutureOf(ret)
		}
		c.info.Types[call] = ret
		return ret
	default:
		c.add(diagAt("DTE0102", nameSpan, "ambiguous overload"))
		return nil
	}
}

func (c *checker) paramNamesForCand(cand *FuncCand) []string {
	// Prefer source names from local decl; else use recorded ParamNames (imports/builtins)
	if cand.Decl != nil {
		out := make([]string, len(cand.Decl.Params))
		for i := range cand.Decl.Params {
			out[i] = cand.Decl.Params[i].Name.Name
		}
		return out
	}
	if len(cand.ParamNames) > 0 {
		cp := make([]string, len(cand.ParamNames))
		copy(cp, cand.ParamNames)
		return cp
	}
	// no names available; fall back to empty strings
	return make([]string, len(cand.Type.Params))
}

func indexOfName(names []string, want string) int {
	for i, n := range names {
		if n == want {
			return i
		}
	}
	return -1
}

func typesMatchExactly(params []types.T, args []types.T) bool {
	if len(params) != len(args) {
		return false
	}
	for i := range params {
		if !types.Equal(params[i], args[i]) {
			return false
		}
	}
	return true
}

// --- Borrow callsite enforcement (inout/ref lvalue + aliasing with secondary label) ---
func (c *checker) enforceCallsiteBorrow(chosen *FuncCand, call *ast.CallExpr) {
	if chosen == nil || call == nil {
		return
	}

	// Gather effective modes for the chosen overload.
	// Prefer local Decl param modes; fall back to chosen.Modes for cross-module exports.
	var modes []ast.ParamMode
	if chosen.Decl != nil {
		fd := chosen.Decl
		n := min(len(fd.Params), len(call.Args))
		modes = make([]ast.ParamMode, n)
		for i := 0; i < n; i++ {
			modes[i] = fd.Params[i].Mode
		}
	} else if len(chosen.Modes) > 0 {
		n := min(len(chosen.Modes), len(call.Args))
		modes = make([]ast.ParamMode, n)
		copy(modes, chosen.Modes[:n])
	} else {
		// No mode info available; nothing to enforce.
		return
	}

	// 1) enforce lvalue requirements and collect bases/spans for aliasing
	bases := make([]string, len(modes))
	argSpans := make([]diag.Span, len(modes))
	for i, mode := range modes {
		name, isLval := c.baseLvalue(call.Args[i])

		switch mode {
		case ast.ParamInout:
			if !isLval {
				c.add(diagAt("DBR0002", call.Args[i].SpanOf(), "inout argument must be a mutable lvalue"))
				continue
			}
			bases[i] = name
			argSpans[i] = call.Args[i].SpanOf()

		case ast.ParamRef:
			// Task 2 rule: ref requires an lvalue.
			if !isLval {
				c.add(diagAt("DBR0005", call.Args[i].SpanOf(), "ref argument must be an lvalue"))
				continue
			}
			bases[i] = name
			argSpans[i] = call.Args[i].SpanOf()

		default:
			// move param: lvalue-ness not required; we don't need its base for aliasing checks
		}
	}

	// 2) aliasing: same base used twice where at least one is inout -> DBR0003
	for i := 0; i < len(modes); i++ {
		if bases[i] == "" {
			continue
		}
		for j := i + 1; j < len(modes); j++ {
			if bases[j] == "" {
				continue
			}
			if bases[i] == bases[j] && (modes[i] == ast.ParamInout || modes[j] == ast.ParamInout) {
				// Keep message stable so tests that look for "alias" continue to pass.
				msg := "inout cannot alias with another argument in the same call"
				d := diagAt("DBR0003", call.Args[j].SpanOf(), msg)
				// NEW: add a secondary label on the earlier conflicting arg.
				sec := d.Primary // reuse the same type as a template (no extra imports)
				sec.Span = argSpans[i]
				sec.Text = "aliases with this argument"
				sec.Primary = false
				d.Labels = append(d.Labels, sec)
				c.add(d)
			}
		}
	}
}

// filterByArity returns candidates whose arity is compatible with n, taking
// parameter defaults into account (M14).
func filterByArity(cands []*FuncCand, n int) []*FuncCand {
	out := make([]*FuncCand, 0, len(cands))
	for _, cand := range cands {
		if cand == nil || cand.Type == nil {
			continue
		}
		total := len(cand.Type.Params)
		defaults := candDefaults(cand)

		required := total
		if len(defaults) == total && total > 0 {
			required = 0
			for i := 0; i < total; i++ {
				if !defaults[i] {
					required++
				}
			}
		}

		if required <= n && n <= total {
			out = append(out, cand)
		}
	}
	return out
}

// filterExactByTypes returns candidates whose parameter types exactly match
// the provided argument types, allowing trailing parameters to be satisfied
// by defaults (M14).
func filterExactByTypes(cands []*FuncCand, args []types.T) []*FuncCand {
	out := make([]*FuncCand, 0, len(cands))
	for _, cand := range cands {
		if cand == nil || cand.Type == nil {
			continue
		}
		params := cand.Type.Params
		if len(args) > len(params) {
			// more args than parameters: cannot match
			continue
		}
		defaults := candDefaults(cand)

		ok := true
		// Check the prefix that has explicit arguments.
		for i := range args {
			if !types.Equal(params[i], args[i]) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		// Any remaining parameters must be satisfied by defaults.
		for i := len(args); i < len(params); i++ {
			if len(defaults) != len(params) || !defaults[i] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		out = append(out, cand)
	}
	return out
}

// callArgs returns the canonical call arguments: if ArgNodes exist (E-2),
// return them; otherwise synthesize from legacy positional Args.
func callArgs(call *ast.CallExpr) []ast.CallArg {
	if call == nil {
		return nil
	}
	if len(call.ArgNodes) > 0 {
		return call.ArgNodes
	}
	if len(call.Args) == 0 {
		return nil
	}
	out := make([]ast.CallArg, len(call.Args))
	for i, e := range call.Args {
		out[i] = ast.CallArg{Expr: e}
	}
	return out
}

func candParamNames(cand *FuncCand) []string {
	if cand == nil || cand.Type == nil {
		return nil
	}
	// Source-of-truth priority:
	// 1) Local decl names (always present for Decl != nil)
	if cand.Decl != nil {
		out := make([]string, len(cand.Decl.Params))
		for i := range cand.Decl.Params {
			out[i] = cand.Decl.Params[i].Name.Name
		}
		return out
	}
	// 2) Imported/exported names from resolver payload
	if len(cand.ParamNames) == len(cand.Type.Params) && len(cand.ParamNames) > 0 {
		return cand.ParamNames
	}
	// 3) Unknown (nil) — means named arguments cannot be mapped for this cand
	return nil
}

// candDefaults returns a bool slice marking which params have defaults.
// For local decls we read directly from the AST; for imported/builtins we use
// FuncCand.Defaults (if present).
func candDefaults(cand *FuncCand) []bool {
	if cand == nil || cand.Type == nil {
		return nil
	}
	if cand.Decl != nil {
		out := make([]bool, len(cand.Decl.Params))
		for i := range cand.Decl.Params {
			out[i] = cand.Decl.Params[i].Default != nil
		}
		return out
	}
	if len(cand.Defaults) == len(cand.Type.Params) && len(cand.Defaults) > 0 {
		return cand.Defaults
	}
	return nil
}

// checkNoPosAfterNamed enforces: after first named arg, no positional args.
// Returns true if OK; false if a diagnostic was emitted.
func (c *checker) checkNoPosAfterNamed(args []ast.CallArg) bool {
	seenNamed := false
	for _, a := range args {
		if a.Name != nil {
			seenNamed = true
			continue
		}
		if seenNamed {
			// First offending positional after a named one.
			c.add(diagAt("DCA0003", a.Expr.SpanOf(), "positional argument after named arguments"))
			return false
		}
	}
	return true
}

// canonicalizeForCandidate maps 'args' into a positional vector aligned to cand.Type.Params.
// It emits DCA0001 (unknown name) / DCA0002 (duplicate) / DCA0003 (positional-after-named) as needed.
// On success, returns a slice of types for each param index, using param types for
// omitted-but-defaulted params.
func (c *checker) canonicalizeForCandidate(cand *FuncCand, args []ast.CallArg) ([]types.T, bool) {
	if cand == nil || cand.Type == nil {
		return nil, false
	}
	// Global rule first: no positional after named.
	if !c.checkNoPosAfterNamed(args) {
		return nil, false
	}

	n := len(cand.Type.Params)
	out := make([]types.T, n)
	filled := make([]bool, n)

	// Param name lookup (maybe nil => cannot map names)
	pnames := candParamNames(cand)
	nameToIdx := map[string]int{}
	if len(pnames) == n {
		for i, nm := range pnames {
			if nm != "" {
				nameToIdx[nm] = i
			}
		}
	}

	// (1) Fill leading positionals
	next := 0
	for _, a := range args {
		if a.Name != nil {
			continue
		}
		if next >= n {
			// Too many args (will just fail to match this cand silently)
			return nil, false
		}
		out[next] = c.typ(a.Expr)
		filled[next] = true
		next++
	}

	// (2) Fill named
	seenName := map[string]bool{}
	for _, a := range args {
		if a.Name == nil {
			continue
		}
		key := a.Name.Name
		if seenName[key] {
			c.add(diagAt("DCA0002", a.Name.Span, "duplicate named argument: "+key))
			return nil, false
		}
		seenName[key] = true

		idx, ok := nameToIdx[key]
		if !ok {
			c.add(diagAt("DCA0001", a.Name.Span, "unknown named argument: "+key))
			return nil, false
		}
		if filled[idx] {
			// Another duplicate route (positional already filled same param)
			c.add(diagAt("DCA0002", a.Name.Span, "duplicate named argument: "+key))
			return nil, false
		}
		out[idx] = c.typ(a.Expr)
		filled[idx] = true
	}

	// (3) Fill omitted parameters using defaults; all non-defaulted params must be provided.
	defaults := candDefaults(cand)
	for i := 0; i < n; i++ {
		if filled[i] {
			continue
		}
		// If this param has a default, treat it as supplied with its declared type.
		if len(defaults) == n && defaults[i] {
			out[i] = cand.Type.Params[i]
			filled[i] = true
			continue
		}
		// No arg and no default: this candidate is not a match.
		return nil, false
	}
	return out, true
}
