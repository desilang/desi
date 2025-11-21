package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
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
	for _, m := range d.Methods {
		c.checkFunc(m)
	}
}
