package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
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

// implementsTrait checks if a type implements a specific trait.
// Returns true if the type has an `impl Trait for Type` in the current module.
func (c *checker) implementsTrait(t types.T, traitName string) bool {
	if t == nil || traitName == "" {
		return false
	}

	// Get the type name from the type
	typeName := getTypeName(t)
	if typeName == "" {
		return false
	}

	// Check if there's an impl for this type+trait
	if impls, ok := c.info.Impls[typeName]; ok {
		if _, hasTrait := impls[traitName]; hasTrait {
			return true
		}
	}

	// Built-in trait implementations for primitives
	switch traitName {
	case "Display":
		// All primitives implement Display
		if isPrimitiveType(t) {
			return true
		}
	case "Numeric":
		// int, float, and sized integers implement Numeric
		if isNumericType(t) {
			return true
		}
	case "Eq", "PartialEq":
		// Most types implement Eq
		if isPrimitiveType(t) || isComparableType(t) {
			return true
		}
	case "Ord", "PartialOrd":
		// Numeric types implement Ord
		if isNumericType(t) || types.Equal(t, types.Str) {
			return true
		}
	case "Copy":
		// Primitives are Copy
		if isPrimitiveType(t) {
			return true
		}
	case "Send":
		return types.IsSend(t)
	case "Sync":
		return types.IsSync(t)
	}

	return false
}

// checkBounds validates that a type argument satisfies all required trait bounds.
// Returns nil if all bounds are satisfied, or a diagnostic if any bound is unsatisfied.
func (c *checker) checkBounds(typeArg types.T, bounds []string, span diag.Span) *diag.Diagnostic {
	if len(bounds) == 0 {
		return nil // No bounds to check
	}

	for _, bound := range bounds {
		if !c.implementsTrait(typeArg, bound) {
			msg := typeArg.String() + " does not implement trait " + bound
			d := diagAt("DSY0010", span, msg)
			return &d
		}
	}

	return nil
}

// extractBoundsFromTypeParams extracts bound names from AST TypeParamNode.
// Returns a slice of trait name strings for the bounds.
func extractBoundsFromTypeParams(tp *ast.TypeParamNode) []string {
	if tp == nil || len(tp.Bounds) == 0 {
		return nil
	}

	bounds := make([]string, 0, len(tp.Bounds))
	for _, b := range tp.Bounds {
		if b != nil {
			bounds = append(bounds, b.Name)
		}
	}
	return bounds
}

// getTypeName extracts the name from a type for impl lookup.
func getTypeName(t types.T) string {
	switch x := t.(type) {
	case *types.Struct:
		return x.Name
	case *types.Class:
		return x.Name
	case *types.Enum:
		return x.Name
	case *types.Generic:
		return getTypeName(x.Base)
	case *types.TypeAlias:
		return x.Name
	}
	// For primitives, use their String() representation
	if t != nil {
		return t.String()
	}
	return ""
}

// isPrimitiveType returns true for built-in primitive types.
func isPrimitiveType(t types.T) bool {
	switch t {
	case types.Int, types.Float, types.Bool, types.Str, types.None:
		return true
	case types.I8, types.I16, types.I32, types.I64:
		return true
	case types.U8, types.U16, types.U32, types.U64:
		return true
	case types.F32, types.F64:
		return true
	}
	return false
}

// isNumericType returns true for numeric types (implement Numeric trait).
func isNumericType(t types.T) bool {
	switch t {
	case types.Int, types.Float:
		return true
	case types.I8, types.I16, types.I32, types.I64:
		return true
	case types.U8, types.U16, types.U32, types.U64:
		return true
	case types.F32, types.F64:
		return true
	}
	return false
}

// isComparableType returns true for types that support equality comparison.
func isComparableType(t types.T) bool {
	// Primitives, structs with Eq, enums, etc.
	if isPrimitiveType(t) {
		return true
	}
	switch t.(type) {
	case *types.Struct, *types.Enum, *types.Class:
		return true
	}
	return false
}
