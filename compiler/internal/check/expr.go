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

	// helpers intInfo/floatInfo elided for brevity — keep your existing body

	switch op {
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

		// Build synthetic arg list: [lhs] + existing call args (respect ArgNodes if present)
		base := callArgs(call)
		synth := make([]ast.CallArg, 0, 1+len(base))
		synth = append(synth, ast.CallArg{Expr: &ast.Ident{Name: "<pipe>", Span: x.Lhs.SpanOf()}}) // placeholder expr; we already have lhsT
		synth = append(synth, base...)

		// Evaluate candidates: force first param to match lhsT, then named mapping for the rest.
		var exact []*FuncCand
		for _, cand := range set.Cands {
			// Arity must be at least 1
			if cand.Type == nil || len(cand.Type.Params) == 0 {
				continue
			}
			vec, ok := c.canonicalizeForCandidate(cand, synth)
			if !ok {
				continue
			}
			// Overwrite first slot with lhsT since placeholder expr has no real type.
			vec[0] = lhsT
			if filterExactByTypes([]*FuncCand{cand}, vec); len(filterExactByTypes([]*FuncCand{cand}, vec)) == 1 {
				exact = append(exact, cand)
			}
		}
		switch len(exact) {
		case 1:
			chosen := exact[0]
			if chosen.Extern && c.unsafeDepth == 0 {
				c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
			}
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

		// ... keep the rest of your existing cases unchanged ...
	}

	// keep the remainder of your original function body intact
	return nil
}

// typCall performs overload resolution for calls (named/positional) and wires move tracking.
func (c *checker) typCall(call *ast.CallExpr) types.T {
	// --- Case 0: direct call of a lambda:  (lambda ...)(args)  (unchanged)
	if l, ok := call.Callee.(*ast.LambdaExpr); ok {
		_ = c.typ(l)
		// Use legacy positional flow for lambdas; named calls to lambdas are not supported in M13.
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

	// Precompute canonical call args (prefers ArgNodes if present).
	args := callArgs(call)

	// --- Case 1: module-qualified call  e.g.  mod.fn(...)
	if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		if set, base, isImport := c.moduleQualifiedOverloadSet(fe); isImport {
			// Build matches per-candidate using canonicalization (named mapping per overload).
			var exact []*FuncCand
			for _, cand := range set.Cands {
				vec, ok := c.canonicalizeForCandidate(cand, args)
				if !ok {
					continue // not a match for this candidate
				}
				// Exact type match?
				if filterExactByTypes([]*FuncCand{cand}, vec); len(filterExactByTypes([]*FuncCand{cand}, vec)) == 1 {
					exact = append(exact, cand)
				}
			}
			switch len(exact) {
			case 1:
				chosen := exact[0]
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				// Borrow/move enforcement keeps legacy positional order (OK for existing tests).
				c.enforceCallsiteBorrow(chosen, call)
				ret := chosen.Type.Ret
				if chosen.Decl != nil && chosen.Decl.Async {
					ret = types.FutureOf(ret)
				}
				c.info.Types[call] = ret
				return ret
			case 0:
				// If no exact match but we have an overload set, keep legacy messages.
				if set == nil || len(set.Cands) == 0 {
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

	// --- Case 2: plain identifier call  e.g.  f(...)
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

		// Evaluate candidates with named mapping.
		var exact []*FuncCand
		for _, cand := range set.Cands {
			vec, ok := c.canonicalizeForCandidate(cand, args)
			if !ok {
				continue
			}
			if filterExactByTypes([]*FuncCand{cand}, vec); len(filterExactByTypes([]*FuncCand{cand}, vec)) == 1 {
				exact = append(exact, cand)
			}
		}
		switch len(exact) {
		case 1:
			chosen := exact[0]
			if chosen.Extern && c.unsafeDepth == 0 {
				c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
			}
			c.enforceCallsiteBorrow(chosen, call)
			ret := chosen.Type.Ret
			if chosen.Decl != nil && chosen.Decl.Async {
				ret = types.FutureOf(ret)
			}
			c.info.Types[call] = ret
			return ret
		case 0:
			// Keep legacy arity mismatch if nothing matches by arity exactly
			// (we don't have defaults yet).
			c.add(diagAt("DTE0101", id.Span, "no matching overload"))
			return nil
		default:
			c.add(diagAt("DTE0102", id.Span, "ambiguous overload"))
			return nil
		}
	}

	// --- Fallback: callee is some other expression (e.g., (1)()).
	_ = c.typ(call.Callee)
	for _, a := range args {
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
// On success, returns a slice of types for each param index.
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

	// Param name lookup (may be nil => cannot map names)
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

	// (3) All params must be provided (no defaults yet)
	for i := 0; i < n; i++ {
		if !filled[i] {
			// Let normal arity/type filtering handle this candidate as non-match.
			return nil, false
		}
	}
	return out, true
}
