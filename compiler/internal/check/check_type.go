package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/eval"
	"github.com/desilang/desi/compiler/internal/macro"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) collectStruct(d *ast.StructDecl) {
	// Create struct type (fields populated in Pass 2)
	st := &types.Struct{Name: d.Name.Name}
	for _, tp := range d.TypeParams {
		bounds := extractBoundsFromTypeParams(tp)
		st.TypeParams = append(st.TypeParams, types.TypeParam{Name: tp.Name.Name, Bounds: bounds})
	}

	// Check for @ffi_struct decorator for C-compatible layout
	for _, dec := range d.Decorators {
		if dec.Name.Name == "ffi_struct" {
			st.FFI = true
		}
		if dec.Name.Name == "packed" {
			st.Packed = true
		}
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

// collectTypeAlias binds a type alias name to a TypeAlias wrapper for nominal typing.
// For non-generic aliases: Two aliases with different names are distinct types.
// For generic aliases: Currently structural (expand to target) for simplicity.
func (c *checker) collectTypeAlias(d *ast.TypeAliasDecl) {
	// Create a temporary scope to add type parameters (for generic type aliases)
	saved := c.scope
	c.scope = NewScope(c.scope)
	defer func() { c.scope = saved }()

	// Add type parameters to scope (e.g., for Box<T>)
	for _, tp := range d.TypeParams {
		bounds := extractBoundsFromTypeParams(tp)
		c.scope.Define(&Symbol{
			Name: tp.Name.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: tp.Name.Name, Bounds: bounds},
		})
	}

	// Resolve the target type
	target := c.resolveType(d.Target)
	if target == nil {
		c.add(diagAt("DTE0001", d.Target.Span, "unknown type '"+d.Target.Name+"'"))
		return
	}

	// For generic type aliases, use structural typing (just the target type)
	// For non-generic aliases, wrap in TypeAlias for nominal typing
	var symType types.T
	if len(d.TypeParams) > 0 {
		// Generic alias: structural (use target directly)
		symType = target
	} else {
		// Non-generic alias: nominal  (wrap in TypeAlias)
		symType = &types.TypeAlias{
			Name:   d.Name.Name,
			Target: target,
		}
	}

	// Register the alias type name in the OUTER scope
	saved.Define(&Symbol{
		Name: d.Name.Name,
		Kind: SymType,
		Type: symType,
		Node: d,
	})

	// Store type for backend access
	c.info.Types[d] = symType
}

func (c *checker) collectClass(d *ast.ClassDecl) {
	// Skip @macro class definitions — they're configuration, not runtime classes.
	for _, dec := range d.Decorators {
		if dec.Name.Name == "macro" {
			return
		}
	}

	// Create Class type
	cls := &types.Class{
		Name:          d.Name.Name,
		TypeParams:    make([]types.TypeParam, 0, len(d.TypeParams)), // Initialize TypeParams slice
		Fields:        nil,                                           // will be populated in Phase 2
		Methods:       make(map[string]*types.Func),
		StaticMethods: make(map[string]*types.Func),
		ClassMethods:  make(map[string]*types.Func),
		Properties:    make(map[string]*types.Func),
		Constants:     make(map[string]*types.ClassConstant),
		StaticFields:  make(map[string]*types.ClassStaticField),
		Dunders:       make(map[string]*types.Func),
		Constructors:  nil, // will be populated in checkClass
		Base:          nil, // will be resolved in Phase 2
		IsNested:      false,
		Decl:          d,
	}

	// Check for macro-registered decorators (Tier 2: @model, etc.)
	// Built-in decorators (Tier 1: @ffi_struct, @packed, etc.) are handled above.
	for _, dec := range d.Decorators {
		if macro.IsBuiltinDecorator(dec.Name.Name) {
			continue // Tier 1: handled by existing hardcoded logic
		}
		if proto := macro.Registry.Lookup(dec.Name.Name); proto != nil {
			if proto.OnCollect != nil {
				ctx := &macro.MacroContext{
					Class:     cls,
					Decorator: dec,
					ClassDecl: d,
				}
				proto.OnCollect(ctx)
			}
		}
	}

	// Add type parameters
	for _, tp := range d.TypeParams {
		bounds := extractBoundsFromTypeParams(tp)
		cls.TypeParams = append(cls.TypeParams, types.TypeParam{Name: tp.Name.Name, Bounds: bounds})
	}

	// Register the class type
	c.scope.Define(&Symbol{
		Name: d.Name.Name,
		Kind: SymType,
		Type: cls,
		Node: d,
	})

	c.info.Types[d] = cls

	// Create inner scope for method collection so generic parameters are visible
	c.scope = NewScope(c.scope)
	defer func() { c.scope = c.scope.parent }()

	for i, tp := range d.TypeParams {
		c.scope.Define(&Symbol{
			Name: tp.Name.Name,
			Kind: SymType,
			Type: &cls.TypeParams[i],
		})
	}

	// Collect methods (will be fully checked in checkClass)
	for _, m := range d.Methods {
		c.collectFunc(m)
	}

	// Collect nested classes
	for _, nested := range d.Nested {
		// Error if nested class has its own nested classes (only 2-level nesting allowed)
		if len(nested.Nested) > 0 {
			c.add(diagAt("DTE0200", nested.Nested[0].Span, "nested classes cannot contain further nested classes"))
			continue
		}
		// Create Class type for nested class with IsNested=true
		// Use qualified name (Outer.Inner) to avoid collisions and allow correct mangling
		nestedCls := &types.Class{
			Name:          d.Name.Name + "." + nested.Name.Name,
			TypeParams:    make([]types.TypeParam, 0, len(nested.TypeParams)),
			Fields:        make([]types.Field, 0, len(nested.Fields)),
			Methods:       make(map[string]*types.Func),
			StaticMethods: make(map[string]*types.Func),
			ClassMethods:  make(map[string]*types.Func),
			Properties:    make(map[string]*types.Func),
			Constants:     make(map[string]*types.ClassConstant),
			StaticFields:  make(map[string]*types.ClassStaticField),
			Dunders:       make(map[string]*types.Func),
			IsNested:      true,
			Decl:          nested,
		}
		// Add type parameters
		for _, tp := range nested.TypeParams {
			bounds := extractBoundsFromTypeParams(tp)
			nestedCls.TypeParams = append(nestedCls.TypeParams, types.TypeParam{Name: tp.Name.Name, Bounds: bounds})
		}

		// Create nested scope for type parameters if any (so T is visible for fields/methods)
		if len(nested.TypeParams) > 0 {
			c.scope = NewScope(c.scope)
			for i, tp := range nested.TypeParams {
				c.scope.Define(&Symbol{
					Name: tp.Name.Name,
					Kind: SymType,
					Type: &nestedCls.TypeParams[i],
				})
			}
		}

		// Collect fields for nested class (so self.field works in methods)
		for _, field := range nested.Fields {
			var fieldType types.T = types.Any
			if field.Type != nil {
				if ft := c.resolveType(field.Type); ft != nil {
					fieldType = ft
				}
			}
			nestedCls.Fields = append(nestedCls.Fields, types.Field{
				Name:  field.Name.Name,
				Type:  fieldType,
				IsPub: field.Pub,
				IsMut: field.Mut,
			})
		}
		// Collect methods for nested class (so they are in c.info.Funcs for checkFunc)
		for _, m := range nested.Methods {
			c.collectFunc(m)
		}

		// Restore scope if we created one for type params
		if len(nested.TypeParams) > 0 {
			c.scope = c.scope.parent
		}
		// Register in outer scope (not the inner scope created for type params)
		c.scope.parent.Define(&Symbol{
			Name: nested.Name.Name,
			Kind: SymType,
			Type: nestedCls,
			Node: nested,
		})
		c.info.Types[nested] = nestedCls
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
		bounds := extractBoundsFromTypeParams(typeParam)
		c.scope.Define(&Symbol{
			Name: typeParam.Name.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: typeParam.Name.Name, Bounds: bounds},
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

		// Validate FFI-compatible types for @ffi_struct
		if st.FFI && !isFFICompatible(fieldType) {
			c.add(diagAt("DFI0004", f.Span,
				fmt.Sprintf("@ffi_struct field '%s' has non-FFI-compatible type '%s'", f.Name.Name, fieldType)))
		}
	}

	// FFI structs cannot be generic
	if st.FFI && len(st.TypeParams) > 0 {
		c.add(diagAt("DFI0005", d.Span, "@ffi_struct cannot be generic"))
	}
}

func (c *checker) checkClass(d *ast.ClassDecl) {
	// Execute custom macro decorators first
	for _, dec := range d.Decorators {
		if macroDef, exists := c.macroDefs[dec.Name.Name]; exists {
			env := eval.NewEnv(nil)
			if len(macroDef.Params) > 0 {
				paramName := macroDef.Params[0].Name.Name
				env.Set(paramName, eval.AstNodeValue{Node: d})
			}
			_, err := eval.Eval(macroDef.Body, env)
			if err != nil {
				c.add(diagAt("DTE9999", d.SpanOf(), "macro evaluation failed: "+err.Error()))
			}
		}
	}

	var cls *types.Class

	// Try scope lookup first (for top-level classes)
	sym := c.scope.Lookup(d.Name.Name)
	if sym != nil {
		cls, _ = sym.Type.(*types.Class)
	}

	// Fallback to c.info.Types for nested classes
	if cls == nil {
		if t, ok := c.info.Types[d]; ok {
			cls, _ = t.(*types.Class)
		}
	}

	if cls == nil {
		return
	}

	// Skip @macro class definitions — they're pure configuration consumed
	// by the macro loader, not runtime classes. Checking them would reject
	// dict/list literals used in macro configuration constants.
	for _, dec := range d.Decorators {
		if dec.Name.Name == "macro" {
			return
		}
	}

	// Add type parameters to scope
	c.scope = NewScope(c.scope)
	defer func() { c.scope = c.scope.parent }()

	for _, tp := range d.TypeParams {
		bounds := extractBoundsFromTypeParams(tp)
		c.scope.Define(&Symbol{
			Name: tp.Name.Name,
			Kind: SymType,
			Type: &types.TypeParam{Name: tp.Name.Name, Bounds: bounds},
		})
	}

	// Handle inheritance (single base class)
	if len(d.Bases) > 0 {
		baseType := c.resolveType(d.Bases[0])

		var baseCls *types.Class
		var subst map[string]types.T

		if cls, ok := baseType.(*types.Class); ok {
			baseCls = cls
		} else if gen, ok := baseType.(*types.Generic); ok {
			if cls, ok := gen.Base.(*types.Class); ok {
				baseCls = cls
				// Create substitution map
				if len(cls.TypeParams) == len(gen.Args) {
					subst = make(map[string]types.T)
					for i, tp := range cls.TypeParams {
						subst[tp.Name] = gen.Args[i]
					}
				}
			}
		}

		if baseCls != nil {
			cls.Base = baseCls

			// Inherit fields (base first)
			if subst == nil {
				cls.Fields = append(baseCls.Fields, cls.Fields...)
			} else {
				// Apply substitution to inherited fields
				for _, f := range baseCls.Fields {
					newType := substitute(f.Type, subst)
					cls.Fields = append(cls.Fields, types.Field{
						Name:  f.Name,
						Type:  newType,
						IsPub: f.IsPub,
						IsMut: f.IsMut,
					})
				}
				// Sort inherited fields to be before own fields
				// The loop above appended to end. We need [inherited..., own...]
				// But wait, cls.Fields currently only has defaults? No, resolveFields hasn't run yet.
				// cls.Fields is empty initially for a new class, right?
				// Actually, 'd.Fields' has AST fields. 'cls.Fields' is what we are building.
				// Wait, if we are revisiting checking (circular checks?), cls.Fields might be populated?
				// But here we are building it. 'cls' is the type object created in collectClass.
				// cls.Fields should be empty at this point in checkClass?
				// Let's check collectClass.
			}

			// Inherit methods (can override)
			for name, method := range baseCls.Methods {
				if _, exists := cls.Methods[name]; !exists {
					// Substitute types in method signatures for generic classes
					if subst != nil {
						newMethod := *method
						newParams := make([]types.T, len(method.Params))
						for i, p := range method.Params {
							newParams[i] = substitute(p, subst)
						}
						newRet := substitute(method.Ret, subst)
						newMethod.Params = newParams
						newMethod.Ret = newRet
						cls.Methods[name] = &newMethod
					} else {
						cls.Methods[name] = method
					}
				}
			}
			// Inherit dunders (can override)
			for name, dunder := range baseCls.Dunders {
				if _, exists := cls.Dunders[name]; !exists {
					cls.Dunders[name] = dunder
				}
			}
		} else {
			c.add(diagAt("DTE0999", d.Bases[0].Span, "CRITICAL_ERROR: base must be a class (or generic instantiation of class)"))
		}
	}

	// Resolve fields (with ORM field type handling for @model classes)
	for _, field := range d.Fields {
		var fieldType types.T

		if cls.MacroDecorator != "" && field.Type != nil {
			// Check if field type is an ORM field descriptor
			ormField, desiType := c.resolveOrmField(field.Name.Name, field.Type)
			if ormField != nil {
				cls.ModelFields = append(cls.ModelFields, *ormField)
				fieldType = desiType
			} else {
				fieldType = c.resolveType(field.Type)
			}
		} else {
			fieldType = c.resolveType(field.Type)
		}

		cls.Fields = append(cls.Fields, types.Field{
			Name:  field.Name.Name,
			Type:  fieldType,
			IsPub: field.Pub,
			IsMut: field.Mut,
		})
	}

	// For @model classes: if no field has PrimaryKey, auto-add id: AutoField
	if cls.MacroDecorator != "" {
		hasPK := false
		for _, mf := range cls.ModelFields {
			if mf.PrimaryKey || mf.Kind == types.OrmAuto || mf.Kind == types.OrmBigAuto {
				hasPK = true
				break
			}
		}
		if !hasPK {
			// Auto-add id: AutoField (like Django)
			autoID := types.OrmField{
				Name:       "id",
				Kind:       types.OrmAuto,
				PrimaryKey: true,
			}
			cls.ModelFields = append([]types.OrmField{autoID}, cls.ModelFields...)
			// Also add to Fields for the class type
			cls.Fields = append([]types.Field{{Name: "id", Type: types.Int, IsPub: true}}, cls.Fields...)
		}
	}

	// For @model classes: extract Meta class options
	if cls.MacroDecorator != "" {
		for _, nested := range d.Nested {
			if nested.Name.Name == "Meta" {
				meta := &types.OrmMeta{}
				for _, f := range nested.Fields {
					switch f.Name.Name {
					case "unique_together":
						// [field1, field2] → UniqueConstraints entry
						if f.Type != nil && len(f.Type.Params) > 0 {
							var fields []string
							for _, p := range f.Type.Params {
								if p.Name != "" {
									fields = append(fields, p.Name)
								}
							}
							if len(fields) > 0 {
								meta.UniqueConstraints = append(meta.UniqueConstraints, fields)
							}
						}
					case "indexes":
						// [field1, field2] → Indexes entry
						if f.Type != nil && len(f.Type.Params) > 0 {
							var fields []string
							for _, p := range f.Type.Params {
								if p.Name != "" {
									fields = append(fields, p.Name)
								}
							}
							if len(fields) > 0 {
								meta.Indexes = append(meta.Indexes, fields)
							}
						}
					case "ordering":
						// [-created_at, name] → Ordering list
						if f.Type != nil && len(f.Type.Params) > 0 {
							for _, p := range f.Type.Params {
								if p.Name != "" {
									meta.Ordering = append(meta.Ordering, p.Name)
								}
							}
						}
					case "abstract":
						// abstract: true
						if f.Type != nil && f.Type.Name == "true" {
							meta.Abstract = true
						}
					case "primary_key":
						// primary_key: [order_id, product_id] → CompositePK
						if f.Type != nil && len(f.Type.Params) > 0 {
							for _, p := range f.Type.Params {
								if p.Name != "" {
									meta.CompositePK = append(meta.CompositePK, p.Name)
								}
							}
							}
					}
				}
				cls.Meta = meta
				break
			}
		}
	}

	// If CompositePK is set, remove the auto-inserted id field
	if cls.MacroDecorator != "" && cls.Meta != nil && len(cls.Meta.CompositePK) > 0 {
		var filteredFields []types.OrmField
		for _, mf := range cls.ModelFields {
			if mf.Name == "id" && mf.Kind == types.OrmAuto {
				continue // skip auto-PK — using composite PK instead
			}
			filteredFields = append(filteredFields, mf)
		}
		cls.ModelFields = filteredFields
		var filteredClassFields []types.Field
		for _, f := range cls.Fields {
			if f.Name == "id" {
				continue
			}
			filteredClassFields = append(filteredClassFields, f)
		}
		cls.Fields = filteredClassFields
	}

	// Check nested classes EARLY (before method bodies need them)
	for _, nested := range d.Nested {
		if cls.MacroDecorator != "" && nested.Name.Name == "Meta" {
			continue // Skip Meta class — already processed above
		}
		c.checkClass(nested)
	}

	// Resolve constants
	for _, cd := range d.Constants {
		constType := c.resolveType(cd.Type)

		// Verify the value is a compile-time constant expression
		constVal := c.evalConst(cd.Value)
		if constVal == nil {
			c.add(diagAt("DTE0004", cd.Value.SpanOf(), "class constant must be a compile-time constant expression"))
		}

		// Check the value expression type
		valType := c.typ(cd.Value)

		if !types.Assignable(constType, valType) {
			c.add(diagAt("DTE0004", cd.Value.SpanOf(), fmt.Sprintf("cannot assign %s to constant of type %s", valType, constType)))
		}

		cls.Constants[cd.Name.Name] = &types.ClassConstant{
			Name:  cd.Name.Name,
			Type:  constType,
			Value: cd.Value,
			IsPub: cd.IsPub,
		}
	}

	// Resolve static fields
	for _, sd := range d.StaticFields {
		staticType := c.resolveType(sd.Type)
		valType := c.typ(sd.Value)

		if !types.Assignable(staticType, valType) {
			c.add(diagAt("DTE0004", sd.Value.SpanOf(), fmt.Sprintf("cannot assign %s to static field of type %s", valType, staticType)))
		}

		cls.StaticFields[sd.Name.Name] = &types.ClassStaticField{
			Name:  sd.Name.Name,
			Type:  staticType,
			IsPub: sd.IsPub,
			IsMut: sd.IsMut,
		}
	}

	// Process methods
	for _, method := range d.Methods {
		methodName := method.Name.Name
		isDunder := strings.HasPrefix(methodName, "__") && strings.HasSuffix(methodName, "__")

		// POLICY: All dunders MUST be pub
		if isDunder && !method.Pub {
			c.add(diagAt("DCL0001", method.Span, fmt.Sprintf("dunder method %s must be pub", methodName)))
		}

		// Look for overloads first
		var funcs []*types.Func
		if overloadSet := c.info.Funcs[methodName]; overloadSet != nil {
			for _, cand := range overloadSet.Cands {
				// Only process the function corresponding to this AST declaration
				if cand.Decl == method {
					cand.Type.IsPub = method.Pub
					funcs = append(funcs, cand.Type)
				}
			}
		} else {
			// Fallback to simple lookup (if not overloaded or not yet processed?)
			// Note: checkFunc should have run by now? No, collectClass runs BEFORE checkFunc?
			// Wait, collectClass runs in Phase 1 (type collection). checkFunc runs in Phase 2.
			// So c.info.Funcs might NOT be populated yet!

			// If we are in Phase 1, we need to resolve the function signature manually here?
			// Or we rely on the fact that we are doing this in checkClass (Phase 2)?
			// collectClass is Phase 1. checkClass is Phase 2.
			// This code is in checkClass (based on file context).

			// Let's verify where we are.
			// The function is `checkClass`.

			methodSym := c.scope.Lookup(methodName)
			if methodSym != nil {
				if ft, ok := methodSym.Type.(*types.Func); ok {
					ft.IsPub = method.Pub
					funcs = append(funcs, ft)
				}
			}
		}

		for _, ft := range funcs {
			//POLICY: Validate __close__ signature if present
			if methodName == "__close__" {
				// __close__(self) -> none
				if ft.Ret != types.None {
					c.add(diagAt("DCL0003", method.Span, "__close__ must return none"))
				}
				// After self injection, should have exactly 1 param (self)
				if len(ft.Params) != 1 {
					c.add(diagAt("DCL0003", method.Span, "__close__ must take only self (no other parameters)"))
				}
			}

			//POLICY: Validate __copy__ signature if present
			if methodName == "__copy__" {
				// __copy__(self) -> ClassName
				// Must return the same class type
				if ft.Ret == nil || !types.Equal(ft.Ret, cls) {
					c.add(diagAt("DCL0003", method.Span, "__copy__ must return "+cls.Name))
				}
				// After self injection, should have exactly 1 param (self)
				if len(ft.Params) != 1 {
					c.add(diagAt("DCL0003", method.Span, "__copy__ must take only self (no other parameters)"))
				}
			}

			//POLICY: Validate __repr__ signature if present
			if methodName == "__repr__" {
				// __repr__(self) -> str
				if ft.Ret != types.Str {
					c.add(diagAt("DCL0003", method.Span, "__repr__ must return str"))
				}
				// After self injection, should have exactly 1 param (self)
				if len(ft.Params) != 1 {
					c.add(diagAt("DCL0003", method.Span, "__repr__ must take only self (no other parameters)"))
				}
			}

			//POLICY: Validate arithmetic operator dunders
			if methodName == "__add__" || methodName == "__sub__" || methodName == "__mul__" || methodName == "__div__" {
				// Signature: __op__(self, other: T) -> T
				// Must have exactly 2 params (self, other)
				if len(ft.Params) != 2 {
					c.add(diagAt("DCL0003", method.Span, methodName+" must take exactly 2 parameters (self, other)"))
				}
			}

			//POLICY: Validate comparison operator dunders
			if methodName == "__eq__" || methodName == "__lt__" || methodName == "__le__" ||
				methodName == "__gt__" || methodName == "__ge__" || methodName == "__ne__" {
				// Signature: __cmp__(self, other: T) -> bool
				if ft.Ret != types.Bool {
					c.add(diagAt("DCL0003", method.Span, methodName+" must return bool"))
				}
				if len(ft.Params) != 2 {
					c.add(diagAt("DCL0003", method.Span, methodName+" must take exactly 2 parameters (self, other)"))
				}
			}

			//POLICY: Validate __hash__ signature
			if methodName == "__hash__" {
				// __hash__(self) -> u64
				if ft.Ret != types.U64 {
					c.add(diagAt("DCL0003", method.Span, "__hash__ must return u64"))
				}
				if len(ft.Params) != 1 {
					c.add(diagAt("DCL0003", method.Span, "__hash__ must take only self (no other parameters)"))
				}
			}

			// ═══════════════════════════════════════════════════════════════════════
			// DECORATOR POLICY (DO NOT REMOVE - Design Documentation)
			// ═══════════════════════════════════════════════════════════════════════
			//
			// WHAT: Support for @staticmethod, @classmethod, and @property decorators
			//
			// HOW:
			//   - @staticmethod: No self/cls parameter injection
			//   - @classmethod:  No cls parameter (use class name directly in body)
			//   - @property:     Validate (self) -> T signature, accessed without ()
			//
			// WHY @classmethod has no cls param:
			//   1. Desi doesn't have class-level state yet (no static fields)
			//   2. Current use: Factory methods → class name is sufficient
			//   3. Simpler than Python's approach (no runtime class objects)
			//
			// FUTURE-PROOF:
			//   - When adding class variables: Introduce `cls` parameter then
			//     Example: `cls.total` for accessing `static total: int`
			//   - Backward compatible: Can add `cls` without breaking existing code
			//
			// WHY @property uses function calls:
			//   1. Simple implementation: property access → getter call
			//   2. Performance: Acceptable for most use cases (not hot loops)
			//   3. LLVM can inline trivial properties automatically
			//
			// OPTIMIZATION PATH:
			//   - Current: Function call per access
			//   - Future: Explicit inline hint for compiler
			//   - Advanced: Memoization for expensive properties
			//
			// HOW TO EXTEND:
			//   - New decorators: Add to this section, follow same pattern
			//   - Class state: When adding static fields, inject implicit `cls`
			//   - Property setters: Add @property.setter with (self, value) -> none
			//
			// ═══════════════════════════════════════════════════════════════════════

			// Check decorators
			isStatic := hasDecorator(method, "staticmethod")
			isClassMethod := hasDecorator(method, "classmethod")
			isProperty := hasDecorator(method, "property")
			isAbstract := hasDecorator(method, "abstract")

			// Validate property signature: must be (self) -> T
			if isProperty {
				// Properties must have signature: (self) -> T (no other params)
				if len(method.Params) > 1 || (len(method.Params) == 1 && method.Params[0].Name.Name != "self") {
					c.add(diagAt("DCL0004", method.Span, "@property must have signature (self) -> T"))
				}
			}

			// Validate abstract methods
			if isAbstract {
				// Abstract methods must not be static or class methods (for now)
				if isStatic {
					c.add(diagAt("DCL0005", method.Span, "@abstract cannot be combined with @staticmethod"))
				}
				if isClassMethod {
					c.add(diagAt("DCL0005", method.Span, "@abstract cannot be combined with @classmethod"))
				}
				// Mark class as abstract
				cls.IsAbstract = true
				if cls.AbstractMethods == nil {
					cls.AbstractMethods = make(map[string]bool)
				}
				cls.AbstractMethods[methodName] = true
			}

			// POLICY: Inject implicit self parameter if not present
			// Check if first param is already self (explicit)
			hasSelf := false
			if len(method.Params) > 0 {
				// Check if first param is named "self" or "cls"
				paramName := method.Params[0].Name.Name
				if paramName == "self" || paramName == "cls" {
					hasSelf = true
					// Ensure the type is correct
					if len(ft.Params) > 0 {
						if paramName == "cls" || isClassMethod {
							// For @classmethod, inject Type[ClassName]
							// v0.2.0: proper Type[T] metatype (Python's Type[cls])
							// Current: use the class type directly — works for dispatch
							ft.Params[0] = cls
						} else {
							ft.Params[0] = cls
						}
					}
				} else if len(ft.Params) > 0 && types.Equal(ft.Params[0], cls) {
					// Also check by type
					hasSelf = true
				}
			}

			// Inject self/cls parameter based on decorator
			if !hasSelf && methodName != "__new__" && !isStatic && !isClassMethod {
				// Only regular instance methods get self
				// Static methods and class methods: no parameter
				// Properties: get self (they're instance methods)
				ft.Params = append([]types.T{cls}, ft.Params...)
			}

			// Store method in appropriate map
			if isStatic {
				cls.StaticMethods[methodName] = ft
			} else if isClassMethod {
				cls.ClassMethods[methodName] = ft
			} else if isProperty {
				cls.Properties[methodName] = ft
			} else if isDunder {
				cls.Dunders[methodName] = ft
				if methodName == "__new__" {
					// Check if already added to avoid duplicates from loop
					found := false
					for _, existing := range cls.Constructors {
						if existing == ft {
							found = true
							break
						}
					}
					if !found {
						cls.Constructors = append(cls.Constructors, ft)
					}
				} else if methodName == "__del__" {
					// Validate __del__ signature: zero params (only self), void return
					if len(ft.Params) != 1 {
						c.add(diagAt("DTC0020", method.Name.Span,
							"__del__ must have zero parameters (only self)"))
					}
					// Return type must be void (nil or types.None)
					if ft.Ret != nil && !types.Equal(ft.Ret, types.None) {
						c.add(diagAt("DTC0021", method.Name.Span,
							"__del__ must not have a return type (should be void)"))
					}
				}
			} else {
				cls.Methods[methodName] = ft
			}
		}

		// Check the method body
		c.checkFunc(method)
	}

	// Handle abstract method inheritance and validation
	if cls.Base != nil && cls.Base.IsAbstract {
		// Inherit abstract methods from base class
		if cls.AbstractMethods == nil {
			cls.AbstractMethods = make(map[string]bool)
		}

		// Check each abstract method from base
		for abstractMethod := range cls.Base.AbstractMethods {
			// Check if this class implements the abstract method
			implemented := false

			// Check in regular methods
			if _, ok := cls.Methods[abstractMethod]; ok {
				// Check if it's not still marked abstract in this class by finding the AST node
				for _, m := range d.Methods {
					if m.Name.Name == abstractMethod && !hasDecorator(m, "abstract") {
						implemented = true
						break
					}
				}
			}

			// Check in dunders
			if _, ok := cls.Dunders[abstractMethod]; ok {
				// Check if it's  not still marked abstract
				for _, m := range d.Methods {
					if m.Name.Name == abstractMethod && !hasDecorator(m, "abstract") {
						implemented = true
						break
					}
				}
			}

			// If not implemented, inherit the abstract method requirement
			if !implemented {
				cls.AbstractMethods[abstractMethod] = true
				cls.IsAbstract = true
			}
		}
	}

	// If class has unimplemented abstract methods, report error (unless class itself is abstract)
	if len(cls.AbstractMethods) > 0 && !cls.IsAbstract {
		// If this class doesn't define any new abstract methods but has inherited ones, it must implement them all
		hasOwnAbstractMethods := false
		for _, method := range d.Methods {
			if hasDecorator(method, "abstract") {
				hasOwnAbstractMethods = true
				break
			}
		}

		if !hasOwnAbstractMethods {
			// This is a concrete class with unimplemented abstract methods
			for abstractMethod := range cls.AbstractMethods {
				c.add(diagAt("DCL0006", d.Span, fmt.Sprintf("class %s must implement abstract method '%s' from base class", cls.Name, abstractMethod)))
			}
		}
	}

	// M14: If Display trait is auto-generated but to_str not explicit, add to Dunders
	// so that field access (obj.to_str()) can be resolved correctly
	if _, hasToStr := cls.Dunders["to_str"]; !hasToStr {
		if _, hasMethod := cls.Methods["to_str"]; !hasMethod {
			// Check if Display was auto-generated
			if impls, ok := c.info.Impls[cls.Name]; ok {
				if _, hasDisplay := impls["Display"]; hasDisplay {
					// Add synthetic to_str method type to Dunders
					// Signature: (self) -> str
					cls.Dunders["to_str"] = types.FuncOf([]types.T{cls}, types.Str, false)
				}
			}
		}
	}

}

// isFFICompatible checks if a type is valid for @ffi_struct fields.
// FFI-compatible types: primitives (int, float, bool), cptr, and other @ffi_struct types.
func isFFICompatible(t types.T) bool {
	// Check for primitive numeric and boolean types
	if types.Equal(t, types.Int) || types.Equal(t, types.Float) || types.Equal(t, types.Bool) {
		return true
	}
	// Check for sized integers
	if types.Equal(t, types.I8) || types.Equal(t, types.I16) || types.Equal(t, types.I32) || types.Equal(t, types.I64) {
		return true
	}
	if types.Equal(t, types.U8) || types.Equal(t, types.U16) || types.Equal(t, types.U32) || types.Equal(t, types.U64) {
		return true
	}
	// Check for sized floats
	if types.Equal(t, types.F32) || types.Equal(t, types.F64) {
		return true
	}
	// Check for cptr (raw C pointer)
	if _, ok := t.(*types.CPtr); ok {
		return true
	}
	// Check for other @ffi_struct types
	if st, ok := t.(*types.Struct); ok {
		return st.FFI
	}
	// str, list, dict, set, etc. are NOT FFI-safe
	return false
}

// resolveOrmField checks if a TypeName is an ORM field type (AutoField, CharField, etc.)
// and returns the OrmField descriptor + corresponding Desi runtime type.
// Returns (nil, nil) if the type name is not an ORM field type.
func (c *checker) resolveOrmField(fieldName string, tn *ast.TypeName) (*types.OrmField, types.T) {
	if tn == nil {
		return nil, nil
	}

	var kind types.OrmFieldKind
	var desiType types.T
	recognized := true

	switch tn.Name {
	case "AutoField":
		kind = types.OrmAuto
		desiType = types.Int
	case "BigAutoField":
		kind = types.OrmBigAuto
		desiType = types.Int
	case "IntField":
		kind = types.OrmInt
		desiType = types.Int
	case "BigIntField":
		kind = types.OrmBigInt
		desiType = types.Int
	case "CharField":
		kind = types.OrmChar
		desiType = types.Str
	case "TextField":
		kind = types.OrmText
		desiType = types.Str
	case "BoolField":
		kind = types.OrmBool
		desiType = types.Bool
	case "FloatField":
		kind = types.OrmFloat
		desiType = types.Float
	case "DecimalField":
		kind = types.OrmDecimal
		desiType = types.Str
	case "DateTimeField":
		kind = types.OrmDateTime
		desiType = types.Str
	case "UUIDField":
		kind = types.OrmUUID
		desiType = types.Str
	case "JsonField":
		kind = types.OrmJson
		desiType = types.Str
	case "ForeignKey":
		kind = types.OrmForeignKey
		desiType = types.Int
	case "GeneratedField":
		kind = types.OrmGenerated
		desiType = types.Str // default; overridden by output= kwarg
	default:
		recognized = false
	}

	if !recognized {
		return nil, nil
	}

	of := &types.OrmField{
		Name: fieldName,
		Kind: kind,
	}

	// Set defaults based on field kind
	switch kind {
	case types.OrmAuto, types.OrmBigAuto:
		of.PrimaryKey = true
	case types.OrmChar:
		of.MaxLength = 255 // default VARCHAR length
	}

	// Parse TypeName.Params for field options (bracket or paren positional params)
	// e.g., CharField(100) or CharField[100] → MaxLength=100
	// e.g., ForeignKey(User) or ForeignKey[User, CASCADE] → RefTable, RefColumn, OnDelete
	for i, param := range tn.Params {
		switch kind {
		case types.OrmChar:
			if i == 0 {
				// First param is max_length for CharField
				if v := parseIntFromTypeName(param); v > 0 {
					of.MaxLength = v
				}
			}
		case types.OrmDecimal:
			if i == 0 {
				if v := parseIntFromTypeName(param); v > 0 {
					of.Precision = v
				}
			} else if i == 1 {
				if v := parseIntFromTypeName(param); v > 0 {
					of.Scale = v
				}
			}
		case types.OrmForeignKey:
			if i == 0 {
				// First param: model class name (Django-style)
				// Resolve class → table name and auto-detect PK column
				refTable := param.Name
				refColumn := "id" // default PK column
				for _, node := range c.info.Types {
					if cls, ok := node.(*types.Class); ok && cls.MacroDecorator != "" && cls.Name == param.Name {
						refTable = cls.TableName
						// Auto-resolve PK column from the referenced model's fields
						for _, mf := range cls.ModelFields {
							if mf.PrimaryKey {
								refColumn = mf.Name
								break
							}
						}
						break
					}
				}
				of.RefTable = refTable
				of.RefColumn = refColumn
				of.OnDelete = "CASCADE" // default on_delete
			} else if i == 1 {
				// Second positional param: on_delete (bracket syntax fallback)
				of.OnDelete = param.Name
			}
		case types.OrmInt, types.OrmBigInt:
			if i == 0 {
				// First positional param for IntField is default value
				of.Default = param.Name
				of.HasDefault = true
			}
		case types.OrmGenerated:
			if i == 0 {
				// First positional param is the SQL expression
				of.Expression = param.Name
			}
		}

		// Common options based on param name (bracket syntax flags)
		switch param.Name {
		case "nullable":
			of.Nullable = true
		case "unique":
			of.Unique = true
		case "db_index":
			of.DbIndex = true
		case "primary_key":
			of.PrimaryKey = true
		case "auto_now":
			of.AutoNow = true
		case "auto_now_add":
			of.AutoNowAdd = true
		}
	}

	// Parse TypeName.KwParams for keyword arguments (paren syntax)
	// e.g., CharField(100, unique=true) → unique kwarg
	// e.g., ForeignKey(User, on_delete=CASCADE) → on_delete kwarg
	for _, kw := range tn.KwParams {
		switch kw.Key {
		case "on_delete":
			if kw.Value != nil {
				of.OnDelete = kw.Value.Name
			}
		case "nullable":
			if kw.Value != nil && kw.Value.Name == "true" {
				of.Nullable = true
			}
		case "unique":
			if kw.Value != nil && kw.Value.Name == "true" {
				of.Unique = true
			}
		case "db_index":
			if kw.Value != nil && kw.Value.Name == "true" {
				of.DbIndex = true
			}
		case "primary_key":
			if kw.Value != nil && kw.Value.Name == "true" {
				of.PrimaryKey = true
			}
		case "auto_now":
			if kw.Value != nil && kw.Value.Name == "true" {
				of.AutoNow = true
			}
		case "auto_now_add":
			if kw.Value != nil && kw.Value.Name == "true" {
				of.AutoNowAdd = true
			}
		case "default":
			if kw.Value != nil {
				of.Default = kw.Value.Name
				of.HasDefault = true
			}
		case "max_length":
			if kw.Value != nil {
				if v := parseIntFromTypeName(kw.Value); v > 0 {
					of.MaxLength = v
				}
			}
		case "expression":
			if kw.Value != nil && kind == types.OrmGenerated {
				of.Expression = kw.Value.Name
			}
		case "output":
			// GeneratedField output type: output=CharField(255), output=IntField()
			if kw.Value != nil && kind == types.OrmGenerated {
				switch kw.Value.Name {
				case "CharField":
					of.OutputKind = types.OrmChar
					of.OutputMaxLen = 255 // default
					if len(kw.Value.Params) > 0 {
						if v := parseIntFromTypeName(kw.Value.Params[0]); v > 0 {
							of.OutputMaxLen = v
						}
					}
					desiType = types.Str
				case "IntField":
					of.OutputKind = types.OrmInt
					desiType = types.Int
				case "BoolField":
					of.OutputKind = types.OrmBool
					desiType = types.Bool
				case "FloatField":
					of.OutputKind = types.OrmFloat
					desiType = types.Float
				case "TextField":
					of.OutputKind = types.OrmText
					desiType = types.Str
				case "BigIntField":
					of.OutputKind = types.OrmBigInt
					desiType = types.Int
				}
			}
		case "choices":
			// choices=["draft", "published"] → CHECK constraint
			if kw.Value != nil && len(kw.Value.Params) > 0 {
				for _, p := range kw.Value.Params {
					if p.Name != "" {
						of.Choices = append(of.Choices, p.Name)
					}
				}
			}
		}
	}

	return of, desiType
}

// parseIntFromTypeName tries to parse a TypeName as an integer (e.g., for CharField[100])
func parseIntFromTypeName(tn *ast.TypeName) int {
	if tn == nil {
		return 0
	}
	n := 0
	for _, c := range tn.Name {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}
