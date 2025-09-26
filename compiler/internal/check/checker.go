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

	// From-import alias map: alias -> original symbol name
	aliases map[string]string

	// Module alias map: alias -> module path (e.g., "util.math")
	modAliases map[string]string
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
		Funcs:   map[string]FuncSig{},
		Types:   map[string]string{},
		Structs: map[string]StructInfo{},
		Enums:   map[string]EnumInfo{}, // NEW
	}
	var errs []error
	var warns []Warning

	// collect structs (M7)
	for _, d := range f.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			fields := map[string]string{}
			for _, ft := range sd.Fields {
				fields[ft.Name] = ft.Type
			}
			info.Structs[sd.Name] = StructInfo{Fields: fields}
		}
	}

	// collect enums (M8)
	for _, d := range f.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			variants := map[string]string{}
			for _, v := range ed.Variants {
				variants[v.Name] = v.Payload
			}
			info.Enums[ed.Name] = EnumInfo{Variants: variants}
		}
	}

	// collect function signatures
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
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
		retK, _ := mapTypeOrStruct(fn.Ret, info)
		info.Funcs[fn.Name] = FuncSig{Name: fn.Name, Params: ps, Ret: retK}
	}

	// ---------- Build alias maps + diagnostics ----------

	// 1) from-import aliases
	fromAliases := map[string]string{} // alias -> original
	for _, fi := range f.FromImports {
		seenLine := map[string]struct{}{}
		for _, it := range fi.Items {
			a := strings.TrimSpace(it.As)
			if a == "" {
				continue // only 'as' aliases are tracked here
			}
			if reason := reservedAliasReason(a); reason != "" {
				errs = append(errs, fmt.Errorf("invalid alias name %q (%s)", a, reason))
			}
			if _, dup := seenLine[a]; dup {
				errs = append(errs, fmt.Errorf("duplicate alias %q in from-import from %q", a, fi.Module))
			}
			seenLine[a] = struct{}{}
			if prev, exists := fromAliases[a]; exists {
				errs = append(errs, fmt.Errorf("duplicate from-import alias %q (already used for %q)", a, prev))
			} else {
				fromAliases[a] = it.Name
			}
		}
	}

	// 2) module aliases
	modAliases := map[string]string{} // alias -> module path
	for _, im := range f.Imports {
		if len(im.Aliases) == 0 {
			continue
		}
		alias := strings.TrimSpace(im.Aliases[0])
		if alias == "" {
			continue
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

	// 3) cross conflicts: same name used as module alias and from-alias
	for a, modPath := range modAliases {
		if orig, ok := fromAliases[a]; ok {
			errs = append(errs, fmt.Errorf("alias %q used both as module alias (from %q) and as from-import alias (to %q)", a, modPath, orig))
		}
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

func checkFunc(info *Info, fn *ast.FuncDecl, fromAliasMap map[string]string, modAliasMap map[string]string) ([]error, []Warning) {
	c := &checker{
		info:       info,
		fnSig:      info.Funcs[fn.Name],
		scope:      &scope{vars: map[string]*varInfo{}},
		locals:     nil,
		aliases:    fromAliasMap,
		modAliases: modAliasMap,
	}
	// params (immutable)
	for i, p := range fn.Params {
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

	// Non-void fallthrough check:
	// Accept either an explicit return OR a tail expression stmt as satisfying.
	if fnRet := c.fnSig.Ret; fnRet != KindVoid && !hasReturn {
		tailExprOK := false
		if len(fn.Body) > 0 {
			if es, ok := fn.Body[len(fn.Body)-1].(*ast.ExprStmt); ok {
				// quick kind check: tail expr kind should unify with return kind
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
