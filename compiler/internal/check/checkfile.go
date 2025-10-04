package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

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

	// collect structs (M7)  publicity (M10) + top-level hygiene (span-based)
	for _, d := range f.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			// struct name hygiene
			if isReservedIdent(sd.Name) {
				errs = append(errs, ErrReservedIdentifierAt(sd.Span, sd.Name, "struct"))
			}
			if isPreludeBuiltin(sd.Name) {
				errs = append(errs, ErrShadowBuiltinAt(sd.Span, sd.Name, "struct"))
			}
			if info.ImportedNames[sd.Name] {
				errs = append(errs, ErrImportNameConflictAt(sd.Span, sd.Name, "struct"))
			}

			fields := map[string]string{}
			for _, ft := range sd.Fields {
				// struct field name hygiene (no import collisions here — field scope is local)
				if isReservedIdent(ft.Name) {
					errs = append(errs, ErrReservedIdentifierAt(ft.Span, ft.Name, "field"))
				}
				if isPreludeBuiltin(ft.Name) {
					errs = append(errs, ErrShadowBuiltinAt(ft.Span, ft.Name, "field"))
				}
				fields[ft.Name] = ft.Type
			}
			info.Structs[sd.Name] = StructInfo{Fields: fields}
			if sd.Pub {
				info.StructsPublic[sd.Name] = true
			}
		}
	}

	// Enums (with top-level hygiene on enum name; variant names get local hygiene)
	for _, d := range f.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			// enum name hygiene
			if isReservedIdent(ed.Name) {
				errs = append(errs, ErrReservedIdentifierAt(ed.Span, ed.Name, "enum"))
			}
			if isPreludeBuiltin(ed.Name) {
				errs = append(errs, ErrShadowBuiltinAt(ed.Span, ed.Name, "enum"))
			}
			if info.ImportedNames[ed.Name] {
				errs = append(errs, ErrImportNameConflictAt(ed.Span, ed.Name, "enum"))
			}

			variants := map[string]string{}
			for _, v := range ed.Variants {
				// variant identifier hygiene (no import collisions here)
				if isReservedIdent(v.Name) {
					errs = append(errs, ErrReservedIdentifierAt(v.Span, v.Name, "enum variant"))
				}
				if isPreludeBuiltin(v.Name) {
					errs = append(errs, ErrShadowBuiltinAt(v.Span, v.Name, "enum variant"))
				}
				variants[v.Name] = v.Payload
			}
			info.Enums[ed.Name] = EnumInfo{Variants: variants}
			if ed.Pub {
				info.EnumsPublic[ed.Name] = true
			}
		}
	}

	// Type aliases (top-level hygiene)
	for _, d := range f.Decls {
		if td, ok := d.(*ast.TypeDecl); ok {
			if isReservedIdent(td.Name) {
				errs = append(errs, ErrReservedIdentifierAt(td.Span, td.Name, "type"))
			}
			if isPreludeBuiltin(td.Name) {
				errs = append(errs, ErrShadowBuiltinAt(td.Span, td.Name, "type"))
			}
			if info.ImportedNames[td.Name] {
				errs = append(errs, ErrImportNameConflictAt(td.Span, td.Name, "type"))
			}
			info.Types[td.Name] = td.Underlying
			if td.Pub {
				info.TypesPublic[td.Name] = true
			}
		}
	}

	// ---- NEW: forbid textual 'void' anywhere in type positions ----
	errs = append(errs, scanForVoidTypes(f)...)

	// collect top-level constants (M10) + top-level hygiene (span-based)
	for _, d := range f.Decls {
		cd, ok := d.(*ast.ConstDecl)
		if !ok {
			continue
		}
		if isReservedIdent(cd.Name) {
			errs = append(errs, ErrReservedIdentifierAt(cd.Span, cd.Name, "constant"))
		}
		if isPreludeBuiltin(cd.Name) {
			errs = append(errs, ErrShadowBuiltinAt(cd.Span, cd.Name, "constant"))
		}
		if info.ImportedNames[cd.Name] {
			errs = append(errs, ErrImportNameConflictAt(cd.Span, cd.Name, "constant"))
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

	// collect function signatures  publicity + top-level hygiene (span-based)
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if isReservedIdent(fn.Name) {
			errs = append(errs, ErrReservedIdentifierAt(fn.Span, fn.Name, "function"))
		}
		if isPreludeBuiltin(fn.Name) {
			errs = append(errs, ErrShadowBuiltinAt(fn.Span, fn.Name, "function"))
		}
		if info.ImportedNames[fn.Name] {
			errs = append(errs, ErrImportNameConflictAt(fn.Span, fn.Name, "function"))
		}

		if _, exists := info.Funcs[fn.Name]; exists {
			errs = append(errs, ErrRedeclaredSymbolAt(fn.Span, fn.Name, "function"))
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
			// alias hygiene
			if isReservedIdent(alias) {
				errs = append(errs, ErrReservedIdentifierAt(it.Span, alias, "import alias"))
			}
			if isPreludeBuiltin(alias) {
				errs = append(errs, ErrShadowBuiltinAt(it.Span, alias, "import alias"))
			}
			// duplicates within a single from-import
			if _, dup := seenLine[alias]; dup {
				errs = append(errs, ErrRedeclaredSymbolAt(it.Span, alias, "from-import alias"))
			}
			seenLine[alias] = struct{}{}

			// duplicates across all from-imports
			if prev, exists := fromAliases[alias]; exists {
				_ = prev // previous symbol name not needed for typed diag
				errs = append(errs, ErrRedeclaredSymbolAt(it.Span, alias, "from-import alias"))
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
		// alias hygiene
		if isReservedIdent(alias) {
			errs = append(errs, ErrReservedIdentifierAt(im.Span, alias, "module alias"))
		}
		if isPreludeBuiltin(alias) {
			errs = append(errs, ErrShadowBuiltinAt(im.Span, alias, "module alias"))
		}
		// duplicate module alias
		if prev, exists := modAliases[alias]; exists {
			_ = prev
			errs = append(errs, ErrRedeclaredSymbolAt(im.Span, alias, "module alias"))
			continue
		}
		modAliases[alias] = im.Path
	}

	// 3) cross conflicts
	for a := range modAliases {
		if _, ok := fromAliases[a]; ok {
			errs = append(errs, ErrImportNameConflict(a, "alias used for both module import and from-import"))
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
