package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

// ---------- public API ----------

// Result is a structured return used by the CLI.
type Result struct {
	Diags []diag.Diagnostic
	Info  *Info
}

// Check keeps the legacy/public surface that tests expect: (diags, info).
func Check(mod *ast.Module) ([]diag.Diagnostic, *Info) {
	res := CheckWithLoader(mod, resolve.NewMemLoader(nil))
	return res.Diags, res.Info
}

// CheckWithLoader resolves + checks using the provided loader (used by the CLI).
func CheckWithLoader(mod *ast.Module, ldr resolve.Loader) *Result {
	res := &Result{Info: NewInfo()}

	// Set up unused-import tracker (bindings from AST); usage is counted post-check.
	ut := newUsageTracker(mod)

	// 1) Resolve imports up-front (Phase-2 brings typed exports in rinfo).
	rdiags, rinfo := resolve.Resolve(mod, ldr)
	res.Diags = append(res.Diags, rdiags...)

	// Check for circular imports
	if rinfo != nil && rinfo.Graph != nil {
		for _, cycle := range rinfo.Graph.Cycles() {
			msg := "circular import detected: " + strings.Join(cycle, " → ")
			res.Diags = append(res.Diags, diag.Diagnostic{
				CodeID:  "DME0008",
				Domain:  "module",
				Message: msg,
			})
		}
	}

	// Bridge resolver info and pre-computed import-name -> module-path map for qualified calls.
	res.Info.R = rinfo
	res.Info.ImportPaths = computeImportPaths(mod)

	// 2) Create the top scope and inject bindings.
	top := NewScope(nil)
	// Make prelude callables visible as identifiers in the top scope (avoid DTE0001).
	injectPreludeIntoScope(top, res.Info)
	// Resolver-provided imports (modules and from-items) become top-scope names.
	injectImports(top, rinfo)
	// Top-level let statements become module-scope globals.
	res.Diags = append(res.Diags, injectGlobals(top, mod)...)

	desugarMapFilter(mod)

	// Phase-2: build exact signatures for 'from … import …' into Info.Funcs.
	PopulateImportedFuncSigs(mod, res.Info, rinfo)

	// 3) Walk module: collect functions first, then check bodies.
	c := &checker{
		info:  res.Info,
		scope: NewScope(top), // child of top so imported names & prelude are visible
	}
	// Remember the module (file-level) scope for anti-shadowing checks.
	c.moduleScope = c.scope

	// Pass 1: collect functions for overload sets.
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			c.collectFunc(dd)
		case *ast.StructDecl:
			c.collectStruct(dd)
		case *ast.EnumDecl:
			c.collectEnum(dd)
		case *ast.ClassDecl:
			c.collectClass(dd)
		case *ast.TraitDecl:
			c.collectTrait(dd)
		case *ast.ImplDecl:
			c.collectImpl(dd)
		case *ast.TypeAliasDecl:
			c.collectTypeAlias(dd)
		}
	}

	// M14: enforce default-parameter consistency across overloads.
	res.Diags = append(res.Diags, enforceDefaultConsistency(res.Info)...)

	// Pass 2: check bodies.
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.StructDecl:
			c.checkStruct(dd)
		case *ast.EnumDecl:
			c.checkEnum(dd)
		case *ast.FuncDecl:
			c.checkFunc(dd)
		case *ast.ClassDecl:
			c.checkClass(dd)
		case *ast.TraitDecl:
			c.checkTraitBody(dd)
		case *ast.ImplDecl:
			c.checkImplBody(dd)
		}
	}

	// Merge checker diagnostics.
	res.Diags = append(res.Diags, c.diags...)

	// ---- Task B hook: len() nicer error when type has no length (DCO0001) ----
	res.Diags = append(res.Diags, collectLenNoLengthDiags(mod, res.Info)...)

	// ---- Task C hook: membership 'in' (string-only for now) -------------------
	res.Diags = append(res.Diags, collectMembershipDiags(mod, res.Info)...)
	// --------------------------------------------------------------------------

	// 4) After we know which identifiers resolved to which symbols,
	//    compute unused-import warnings and append them.
	ut.countUsesFromIdents(res.Info.Idents)
	ut.countUsesFromTypes(res.Info)
	res.Diags = append(res.Diags, ut.emitUnusedDiags()...)

	return res
}

// enforceDefaultConsistency ensures that all overloads for a given name share
// the same "which parameters have defaults" mask. It only reports on local
// declarations (Decl != nil); imported/builtin candidates participate in the
// comparison but don't get their own spans.
func enforceDefaultConsistency(info *Info) []diag.Diagnostic {
	var out []diag.Diagnostic
	if info == nil {
		return out
	}

	for name, set := range info.Funcs {
		if set == nil || len(set.Cands) <= 1 {
			continue
		}

		var baseline []bool

		for _, cand := range set.Cands {
			if cand == nil || cand.Type == nil {
				continue
			}

			mask := candDefaults(cand)
			if len(mask) == 0 {
				continue
			}

			if baseline == nil {
				baseline = make([]bool, len(mask))
				copy(baseline, mask)
				continue
			}

			if len(mask) != len(baseline) {
				if cand.Decl != nil {
					out = append(out, diagAt("DDF0005", cand.Decl.Name.Span,
						"default parameters must be consistent across overloads of "+name))
				}
				continue
			}

			mismatchIdx := -1
			for i := range baseline {
				if mask[i] != baseline[i] {
					mismatchIdx = i
					break
				}
			}
			if mismatchIdx >= 0 && cand.Decl != nil {
				// Point at the parameter whose default status disagrees.
				if mismatchIdx < len(cand.Decl.Params) {
					out = append(out, diagAt("DDF0005", cand.Decl.Params[mismatchIdx].Name.Span,
						"default parameters must be consistent across overloads of "+name))
				} else {
					out = append(out, diagAt("DDF0005", cand.Decl.Name.Span,
						"default parameters must be consistent across overloads of "+name))
				}
			}
		}
	}
	return out
}

type checker struct {
	info        *Info
	diags       []diag.Diagnostic
	scope       *Scope
	moduleScope *Scope
	curFuncRet  types.T
	curFuncName string // Name of current function (for __new__ detection)
	moved       MoveSet
	unsafeDepth int
	expected    types.T // Expected type from context (for bidirectional checking)
}

func (c *checker) add(diag diag.Diagnostic) { c.diags = append(c.diags, diag) }

func (c *checker) collectFunc(fd *ast.FuncDecl) {
	name := fd.Name.Name

	// Top-level shadowing of builtin names is not allowed.
	c.forbidBuiltinShadowing(name, fd.Name.Span)

	// Create a temporary scope to resolve parameters using generic types
	saved := c.scope
	c.scope = NewScope(c.scope)
	defer func() { c.scope = saved }()

	// Add generic type parameters to scope
	for _, typeParam := range fd.TypeParams {
		c.scope.Define(&Symbol{
			Name: typeParam.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: typeParam.Name},
		})
	}

	// Build function type from parameter annotations (surface forms allowed).
	params := make([]types.T, len(fd.Params))
	variadic := false
	for i, p := range fd.Params {
		if p.Type != nil {
			if tt := c.resolveType(p.Type); tt != nil {
				// If this is a variadic parameter, wrap in list[T]
				if p.Variadic {
					params[i] = types.ListOf(tt)
				} else {
					params[i] = tt
				}
			}
		}
		if p.Variadic {
			variadic = true
		}
	}
	var ret types.T = types.None
	if fd.RetType != nil {
		if tt := c.resolveType(fd.RetType); tt != nil {
			ret = tt
		}
	}
	sig := types.FuncOf(params, ret, variadic)
	sig.Name = name
	sig.IsPub = fd.Pub
	for _, tp := range fd.TypeParams {
		sig.TypeParams = append(sig.TypeParams, types.TypeParam{Name: tp.Name})
	}

	set := c.info.Funcs[name]
	if set == nil {
		set = &OverloadSet{Name: name}
		c.info.Funcs[name] = set
	}
	// Collect param modes + default mask from the declaration.
	modes := make([]ast.ParamMode, len(fd.Params))
	defaults := make([]bool, len(fd.Params))
	for i := range fd.Params {
		modes[i] = fd.Params[i].Mode
		if fd.Params[i].Default != nil {
			defaults[i] = true
		}
	}
	set.Add(&FuncCand{
		Decl:     fd,
		Type:     sig,
		Modes:    modes,
		Extern:   isExternDecl(fd), // <-- critical: mark local @extern functions
		Defaults: defaults,         // M14: record which params have defaults
	})

	// Track @test decorated functions for the test runner
	if hasDecorator(fd, "test") {
		c.info.TestFuncs[name] = fd
	}

	// Bind the function name in the OUTER scope (not the temp scope)
	_ = saved.Define(&Symbol{Name: name, Kind: SymFunc, Type: sig, Node: fd})
}

func (c *checker) checkFunc(fd *ast.FuncDecl) {
	// New scope for parameters and locals.
	saved := c.scope
	savedFuncName := c.curFuncName
	c.scope = NewScope(c.scope)
	c.curFuncName = fd.Name.Name
	defer func() {
		c.scope = saved
		c.curFuncName = savedFuncName
	}()

	// Reset per-function move-tracking state (our local tracker)
	c.moved = MoveSet{}
	if c.info != nil {
		c.info.Moved = make(map[string]diag.Span)
	}

	// Add generic type parameters to scope (like we do for enums)
	for _, typeParam := range fd.TypeParams {
		c.scope.Define(&Symbol{
			Name: typeParam.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: typeParam.Name},
		})
	}

	// Find canonical function type to get correct parameter types (including injected self)
	var funcType *types.Func
	if set := c.info.Funcs[fd.Name.Name]; set != nil {
		for _, cand := range set.Cands {
			if cand.Decl == fd {
				funcType = cand.Type
				break
			}
		}
	}
	if len(fd.Params) > 0 && fd.Params[0].Name.Name == "self" {
	}

	// Bind params.
	for i := range fd.Params {
		p := fd.Params[i]
		var pt types.T

		// Use canonical type if available (handles self injection and resolved types)
		if funcType != nil && i < len(funcType.Params) {
			pt = funcType.Params[i]
		} else {
			// Fallback (shouldn't happen for valid code, but safe)
			if p.Type != nil {
				pt = c.resolveType(p.Type)
			}
		}

		if p.Variadic {
			// If variadic, the canonical type might be the element type or list type depending on implementation.
			// In types.Func, Params[i] is the type of the parameter.
			// If it's *args, types.Func usually stores it as List[T] or similar?
			// Let's check types.Func definition.
			// types.Func has Variadic bool. Params are T.
			// Usually for variadic, the last param type is the element type or the slice type?
			// In Go, it's slice type.
			// In Desi collectFunc:
			// if p.Variadic { ... paramType = types.ListOf(elemType) ... }
			// So funcType.Params[i] IS the list type.
			// So we don't need to wrap it again if we got it from funcType.
			if funcType == nil {
				pt = types.ListOf(pt)
			}
		}

		_ = c.scope.Define(&Symbol{
			Name: p.Name.Name, Kind: SymParam, Type: pt, Node: &fd.Params[i].Name,
		})
	}

	// Declared return type (if any).
	c.curFuncRet = types.None
	if fd.RetType != nil {
		if tt := c.resolveType(fd.RetType); tt != nil {
			c.curFuncRet = tt
		}
	}

	// M14: validate parameter defaults (modes, trailing, const-ness, type match).
	c.validateParamDefaults(fd)

	// M6-C: callee-side borrow rule on async functions.
	c.checkAsyncInoutAwait(fd)

	// Body.
	if fd.Body != nil {
		c.checkBlock(fd.Body)
	}

	// Save moved variables for backend
	if c.info != nil && c.info.FuncMoves != nil {
		moves := make(map[string]bool)
		for _, k := range c.moved.Keys() {
			moves[k] = true
		}
		c.info.FuncMoves[fd] = moves
	}
}

func (c *checker) checkBlock(b *ast.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		c.checkStmt(s)
	}
}

// validateParamDefaults enforces:
//   - no defaults on inout params (DDF0002)
//   - trailing-defaults rule (DDF0003)
//   - default expr must be a compile-time constant (DDF0001)
//   - if param has a type, default type must match (DTE0004)
func (c *checker) validateParamDefaults(fd *ast.FuncDecl) {
	if fd == nil {
		return
	}

	// Trailing-defaults + inout rule.
	seenDefault := false
	for i := range fd.Params {
		p := &fd.Params[i]
		if p.Default != nil {
			if p.Mode == ast.ParamInout {
				c.add(diagAt("DDF0002", p.Name.Span, "inout parameters cannot have default values"))
			}
			seenDefault = true
		} else if seenDefault {
			// Non-trailing param without default after a defaulted param.
			c.add(diagAt("DDF0003", p.Name.Span, "non-default parameter cannot follow parameter with default"))
		}
	}

	// Const-ness + type compatibility of defaults.
	for i := range fd.Params {
		p := &fd.Params[i]
		if p.Default == nil {
			continue
		}

		// Const-ness: only a narrow set of literal forms are allowed in M14.
		if !isConstDefaultExpr(p.Default) {
			c.add(diagAt("DDF0001", p.Default.SpanOf(), "default value must be a compile-time constant"))
			// Don't try to type-match non-const defaults; we've already rejected them.
			continue
		}

		// If the param has a declared type, enforce that the default's type matches.
		if p.Type != nil {
			pt := surfaceToType(p.Type.Name)
			if pt != nil {
				dt := c.typ(p.Default)
				if dt != nil && !types.Equal(pt, dt) {
					c.add(diagAt("DTE0004", p.Default.SpanOf(), "default value has type "+dt.String()+", expected "+pt.String()))
				}
			}
		}
	}
}

// isConstDefaultExpr reports whether e is an allowed compile-time constant default
// expression in M14 (literals and simple unary-minus on numeric literals).
func isConstDefaultExpr(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.BoolLit, *ast.StrLit, *ast.NoneLit:
		return true
	case *ast.UnaryExpr:
		if x.Op == "-" {
			switch x.X.(type) {
			case *ast.IntLit, *ast.FloatLit:
				return true
			}
		}
		return false
	default:
		return false
	}
}

// isExternDecl reports whether a function has an @extern decorator (any args).
func isExternDecl(fd *ast.FuncDecl) bool {
	if fd == nil {
		return false
	}
	for _, dec := range fd.Decorators {
		if dec != nil && dec.Name.Name == "extern" {
			return true
		}
	}
	return false
}

// forbidBuiltinShadowing emits DPL0001 if a top-level binding redefines a builtin.
func (c *checker) forbidBuiltinShadowing(name string, sp diag.Span) {
	if c == nil || c.scope == nil {
		return
	}
	// Only enforce at module scope (top-level bindings).
	if c.scope == c.moduleScope && isPreludeBuiltinName(name) {
		c.add(diagAt("DPL0001", sp, "cannot shadow builtin: "+name))
	}
}

// collectLenNoLengthDiags appends DCO0001 for len(x) where type(x) has no length.
// Phase-1 allow-list: str only. (Collections get added later in M10.)
func collectLenNoLengthDiags(mod *ast.Module, info *Info) []diag.Diagnostic {
	var out []diag.Diagnostic
	if mod == nil || info == nil {
		return out
	}
	walkFunc := func(fd *ast.FuncDecl) {
		if fd == nil || fd.Body == nil {
			return
		}
		for _, s := range fd.Body.Stmts {
			es, ok := s.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := es.Expr.(*ast.CallExpr)
			if !ok || call == nil {
				continue
			}
			id, ok := call.Callee.(*ast.Ident)
			if !ok || id == nil || id.Name != "len" {
				continue
			}
			if len(call.Args) != 1 {
				continue
			}
			arg := call.Args[0]
			t := info.Types[arg]
			if t == nil {
				continue
			}
			// Allow str, list, dict, set, tuple
			switch t.(type) {
			case *types.Tuple, *types.List, *types.Dict, *types.Set:
				// These types support len()
				continue
			}
			if types.Equal(t, types.Str) {
				continue
			}
			out = append(out, diagAt("DCO0001", id.Span, "type has no length"))
		}
	}
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			walkFunc(dd)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				walkFunc(m)
			}
		}
	}
	return out
}

// collectMembershipDiags handles 'in' operator typing/diags (Phase-1: str in str).
func collectMembershipDiags(mod *ast.Module, info *Info) []diag.Diagnostic {
	var out []diag.Diagnostic
	if mod == nil || info == nil {
		return out
	}

	// local helper: if the checker didn't record a type (e.g., because it
	// didn't walk this BinaryExpr), infer the obvious literal kinds.
	litOrInfoType := func(e ast.Expr) types.T {
		if t := info.Types[e]; t != nil {
			return t
		}
		switch e.(type) {
		case *ast.StrLit:
			return types.Str
		case *ast.IntLit:
			return types.Int
		case *ast.FloatLit:
			return types.Float
		}
		return nil
	}

	walkFunc := func(fd *ast.FuncDecl) {
		if fd == nil || fd.Body == nil {
			return
		}
		for _, s := range fd.Body.Stmts {
			es, ok := s.(*ast.ExprStmt)
			if !ok {
				continue
			}
			be, ok := es.Expr.(*ast.BinaryExpr)
			if !ok || be == nil {
				continue
			}

			// Op is a string in our AST; check for "in".
			if be.Op != "in" {
				continue
			}

			lt := litOrInfoType(be.Lhs)
			rt := litOrInfoType(be.Rhs)

			// Phase-1 rule: str in str -> bool
			if lt != nil && rt != nil && types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
				info.Types[be] = types.Bool
				continue
			}

			// Tuple membership: x in (a, b, c) where tuple is homogeneous
			if lt != nil && rt != nil {
				if tupT, ok := rt.(*types.Tuple); ok {
					if len(tupT.Elems) > 0 {
						// Check if tuple is homogeneous and element type matches LHS
						firstType := tupT.Elems[0]
						allSame := true
						for _, e := range tupT.Elems {
							if !types.Equal(e, firstType) {
								allSame = false
								break
							}
						}
						if allSame && types.Equal(lt, firstType) {
							info.Types[be] = types.Bool
							continue
						}
					}
				}
			}

			out = append(out, diagAt("DCO0002", be.Span, "unsupported membership"))
			// Best-effort type to keep downstream happy.
			info.Types[be] = types.Bool
		}
	}
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			walkFunc(dd)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				walkFunc(m)
			}
		}
	}
	return out
}
