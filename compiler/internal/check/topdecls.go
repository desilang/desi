package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

/*
Top-level pass for Phase B:

• Records visibility bits for funcs/structs into Info.{FuncsPublic,StructsPublic}
• Records top-level consts into Info.Consts (+ Info.ConstsPublic)
• Builds import alias maps for this file:
    - c.aliases:     from-import alias  -> original name (symbol)
    - c.modAliases:  module alias name  -> module path ("util.math")
• Enforces:
    - DTE0012: `pub let mut` is forbidden
    - DTE0011: `pub let` must be a compile-time constant (Phase B: int/str/bool only)
    - Forbid reserved/builtins as top-level names (DTE0020/DTE0021)
    - Forbid redefining names introduced by imports (DTE0022)
    - From-import visibility (DTE0010) against the *source module*
*/

func (c *checker) collectVisibilityAndConsts(f *ast.File) {
	// Ensure maps exist
	if c.info.FuncsPublic == nil {
		c.info.FuncsPublic = map[string]bool{}
	}
	if c.info.StructsPublic == nil {
		c.info.StructsPublic = map[string]bool{}
	}
	if c.info.Consts == nil {
		c.info.Consts = map[string]ConstInfo{}
	}
	if c.info.ConstsPublic == nil {
		c.info.ConstsPublic = map[string]bool{}
	}
	if c.info.ImportedNames == nil {
		c.info.ImportedNames = map[string]bool{}
	}
	if c.aliases == nil {
		c.aliases = map[string]string{}
	}
	if c.modAliases == nil {
		c.modAliases = map[string]string{}
	}

	// ---------- Imports (build alias maps + visibility check on from-import) ----------

	// 1) from-imports: alias -> original name, validate visibility in source module
	for _, fi := range f.FromImports {
		modPath := strings.TrimSpace(fi.Module) // <-- ast.FromImportDecl.Module
		if modPath == "" {
			continue
		}
		for _, it := range fi.Items { // <-- ast.FromImportDecl.Items
			orig := strings.TrimSpace(it.Name)
			if orig == "" {
				continue
			}
			alias := strings.TrimSpace(it.As)
			if alias == "" {
				alias = orig
			}

			// alias hygiene
			if isReservedIdent(alias) {
				c.errors = append(c.errors, ErrReservedIdentifierAt(it.Span, alias, "from-import alias"))
			}
			if isPreludeBuiltin(alias) {
				c.errors = append(c.errors, ErrShadowBuiltinAt(it.Span, alias, "from-import alias"))
			}
			if _, dup := c.aliases[alias]; dup {
				c.errors = append(c.errors, ErrRedeclaredSymbolAt(it.Span, alias, "from-import alias"))
			}
			c.aliases[alias] = orig
			c.info.ImportedNames[alias] = true

			// **Visibility**: consult source module public tables if provider is present
			if DefaultModuleInfoProvider != nil {
				if src, ok := DefaultModuleInfoProvider.Lookup(modPath); ok && src != nil {
					switch {
					case hasConst(src, orig):
						if !src.ConstsPublic[orig] {
							c.errors = append(c.errors, ErrNotPublicAt(it.Span, orig, "from-import"))
						}
					case hasFunc(src, orig):
						if !src.FuncsPublic[orig] {
							c.errors = append(c.errors, ErrNotPublicAt(it.Span, orig, "from-import"))
						}
					case hasStruct(src, orig):
						if !src.StructsPublic[orig] {
							c.errors = append(c.errors, ErrNotPublicAt(it.Span, orig, "from-import"))
						}
					case hasType(src, orig):
						if !src.TypesPublic[orig] {
							c.errors = append(c.errors, ErrNotPublicAt(it.Span, orig, "from-import"))
						}
					case hasEnum(src, orig):
						if !src.EnumsPublic[orig] {
							c.errors = append(c.errors, ErrNotPublicAt(it.Span, orig, "from-import"))
						}
					default:
						// Unknown in that module — prefer a clear error
						c.errors = append(c.errors, ErrUndefinedNameAt(it.Span, orig, "from-import"))
					}
				}
			}
		}
	}

	// 2) module aliases: alias -> module path
	for _, im := range f.Imports {
		path := strings.TrimSpace(im.Path)
		if path == "" {
			continue
		}
		alias := strings.TrimSpace(im.As)
		if alias == "" {
			parts := strings.Split(path, ".")
			alias = parts[len(parts)-1]
		}

		// alias hygiene
		if isReservedIdent(alias) {
			c.errors = append(c.errors, ErrReservedIdentifierAt(im.Span, alias, "module alias"))
		}
		if isPreludeBuiltin(alias) {
			c.errors = append(c.errors, ErrShadowBuiltinAt(im.Span, alias, "module alias"))
		}
		if _, dup := c.modAliases[alias]; dup {
			c.errors = append(c.errors, ErrRedeclaredSymbolAt(im.Span, alias, "module alias"))
		}
		// collision with from-import alias
		if _, used := c.aliases[alias]; used {
			c.errors = append(c.errors, ErrImportNameConflictAt(im.Span, alias, "module alias"))
		}
		c.modAliases[alias] = path
		c.info.ImportedNames[alias] = true
	}

	// ---------- Top-level declarations ----------

	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			// Forbid reserved/builtins and imported names
			if isReservedIdent(v.Name) {
				c.errors = append(c.errors, ErrReservedIdentifierAt(v.Span, v.Name, "function"))
			}
			if isPreludeBuiltin(v.Name) {
				c.errors = append(c.errors, ErrShadowBuiltinAt(v.Span, v.Name, "function"))
			}
			if c.info.ImportedNames[v.Name] {
				c.errors = append(c.errors, ErrImportNameConflictAt(v.Span, v.Name, "function"))
			}
			c.info.FuncsPublic[v.Name] = v.Pub

		case *ast.StructDecl:
			if isReservedIdent(v.Name) {
				c.errors = append(c.errors, ErrReservedIdentifierAt(v.Span, v.Name, "struct"))
			}
			if isPreludeBuiltin(v.Name) {
				c.errors = append(c.errors, ErrShadowBuiltinAt(v.Span, v.Name, "struct"))
			}
			if c.info.ImportedNames[v.Name] {
				c.errors = append(c.errors, ErrImportNameConflictAt(v.Span, v.Name, "struct"))
			}
			c.info.StructsPublic[v.Name] = v.Pub

		case *ast.ConstDecl:
			// Forbid reserved/builtins and imported names
			if isReservedIdent(v.Name) {
				c.errors = append(c.errors, ErrReservedIdentifierAt(v.Span, v.Name, "constant"))
			}
			if isPreludeBuiltin(v.Name) {
				c.errors = append(c.errors, ErrShadowBuiltinAt(v.Span, v.Name, "constant"))
			}
			if c.info.ImportedNames[v.Name] {
				c.errors = append(c.errors, ErrImportNameConflictAt(v.Span, v.Name, "constant"))
			}

			// 1) `pub let mut` is forbidden (DTE0012).
			if v.Pub && v.Mutable {
				c.errors = append(c.errors, ErrPubLetMutForbidden(v.Span, v.Name))
			}

			// 2) `pub let` must be a compile-time constant (literal only).
			ck, isConst := constKindIfLiteral(v.Value)
			if v.Pub && !isConst {
				c.errors = append(c.errors, ErrPublicConstNotConst(v.Span, v.Name))
			}

			// Record const
			c.info.Consts[v.Name] = ConstInfo{Kind: ck}
			c.info.ConstsPublic[v.Name] = v.Pub && isConst && !v.Mutable
		}
	}
}

// Phase B: a constant is compile-time iff it's a bare literal (int/str/bool).
func constKindIfLiteral(e ast.Expr) (Kind, bool) {
	switch e.(type) {
	case *ast.IntLit:
		return KindInt, true
	case *ast.StrLit:
		return KindStr, true
	case *ast.BoolLit:
		return KindBool, true
	default:
		return KindUnknown, false
	}
}
