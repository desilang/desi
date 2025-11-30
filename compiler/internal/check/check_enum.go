package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// collectEnum registers an enum type in Pass 1.
// Creates a placeholder Enum with empty variants (populated in Pass 2).
func (c *checker) collectEnum(d *ast.EnumDecl) {
	// Create enum type (variants populated in Pass 2)
	et := &types.Enum{Name: d.Name.Name}
	for _, tp := range d.TypeParams {
		et.TypeParams = append(et.TypeParams, types.TypeParam{Name: tp.Name})
	}

	// Register the enum type name
	c.scope.Define(&Symbol{
		Name: d.Name.Name,
		Kind: SymType,
		Type: et,
		Node: d,
	})

	// Store type for backend access
	c.info.Types[d] = et

	// M14 Stage 2: Auto-generate default Display impl if not provided
	c.ensureDefaultDisplay(d.Name.Name)
}

// checkEnum resolves variant field types in Pass 2.
func (c *checker) checkEnum(d *ast.EnumDecl) {
	// Look up the enum type
	sym := c.scope.Lookup(d.Name.Name)
	if sym == nil || sym.Type == nil {
		return // Shouldn't happen if collectEnum ran
	}
	et, ok := sym.Type.(*types.Enum)
	if !ok {
		return
	}

	// Add type parameters to scope for generic enums
	// e.g., for "enum Option<T>", add T as a TypeParam
	c.scope = NewScope(c.scope)
	for _, typeParam := range d.TypeParams {
		c.scope.Define(&Symbol{
			Name: typeParam.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: typeParam.Name},
		})
	}

	// Resolve each variant
	for i, v := range d.Variants {
		variant := types.Variant{
			Name: v.Name.Name,
			Tag:  i, // 0-indexed tag
		}

		// Resolve variant type if present
		if v.Type != nil {
			variantType := c.resolveType(v.Type)
			// For now, treat the entire type as a single field
			// If it's a tuple, it will be handled as tuple[...]
			// For simple types like `int`, it's a single field
			variant.Fields = []types.Field{{
				Name: "value", // Generic name for the payload
				Type: variantType,
			}}
		}
		// If v.Type is nil, this is a unit variant (no payload)

		et.Variants = append(et.Variants, variant)
	}

	// Pop type parameter scope
	c.scope = c.scope.parent
}
