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

		// Existing ergonomics: string + stringy => string
		if op == "+" && types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
			c.info.Types[x] = types.Str
			return types.Str
		}
		if op == "+" && (types.Equal(lt, types.Str) || types.Equal(rt, types.Str)) {
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
		args := make([]types.T, 0, 1+len(call.Args))
		args = append(args, lhsT)
		for _, a := range call.Args {
			args = append(args, c.typ(a))
		}

		set := c.info.Funcs[id.Name]
		if set == nil || len(set.Cands) == 0 {
			c.add(diagAt("DTE0001", id.Span, "pipeline target undefined function: "+id.Name))
			return nil
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
			c.markMovesFromCall(chosen, call, args)
			c.enforceCallsiteBorrow(chosen, call)
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

	case "<", "<=", ">", ">=", "==", "!=":
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

	case "and", "or":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		if types.Equal(lt, types.Bool) && types.Equal(rt, types.Bool) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "logical operators require bool operands"))
		return nil

	default:
		return nil
	}
}

// typCall performs overload resolution for calls and wires move tracking so
// later identifier reads can trigger DBR0004 via typIdent.
func (c *checker) typCall(call *ast.CallExpr) types.T {
	// --- Case 0: direct call of a lambda:  (lambda ...)(args)
	if l, ok := call.Callee.(*ast.LambdaExpr); ok {
		// Type the lambda first (enforces "typed params in M4" and sets its func type).
		_ = c.typ(l)

		// Arity check
		if len(call.Args) != len(l.Params) {
			c.add(diagAt("DTE0046", l.Span, "arity mismatch: wrong number of arguments"))
			return nil
		}

		// Per-arg type check against lambda's declared param types
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
		// Lambda call result is the lambda's body type (already computed)
		if ft, ok := c.info.Types[l].(*types.Func); ok {
			c.info.Types[call] = ft.Ret
			return ft.Ret
		}
		return nil
	}

	// --- Case 1: module-qualified call  e.g.  mod.fn(...)
	if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		if set, base, isImport := c.moduleQualifiedOverloadSet(fe); isImport {
			// Arg types
			args := make([]types.T, len(call.Args))
			for i, a := range call.Args {
				args[i] = c.typ(a)
			}
			// No exported candidates
			if set == nil || len(set.Cands) == 0 {
				c.add(diagAt("DME0003", fe.Name.Span, base.Name+" has no exported '"+fe.Name.Name+"'"))
				return nil
			}
			// Filter by arity then exact types
			arityCands := filterByArity(set.Cands, len(args))
			if len(arityCands) == 0 {
				c.add(diagAt("DTE0045", fe.Name.Span, "arity mismatch: wrong number of arguments"))
				return nil
			}
			exact := filterExactByTypes(arityCands, args)
			switch len(exact) {
			case 1:
				chosen := exact[0]
				// Require unsafe for extern FFI calls.
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				// Mark moves for local decls (Decl!=nil). Cross-module exports have Decl==nil.
				c.markMovesFromCall(chosen, call, args)
				// Enforce caller-side borrow rules.
				c.enforceCallsiteBorrow(chosen, call)

				ret := chosen.Type.Ret
				// If callee is async, calls return a future[T].
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
		// If it wasn't an import-qualified callee, fall through and type the pieces.
		_ = c.typ(fe.X)
		// NOTE: fe.Name is an ast.Ident value; don't call c.typ on it.
		return nil
	}

	// --- Case 2: plain identifier call  e.g.  f(...)
	if id, ok := call.Callee.(*ast.Ident); ok {
		set, hasSet := c.info.Funcs[id.Name]

		// Is there a function symbol in scope?
		sym := c.scope.Lookup(id.Name)
		isCallableSym := sym != nil && sym.Kind == SymFunc

		// Callable if it's a real function symbol OR we have an overload set
		callable := isCallableSym || (hasSet && set != nil)
		if !callable {
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
				return nil
			}
			c.add(diagAt("DTE0105", id.Span, "value is not callable"))
			return nil
		}

		// Arg types
		args := make([]types.T, len(call.Args))
		for i, a := range call.Args {
			args[i] = c.typ(a)
		}

		// If we have no overload set at all, bail as not callable
		if set == nil || len(set.Cands) == 0 {
			_ = c.typ(call.Callee)
			for _, a := range call.Args {
				_ = c.typ(a)
			}
			c.add(diagAt("DTE0105", call.Callee.SpanOf(), "value is not callable"))
			return nil
		}

		// Filter by arity then exact types
		arityCands := filterByArity(set.Cands, len(args))
		if len(arityCands) == 0 {
			c.add(diagAt("DTE0045", id.Span, "arity mismatch: wrong number of arguments"))
			return nil
		}
		exact := filterExactByTypes(arityCands, args)
		switch len(exact) {
		case 1:
			chosen := exact[0]
			// Require unsafe for extern FFI calls.
			if chosen.Extern && c.unsafeDepth == 0 {
				c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
			}
			// Mark moves so later ident reads can trigger DBR0004.
			c.markMovesFromCall(chosen, call, args)
			// Enforce caller-side borrow rules (inout lvalue + aliasing).
			c.enforceCallsiteBorrow(chosen, call)

			ret := chosen.Type.Ret
			// If callee is async, calls return a future[T].
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
	for _, a := range call.Args {
		_ = c.typ(a)
	}
	c.add(diagAt("DTE0105", call.Callee.SpanOf(), "value is not callable"))
	return nil
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

// filterByArity returns candidates whose arity equals n.
func filterByArity(cands []*FuncCand, n int) []*FuncCand {
	out := make([]*FuncCand, 0, len(cands))
	for _, cand := range cands {
		if len(cand.Type.Params) == n {
			out = append(out, cand)
		}
	}
	return out
}

// filterExactByTypes returns candidates whose parameter types exactly match args.
func filterExactByTypes(cands []*FuncCand, args []types.T) []*FuncCand {
	out := make([]*FuncCand, 0, len(cands))
	for _, cand := range cands {
		if len(cand.Type.Params) != len(args) {
			continue
		}
		ok := true
		for i := range args {
			if !types.Equal(args[i], cand.Type.Params[i]) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, cand)
		}
	}
	return out
}
