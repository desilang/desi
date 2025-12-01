package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) collectStruct(d *ast.StructDecl) {
	// Create struct type (fields populated in Pass 2)
	st := &types.Struct{Name: d.Name.Name}
	for _, tp := range d.TypeParams {
		st.TypeParams = append(st.TypeParams, types.TypeParam{Name: tp.Name})
	}

	// Register the struct type name.
	c.scope.Define(&Symbol{
		Name: d.Name.Name,
		Kind: SymType,
		Type: st,
		Node: d,
	})

	// Store type for backend access
	c.info.Types[d] = st

	// M14 Stage 2: Auto-generate default Display impl if not provided
	c.ensureDefaultDisplay(d.Name.Name)
}

func (c *checker) collectClass(d *ast.ClassDecl) {
	// Create class type with methods and dunders tracking
	cls := &types.Class{
		Name:     d.Name.Name,
		Methods:  make(map[string]*types.Func),
		Dunders:  make(map[string]*types.Func),
		IsNested: false, // TODO: Detect if nested
	}

	// Add type parameters
	for _, tp := range d.TypeParams {
		cls.TypeParams = append(cls.TypeParams, types.TypeParam{Name: tp.Name})
	}

	// Register the class type
	c.scope.Define(&Symbol{
		Name: d.Name.Name,
		Kind: SymType,
		Type: cls,
		Node: d,
	})

	c.info.Types[d] = cls

	// Collect methods (will be fully checked in checkClass)
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

	// Add type parameters to scope for generic structs
	// e.g., for "struct Pair<A, B>", add A and B as TypeParams
	c.scope = NewScope(c.scope)
	defer func() { c.scope = c.scope.parent }()

	for _, typeParam := range d.TypeParams {
		c.scope.Define(&Symbol{
			Name: typeParam.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: typeParam.Name},
		})
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

func (c *checker) checkClass(d *ast.ClassDecl) {
	sym := c.scope.Lookup(d.Name.Name)
	if sym == nil {
		return
	}

	cls, ok := sym.Type.(*types.Class)
	if !ok {
		return
	}

	// Add type parameters to scope
	c.scope = NewScope(c.scope)
	defer func() { c.scope = c.scope.parent }()

	for _, tp := range d.TypeParams {
		c.scope.Define(&Symbol{
			Name: tp.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: tp.Name},
		})
	}

	// Handle inheritance (single base class)
	if len(d.Bases) > 0 {
		baseType := c.resolveType(d.Bases[0])
		if baseCls, ok := baseType.(*types.Class); ok {
			cls.Base = baseCls
			// Inherit fields (base first)
			cls.Fields = append(baseCls.Fields, cls.Fields...)
			// Inherit methods (can override)
			for name, method := range baseCls.Methods {
				if _, exists := cls.Methods[name]; !exists {
					cls.Methods[name] = method
				}
			}
			// Inherit dunders (can override)
			for name, dunder := range baseCls.Dunders {
				if _, exists := cls.Dunders[name]; !exists {
					cls.Dunders[name] = dunder
				}
			}
		} else {
			c.add(diagAt("DTE0004", d.Bases[0].Span, "base must be a class"))
		}
	}

	// Resolve fields
	for _, field := range d.Fields {
		fieldType := c.resolveType(field.Type)
		cls.Fields = append(cls.Fields, types.Field{
			Name:  field.Name.Name,
			Type:  fieldType,
			IsPub: field.Pub,
		})
	}

	// Process methods
	for _, method := range d.Methods {
		methodName := method.Name.Name
		isDunder := strings.HasPrefix(methodName, "__") && strings.HasSuffix(methodName, "__")

		// POLICY: All dunders MUST be pub
		if isDunder && !method.Pub {
			c.add(diagAt("DCL0001", method.Span, fmt.Sprintf("dunder method %s must be pub", methodName)))
		}

		// Look up method symbol
		methodSym := c.scope.Lookup(methodName)
		if methodSym == nil {
			continue
		}

		ft, ok := methodSym.Type.(*types.Func)
		if !ok {
			continue
		}

		// POLICY: Inject implicit self parameter if not present
		// Check if first param is already self (explicit)
		hasSelf := false
		if len(ft.Params) > 0 {
			// If first param is the class type, it's explicit self
			if types.Equal(ft.Params[0], cls) {
				hasSelf = true
			}
		}

		if !hasSelf {
			// Prepend self as first parameter
			ft.Params = append([]types.T{cls}, ft.Params...)
		}

		if isDunder {
			cls.Dunders[methodName] = ft
		} else {
			cls.Methods[methodName] = ft
		}
	}
}
