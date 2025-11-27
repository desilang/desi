package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) collectStruct(d *ast.StructDecl) {
	// Create struct type (fields populated in Pass 2)
	st := &types.Struct{Name: d.Name.Name}

	// Register the struct type name.
	c.scope.Define(&Symbol{
		Name: d.Name.Name,
		Kind: SymType,
		Type: st,
		Node: d,
	})

	// M14 Stage 2: Auto-generate default Display impl if not provided
	c.ensureDefaultDisplay(d.Name.Name)
}

func (c *checker) collectClass(d *ast.ClassDecl) {
	// Register the class type name.
	c.scope.Define(&Symbol{
		Name: d.Name.Name,
		Kind: SymType,
		Type: types.Type,
		Node: d,
	})

	// Collect methods.
	for _, m := range d.Methods {
		c.collectFunc(m)
	}

	// M14 Stage 2: Auto-generate default Display impl if not provided
	c.ensureDefaultDisplay(d.Name.Name)
}

// ensureDefaultDisplay creates a default impl Display if one doesn't exist.
func (c *checker) ensureDefaultDisplay(typeName string) {
	// Check if Display is already implemented for this type
	if impls, ok := c.info.Impls[typeName]; ok {
		if _, hasDisplay := impls["Display"]; hasDisplay {
			// Already has Display impl, nothing to do
			return
		}
	}

	// Synthesize a default to_str() method
	// The method signature: def to_str() -> str
	defaultMethod := &ast.FuncDecl{
		Name:    ast.Ident{Name: "to_str"},
		Params:  nil, // No parameters for to_str
		RetType: &ast.TypeName{Name: "str"},
		Body:    nil,         // We'll handle the body in the backend/IR
		Span:    diag.Span{}, // Synthetic, no source location
	}

	// Register in Info.Impls
	if c.info.Impls[typeName] == nil {
		c.info.Impls[typeName] = make(map[string][]*ast.FuncDecl)
	}
	c.info.Impls[typeName]["Display"] = []*ast.FuncDecl{defaultMethod}
}

func (c *checker) checkStruct(d *ast.StructDecl) {
	sym := c.scope.Lookup(d.Name.Name)
	if sym == nil || sym.Type == nil {
		return
	}

	st, ok := sym.Type.(*types.Struct)
	if !ok {
		return
	}

	// Resolve fields
	for _, f := range d.Fields {
		var fieldType types.T = types.Any // default
		if f.Type != nil {
			fieldType = c.resolveType(f.Type)
		}
		st.Fields = append(st.Fields, types.Field{
			Name: f.Name.Name,
			Type: fieldType,
		})
	}
}
