package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
)

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
		if typesMatchExactly(m.cand.Type.Params, m.vecType, m.cand.Type.Variadic) {
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

func typesMatchExactly(params []types.T, args []types.T, isVariadic bool) bool {
	if isVariadic {
		// Variadic: args must be at least len(params)-1
		if len(args) < len(params)-1 {
			return false
		}
		// Check prefix
		for i := 0; i < len(params)-1; i++ {
			if !types.Assignable(params[i], args[i]) {
				return false
			}
		}
		// Check variadic tail
		lastParam := params[len(params)-1]
		var elemType types.T
		if lst, ok := lastParam.(*types.List); ok {
			elemType = lst.Elem
		} else {
			return false
		}
		for i := len(params) - 1; i < len(args); i++ {
			if !types.Assignable(elemType, args[i]) {
				return false
			}
		}
		return true
	}

	if len(params) != len(args) {
		return false
	}
	for i := range params {
		if !types.Assignable(params[i], args[i]) {
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
	isVariadic := chosen.Type.Variadic

	if chosen.Decl != nil {
		fd := chosen.Decl
		nParams := len(fd.Params)
		nArgs := len(call.Args)
		n := nArgs
		if !isVariadic && nParams < n {
			n = nParams
		}

		modes = make([]ast.ParamMode, n)
		for i := 0; i < n; i++ {
			if i < nParams {
				modes[i] = fd.Params[i].Mode
			} else if isVariadic {
				modes[i] = fd.Params[nParams-1].Mode
			}
		}
	} else if len(chosen.Modes) > 0 {
		nParams := len(chosen.Modes)
		nArgs := len(call.Args)
		n := nArgs
		if !isVariadic && nParams < n {
			n = nParams
		}

		modes = make([]ast.ParamMode, n)
		for i := 0; i < n; i++ {
			if i < nParams {
				modes[i] = chosen.Modes[i]
			} else if isVariadic {
				modes[i] = chosen.Modes[nParams-1]
			}
		}
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

		if cand.Type.Variadic {
			// Variadic: last param is *args (list[T]), always optional.
			// Required count depends on preceding params.
			required := 0
			for i := 0; i < total-1; i++ {
				if !defaults[i] {
					required++
				}
			}
			if n >= required {
				out = append(out, cand)
			}
		} else {
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
		defaults := candDefaults(cand)
		isVariadic := cand.Type.Variadic

		if !isVariadic {
			if len(args) > len(params) {
				continue
			}
		}

		ok := true
		// Check explicit arguments against parameters
		for i := range args {
			var paramType types.T
			if isVariadic && i >= len(params)-1 {
				// Variadic argument: match against element type of the last param (list[T])
				lastParam := params[len(params)-1]
				if lst, ok := lastParam.(*types.List); ok {
					paramType = lst.Elem
				} else {
					ok = false
					break
				}
			} else {
				// Normal argument
				if i >= len(params) {
					ok = false
					break
				}
				paramType = params[i]
			}

			if !types.Assignable(paramType, args[i]) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}

		// Ensure unsatisfied parameters have defaults
		// For variadic, we only care about non-variadic params (prefix)
		checkLimit := len(params)
		if isVariadic {
			checkLimit--
		}

		for i := len(args); i < checkLimit; i++ {
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
// FuncCand.Defaults (if present). If no metadata is available, we return an
// all-false slice sized to the arity so arity logic can still run.
func candDefaults(cand *FuncCand) []bool {
	if cand == nil || cand.Type == nil {
		return nil
	}

	// Local declaration: source of truth is the AST params.
	if cand.Decl != nil {
		out := make([]bool, len(cand.Decl.Params))
		for i := range cand.Decl.Params {
			out[i] = cand.Decl.Params[i].Default != nil
		}
		return out
	}

	// Imported/builtin: use stored Defaults if it’s arity-aligned.
	n := len(cand.Type.Params)
	if n == 0 {
		return nil
	}
	if len(cand.Defaults) == n {
		out := make([]bool, n)
		copy(out, cand.Defaults)
		return out
	}

	// No metadata: treat as "no defaults" but still expose arity.
	return make([]bool, n)
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
	isVariadic := cand.Type.Variadic
	for _, a := range args {
		if a.Name != nil {
			continue
		}
		if next >= n {
			if isVariadic {
				out = append(out, c.typ(a.Expr))
				continue
			}
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
			// Unknown named arg for this candidate - don't emit error yet,
			// another candidate may have this param. Just fail this candidate.
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
