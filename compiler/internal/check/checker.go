package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

type checker struct {
	info  *Info
	fnSig FuncSig

	scope *scope

	errors   []error
	warnings []Warning

	locals        []*varInfo
	blockReturned []bool

	// From-import alias map: alias -> original symbol name (includes no-`as` items as alias==name)
	aliases map[string]string

	// Module alias map: alias -> module path (e.g., "util.math")
	modAliases map[string]string

	// Feature gate (M11)
	features struct {
		Async bool
	}
}

// ---------- reserved names for aliasing ----------

func reservedAliasReason(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}

	// language keywords (Stage-1 superset, harmless if extra)
	keywords := map[string]struct{}{
		"package": {}, "import": {}, "from": {}, "as": {}, "def": {}, "struct": {}, "enum": {}, "type": {},
		"let": {}, "mut": {}, "if": {}, "elif": {}, "else": {}, "while": {}, "return": {}, "match": {}, "defer": {},
		"true": {}, "false": {},
	}
	if _, ok := keywords[n]; ok {
		return "reserved keyword"
	}

	// builtins / std shims / known module roots
	builtins := map[string]struct{}{
		"print": {}, "io": {}, "fs": {}, "os": {}, "mem": {}, "str": {},
	}
	if _, ok := builtins[n]; ok {
		return "reserved builtin"
	}

	return ""
}

// CheckFile performs semantic checks and returns info, errors, and warnings.
func CheckFile(f *ast.File) (*Info, []error, []Warning) {
	info := &Info{
		Funcs:         map[string]FuncSig{},
		Types:         map[string]string{},
		Structs:       map[string]StructInfo{},
		Enums:         map[string]EnumInfo{},
		Aliases:       map[string]string{},
		FuncsPublic:   map[string]bool{},
		StructsPublic: map[string]bool{},
		Consts:        map[string]ConstInfo{},
		ConstsPublic:  map[string]bool{},
		FuncsLocal:    map[string]bool{},
		TypesPublic:   map[string]bool{},
		EnumsPublic:   map[string]bool{},
		ImportedNames: map[string]bool{},
	}
	var errs []error
	var warns []Warning

	// Make imported names available early for top-level hygiene checks.
	if f != nil && info.ImportedNames != nil {
		collectImportedNamesInto(info.ImportedNames, f)
	}

	// Seed local funcs set from resolver's entry-file annotation.
	if f != nil && f.LocalFuncNames != nil {
		for n := range f.LocalFuncNames {
			info.FuncsLocal[n] = true
		}
	}

	// collect structs (M7)  publicity (M10) + top-level hygiene
	for _, d := range f.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			if err := enforceAllowedTopName(info, sd.Name, "struct"); err != nil {
				errs = append(errs, err)
			}
			fields := map[string]string{}
			for _, ft := range sd.Fields {
				fields[ft.Name] = ft.Type
			}
			info.Structs[sd.Name] = StructInfo{Fields: fields}
			if sd.Pub {
				info.StructsPublic[sd.Name] = true
			}
		}
	}

	for _, d := range f.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			variants := map[string]string{}
			for _, v := range ed.Variants {
				variants[v.Name] = v.Payload
			}
			info.Enums[ed.Name] = EnumInfo{Variants: variants}
			if ed.Pub {
				info.EnumsPublic[ed.Name] = true
			}
		}
	}

	for _, d := range f.Decls {
		if td, ok := d.(*ast.TypeDecl); ok {
			info.Types[td.Name] = td.Underlying
			if td.Pub {
				info.TypesPublic[td.Name] = true
			}
		}
	}

	// ---- NEW: forbid textual 'void' anywhere in type positions ----
	errs = append(errs, scanForVoidTypes(f)...)

	// collect top-level constants (M10) + top-level hygiene
	for _, d := range f.Decls {
		cd, ok := d.(*ast.ConstDecl)
		if !ok {
			continue
		}
		if err := enforceAllowedTopName(info, cd.Name, "constant"); err != nil {
			errs = append(errs, err)
		}
		// pub let mut is forbidden
		if cd.Pub && cd.Mutable {
			errs = append(errs, ErrPubLetMutForbidden(cd.Span, cd.Name))
		}
		// public constants must be compile-time constants (simple rule: literal only for now)
		if cd.Pub && !isConstExpr(cd.Value) {
			errs = append(errs, ErrPublicConstNotConst(cd.Span, cd.Name))
		}
		// record kind (best-effort; literal-only right now)
		k := constKind(cd.Value)
		info.Consts[cd.Name] = ConstInfo{Kind: k}
		if cd.Pub {
			info.ConstsPublic[cd.Name] = true
		}
	}

	// collect function signatures  publicity + top-level hygiene
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if err := enforceAllowedTopName(info, fn.Name, "function"); err != nil {
			errs = append(errs, err)
		}
		if _, exists := info.Funcs[fn.Name]; exists {
			errs = append(errs, fmt.Errorf("duplicate function %q", fn.Name))
			continue
		}
		var ps []Kind
		for _, p := range fn.Params {
			k, _ := mapTypeOrStruct(p.Type, info) // now also maps enums
			ps = append(ps, k)
		}
		retElem, _ := mapTypeOrStruct(fn.Ret, info)
		sig := FuncSig{
			Name:    fn.Name,
			Params:  ps,
			Ret:     KindUnknown,
			Async:   false,
			RetElem: KindUnknown,
		}
		if fn.Async {
			sig.Async = true
			sig.Ret = KindFuture
			sig.RetElem = retElem
		} else {
			sig.Ret = retElem
		}
		info.Funcs[fn.Name] = sig
		if fn.Pub {
			info.FuncsPublic[fn.Name] = true
		}
	}

	// ---------- Build alias maps  diagnostics ----------

	// 1) from-import aliases
	fromAliases := map[string]string{} // alias -> original
	for _, fi := range f.FromImports {
		seenLine := map[string]struct{}{}
		for _, it := range fi.Items {
			alias := strings.TrimSpace(it.As)
			if alias == "" {
				alias = it.Name // include no-`as` bindings
			}
			if reason := reservedAliasReason(alias); reason != "" {
				errs = append(errs, fmt.Errorf("invalid alias name %q (%s)", alias, reason))
			}
			if _, dup := seenLine[alias]; dup {
				errs = append(errs, fmt.Errorf("duplicate alias %q in from-import from %q", alias, fi.Module))
			}
			seenLine[alias] = struct{}{}

			if prev, exists := fromAliases[alias]; exists {
				errs = append(errs, fmt.Errorf("duplicate from-import alias %q (already used for %q)", alias, prev))
			} else {
				fromAliases[alias] = it.Name
			}

			// Phase B rule: from-imported symbol must be public
			name := it.Name
			switch {
			case hasConst(info, name) && !info.ConstsPublic[name]:
				errs = append(errs, ErrNotPublicAt(it.Span, name, "from-import"))
			case hasFunc(info, name) && !info.FuncsPublic[name]:
				errs = append(errs, ErrNotPublicAt(it.Span, name, "from-import"))
			case hasStruct(info, name) && !info.StructsPublic[name]:
				errs = append(errs, ErrNotPublicAt(it.Span, name, "from-import"))
			case hasType(info, name) && !info.TypesPublic[name]:
				errs = append(errs, ErrNotPublicAt(it.Span, name, "from-import"))
			case hasEnum(info, name) && !info.EnumsPublic[name]:
				errs = append(errs, ErrNotPublicAt(it.Span, name, "from-import"))
			default:
			}
		}
	}
	info.Aliases = fromAliases

	// 2) module aliases
	modAliases := map[string]string{} // alias -> module path
	for _, im := range f.Imports {
		alias := strings.TrimSpace(im.As)
		if alias == "" {
			p := strings.TrimSpace(im.Path)
			if p == "" {
				continue
			}
			parts := strings.Split(p, ".")
			alias = parts[len(parts)-1]
		}
		if reason := reservedAliasReason(alias); reason != "" {
			errs = append(errs, fmt.Errorf("invalid alias name %q (%s)", alias, reason))
		}
		if prev, exists := modAliases[alias]; exists {
			errs = append(errs, fmt.Errorf("duplicate module alias %q (for %q and %q)", alias, prev, im.Path))
			continue
		}
		modAliases[alias] = im.Path
	}

	// 3) cross conflicts
	for a, modPath := range modAliases {
		if orig, ok := fromAliases[a]; ok {
			errs = append(errs, fmt.Errorf("alias %q used both as module alias (from %q) and as from-import alias (to %q)", a, modPath, orig))
		}
	}

	// 4) NEW: record the set of *locally visible imported names* for hygiene checks.
	//     - from-imports introduce local names (= alias or item name)
	//     - plain imports introduce a local module alias (explicit or derived)
	if info.ImportedNames == nil {
		info.ImportedNames = map[string]bool{}
	}
	for a := range fromAliases {
		info.ImportedNames[a] = true
	}
	for a := range modAliases {
		info.ImportedNames[a] = true
	}

	// ---------- check bodies ----------
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			fnErrs, fnWarns := checkFunc(info, fn, fromAliases, modAliases)
			errs = append(errs, fnErrs...)
			warns = append(warns, fnWarns...)
		}
	}
	return info, errs, warns
}

// ---- helpers for public lookups / existence ----

func hasConst(info *Info, name string) bool  { _, ok := info.Consts[name]; return ok }
func hasFunc(info *Info, name string) bool   { _, ok := info.Funcs[name]; return ok }
func hasStruct(info *Info, name string) bool { _, ok := info.Structs[name]; return ok }
func hasType(info *Info, name string) bool   { _, ok := info.Types[name]; return ok }
func hasEnum(info *Info, name string) bool   { _, ok := info.Enums[name]; return ok }

// ---- helpers for public-const checks ----

// isConstExpr: Phase B minimal rule — only simple literals count as compile-time constants.
func isConstExpr(e ast.Expr) bool {
	switch e.(type) {
	case *ast.IntLit, *ast.StrLit, *ast.BoolLit:
		return true
	default:
		return false
	}
}

// constKind returns the Kind for a literal expression (Unknown otherwise).
func constKind(e ast.Expr) Kind {
	switch e.(type) {
	case *ast.IntLit:
		return KindInt
	case *ast.StrLit:
		return KindStr
	case *ast.BoolLit:
		return KindBool
	default:
		return KindUnknown
	}
}

func checkFunc(info *Info, fn *ast.FuncDecl, fromAliasMap map[string]string, modAliasMap map[string]string) ([]error, []Warning) {
	c := &checker{
		info:       info,
		fnSig:      info.Funcs[fn.Name],
		scope:      &scope{vars: map[string]*varInfo{}},
		locals:     nil,
		aliases:    fromAliasMap,
		modAliases: modAliasMap,
	}
	// Feature flags (default ON; CLI may toggle later)
	c.features.Async = true

	// params (immutable)  — STRICT: forbid collisions with imported names
	for i, p := range fn.Params {
		// STRICT RULE: parameter name must not collide with from-import alias or module alias
		if _, ok := c.aliases[p.Name]; ok {
			c.errors = append(c.errors, ErrImportNameConflict(p.Name, "parameter"))
		}
		if _, ok := c.modAliases[p.Name]; ok {
			c.errors = append(c.errors, ErrImportNameConflict(p.Name, "parameter"))
		}
		if err := enforceAllowedLocalName(c.info, p.Name, "parameter"); err != nil {
			c.errors = append(c.errors, err)
		}

		k, sname := mapTypeOrStruct(p.Type, info)
		v := &varInfo{
			kind:       k,
			mutable:    false,
			declName:   p.Name,
			structName: sname, // reused for both struct+enum
			read:       false,
			written:    true,
		}
		if err := c.scope.define(p.Name, v); err != nil {
			c.errors = append(c.errors, fmt.Errorf("parameter %d %q: %v", i, p.Name, err))
		}
		c.locals = append(c.locals, v)
	}

	c.blockReturned = push(c.blockReturned, false)
	for _, s := range fn.Body {
		c.checkStmt(s)
	}
	hasReturn := *top(c.blockReturned)
	c.blockReturned = pop(c.blockReturned)

	// Non-void fallthrough check (keep as-is)
	if fnRet := c.fnSig.Ret; fnRet != KindVoid && !hasReturn {
		if fnRet != KindFuture {
			tailExprOK := false
			if len(fn.Body) > 0 {
				if es, ok := fn.Body[len(fn.Body)-1].(*ast.ExprStmt); ok {
					tk := c.kindOfExpr(es.Expr)
					if _, ok := unifyKinds(fnRet, tk); ok {
						tailExprOK = true
					}
				}
			}
			if !tailExprOK {
				c.warnings = append(c.warnings, Warning{
					Code: warnCode("warn", "missing_explicit_return", "DW0006"),
					Msg:  fmt.Sprintf("function %q returns %s but may fall through without an explicit return", fn.Name, fnRet),
				})
			}
		}
	}

	// Unused vars/params (ignore names starting with "_")
	for _, v := range c.locals {
		if strings.HasPrefix(v.declName, "_") {
			continue
		}
		if !v.read {
			c.warnings = append(c.warnings, Warning{
				Code: warnCode("warn", "unused_variable", "DW0001"),
				Msg:  fmt.Sprintf("unused variable or parameter %q", v.declName),
			})
		}
	}

	return c.errors, c.warnings
}

// ---- NEW: scan textual 'void' in type positions ----

func scanForVoidTypes(f *ast.File) []error {
	var out []error

	isVoid := func(s string) bool {
		return strings.EqualFold(strings.TrimSpace(s), "void")
	}
	add := func(where string, typ string) {
		if isVoid(typ) {
			out = append(out, ErrUseNoneInsteadOfVoid(where))
		}
	}

	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			add(fmt.Sprintf("function %q return type", v.Name), v.Ret)
			for _, p := range v.Params {
				add(fmt.Sprintf("parameter %q of function %q", p.Name, v.Name), p.Type)
			}
		case *ast.StructDecl:
			for _, ft := range v.Fields {
				add(fmt.Sprintf("field %q of struct %q", ft.Name, v.Name), ft.Type)
			}
		case *ast.EnumDecl:
			for _, ev := range v.Variants {
				add(fmt.Sprintf("payload of %s.%s", v.Name, ev.Name), ev.Payload)
			}
		case *ast.TypeDecl:
			add(fmt.Sprintf("type alias %q", v.Name), v.Underlying)
		case *ast.ConstDecl:
			// (no explicit type field on ConstDecl today)
		}
	}
	return out
}
