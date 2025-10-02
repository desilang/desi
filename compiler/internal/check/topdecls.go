package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

/*
Top-level pass for Phase B:

• Records visibility bits for funcs/structs into Info.{FuncsPublic,StructsPublic}
• Records top-level consts into Info.Consts (+ Info.ConstsPublic)
• Enforces:
    - DTE0012: `pub let mut` is forbidden
    - DTE0011: `pub let` must be a compile-time constant (Phase B: int/str/bool only)
    - Forbid reserved/builtins as top-level names (DTE0020/DTE0021)
    - Forbid redefining names introduced by imports (DTE0022)
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

	// 1) Collect identifiers introduced by imports in THIS file.
	collectImportedNamesInto(c.info.ImportedNames, f)

	// 2) Process top-level declarations.
	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			// Forbid reserved/builtins and imported names
			if isReservedIdent(v.Name) {
				c.errors = append(c.errors, ErrReservedIdentifier(v.Name, "function"))
			}
			if isPreludeBuiltin(v.Name) {
				c.errors = append(c.errors, ErrShadowBuiltin(v.Name, "function"))
			}
			if c.info.ImportedNames[v.Name] {
				c.errors = append(c.errors, ErrImportNameConflict(v.Name, "function"))
			}
			c.info.FuncsPublic[v.Name] = v.Pub

		case *ast.StructDecl:
			if isReservedIdent(v.Name) {
				c.errors = append(c.errors, ErrReservedIdentifier(v.Name, "struct"))
			}
			if isPreludeBuiltin(v.Name) {
				c.errors = append(c.errors, ErrShadowBuiltin(v.Name, "struct"))
			}
			if c.info.ImportedNames[v.Name] {
				c.errors = append(c.errors, ErrImportNameConflict(v.Name, "struct"))
			}
			c.info.StructsPublic[v.Name] = v.Pub

		case *ast.ConstDecl:
			// Forbid reserved/builtins and imported names
			if isReservedIdent(v.Name) {
				c.errors = append(c.errors, ErrReservedIdentifier(v.Name, "constant"))
			}
			if isPreludeBuiltin(v.Name) {
				c.errors = append(c.errors, ErrShadowBuiltin(v.Name, "constant"))
			}
			if c.info.ImportedNames[v.Name] {
				c.errors = append(c.errors, ErrImportNameConflict(v.Name, "constant"))
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
