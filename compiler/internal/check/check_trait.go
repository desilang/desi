package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// collectTrait registers the trait in the scope (M14 stub).
func (c *checker) collectTrait(d *ast.TraitDecl) {
	// For M14, we just register the name so it's not "unknown type".
	// Real trait system would build a type symbol with method signatures.
	// We'll treat it as a type for now.
	// TODO: Add proper TraitSymbol to types package.
}

// collectImpl registers the impl (M14).
func (c *checker) collectImpl(d *ast.ImplDecl) {
	// Register methods for this type+trait combination.
	typeName := d.ForType.Name
	traitName := d.Trait.Name

	if c.info.Impls[typeName] == nil {
		c.info.Impls[typeName] = make(map[string][]*ast.FuncDecl)
	}
	c.info.Impls[typeName][traitName] = d.Methods // Replace, not append
}

// checkTraitBody checks the methods inside a trait (signatures).
func (c *checker) checkTraitBody(d *ast.TraitDecl) {
	for _, m := range d.Methods {
		// Check signature only?
		// For M14, we might just skip body check or ensure Body is nil.
		if m.Body != nil {
			c.add(diagAt("DTR0001", m.Body.SpanOf(), "trait methods cannot have bodies in M14"))
		}
	}
}

// checkImplBody checks the methods inside an impl.
func (c *checker) checkImplBody(d *ast.ImplDecl) {
	// Resolve the struct type that this impl is for
	var structType types.T
	if d.ForType != nil {
		structType = c.resolveType(d.ForType)
	}

	for _, m := range d.Methods {
		// If this method has a 'self' parameter, we need to set its type to the struct
		if len(m.Params) > 0 && m.Params[0].Name.Name == "self" {
			// Temporarily store the struct type for this parameter
			// We'll modify the AST node to include the type
			// Actually, we can't modify AST. Instead, we should register it in scope manually.
			// Let's use a different approach - modify checkFunc to accept context
			// OR we can set the parameter's Type field before calling checkFunc
			if structType != nil && m.Params[0].Type == nil {
				// Create a TypeName node for the struct
				m.Params[0].Type = d.ForType
			}
		}
		c.checkFunc(m)
	}
}
