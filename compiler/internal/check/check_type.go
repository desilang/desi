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
	// Create Class type
	cls := &types.Class{
		Name:          d.Name.Name,
		TypeParams:    make([]types.TypeParam, 0, len(d.TypeParams)), // Initialize TypeParams slice
		Fields:        nil,                                           // will be populated in Phase 2
		Methods:       make(map[string]*types.Func),
		StaticMethods: make(map[string]*types.Func),
		ClassMethods:  make(map[string]*types.Func),
		Properties:    make(map[string]*types.Func),
		Dunders:       make(map[string]*types.Func),
		Constructors:  nil, // will be populated in checkClass
		Base:          nil, // will be resolved in Phase 2
		IsNested:      false,
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
							// For @classmethod, inject Type[ClassName] (for now, just use cls)
							// TODO: Implement Type[T] wrapper
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
}
