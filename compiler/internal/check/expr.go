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
		// pipeline: lhs |> f(...)  ==>  f(lhs, ...)
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

		set := c.info.Funcs[id.Name]
		if set == nil || len(set.Cands) == 0 {
			c.add(diagAt("DTE0001", id.Span, "pipeline target undefined function: "+id.Name))
			return nil
		}

		// Build a synthetic call arg list: lhs as the first positional + original call.ArgNodes
		synth := &ast.CallExpr{
			Callee: call.Callee,
			Span:   call.Span,
			Args:   nil, // legacy field unused for mapping
		}
		// copy original ArgNodes and prepend one positional for lhs
		synth.ArgNodes = make([]ast.CallArg, 0, 1+len(call.ArgNodes))
		synth.ArgNodes = append(synth.ArgNodes, ast.CallArg{Name: nil, Expr: x.Lhs})
		synth.ArgNodes = append(synth.ArgNodes, call.ArgNodes...)

		return c.resolveCallAgainstSet(synth, set, id.Span, call.Callee.SpanOf())

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

// typCall performs overload resolution for calls with named-arg canonicalization.
func (c *checker) typCall(call *ast.CallExpr) types.T {
	// module-qualified call: mod.fn(...)
	if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		if set, base, isImport := c.moduleQualifiedOverloadSet(fe); isImport {
			return c.resolveCallAgainstSet(call, set, fe.Name.Span, base.Span)
		}
		_ = c.typ(fe.X)
		return nil
	}

	// plain identifier call: f(...)
	if id, ok := call.Callee.(*ast.Ident); ok {
		set, hasSet := c.info.Funcs[id.Name]
		sym := c.scope.Lookup(id.Name)
		isCallableSym := sym != nil && sym.Kind == SymFunc
		callable := isCallableSym || (hasSet && set != nil)
		if !callable {
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
				return nil
			}
			c.add(diagAt("DTE0105", id.Span, "value is not callable"))
			return nil
		}
		if set == nil || len(set.Cands) == 0 {
			_ = c.typ(call.Callee)
			for _, a := range call.ArgNodes {
				_ = c.typ(a.Expr)
			}
			c.add(diagAt("DTE0105", call.Callee.SpanOf(), "value is not callable"))
			return nil
		}
		return c.resolveCallAgainstSet(call, set, id.Span, call.Callee.SpanOf())
	}

	// fallback
	_ = c.typ(call.Callee)
	for _, a := range call.ArgNodes {
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
