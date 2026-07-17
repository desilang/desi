package check

import (
	"reflect"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// runEscapeAnalysis is the entry point for the escape analysis pass.
// It iterates over all declarations in the module and analyzes function bodies.
func runEscapeAnalysis(mod *ast.Module, info *Info) {
	if mod == nil || info == nil {
		return
	}
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			if dd.Body != nil {
				analyzeFuncEscape(dd, info)
			}
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				if m.Body != nil {
					analyzeFuncEscape(m, info)
				}
			}
		case *ast.ImplDecl:
			for _, m := range dd.Methods {
				if m.Body != nil {
					analyzeFuncEscape(m, info)
				}
			}
		}
	}
}

type escapeVisitor struct {
	info       *Info
	candidates map[*Symbol]*ast.Ident // Maps candidate Symbol -> its declaring Ident node
	tracked    map[*Symbol]bool       // Alias graph nodes that are not candidates (loop vars, match bindings)
	escapes    map[*Symbol]bool       // Track if a symbol escapes
	deps       map[*Symbol][]*Symbol  // Dependency edges: if B escapes, A in deps[B] escapes
}

func analyzeFuncEscape(fd *ast.FuncDecl, info *Info) {
	v := &escapeVisitor{
		info:       info,
		candidates: make(map[*Symbol]*ast.Ident),
		tracked:    make(map[*Symbol]bool),
		escapes:    make(map[*Symbol]bool),
		deps:       make(map[*Symbol][]*Symbol),
	}

	// Step 1: Collect candidates (local variables with heap-allocated types declared via let)
	inspect(fd.Body, func(n ast.Node) bool {
		if let, ok := n.(*ast.LetStmt); ok {
			// Handle simple let name
			if let.Name.Name != "" {
				if sym := info.Idents[&let.Name]; sym != nil {
					if isHeapAllocatedType(sym.Type) {
						v.candidates[sym] = &let.Name
					}
				}
			}
			// Handle destructuring pattern
			for i := range let.Pattern {
				ident := &let.Pattern[i]
				if sym := info.Idents[ident]; sym != nil {
					if isHeapAllocatedType(sym.Type) {
						v.candidates[sym] = ident
					}
				}
			}
		}
		return true
	})

	if len(v.candidates) == 0 {
		return
	}

	// Step 2: Build constraints by visiting all statements and expressions
	v.buildConstraints(fd.Body)

	// Step 3: Propagate escape status backward along dependency edges.
	// Iterate over all graph nodes (candidates AND tracked aliases like loop
	// vars) — an escaping alias must pull the value it aliases along.
	changed := true
	for changed {
		changed = false
		for sym, dList := range v.deps {
			if v.escapes[sym] {
				// If sym escapes, all variables flowing into sym also escape
				for _, dep := range dList {
					if !v.escapes[dep] {
						v.escapes[dep] = true
						changed = true
					}
				}
			}
		}
	}

	// Step 4: Record non-escaping variables in check.Info.NonEscaping
	for sym, ident := range v.candidates {
		if !v.escapes[sym] {
			info.NonEscaping[ident] = true
		}
	}
}

func hasDestructor(c *types.Class) bool {
	if c == nil {
		return false
	}
	if _, exists := c.Dunders["__del__"]; exists {
		return true
	}
	if c.Base != nil {
		return hasDestructor(c.Base)
	}
	return false
}

// isHeapAllocatedType returns true if the type is list, dict, set, tuple, or class.
func isHeapAllocatedType(t types.T) bool {
	if t == nil {
		return false
	}
	if ta, ok := t.(*types.TypeAlias); ok {
		t = ta.Target
	}
	switch tt := t.(type) {
	case *types.List, *types.Dict, *types.Set, *types.Tuple:
		return true
	case *types.Class:
		return !hasDestructor(tt)
	case *types.Generic:
		if cls, ok := tt.Base.(*types.Class); ok {
			return !hasDestructor(cls)
		}
	}
	return false
}

func (v *escapeVisitor) buildConstraints(node ast.Node) {
	inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.LetStmt:
			// Find all declared candidate symbols
			var declared []*Symbol
			if x.Name.Name != "" {
				if sym := v.info.Idents[&x.Name]; sym != nil {
					if _, isCand := v.candidates[sym]; isCand {
						declared = append(declared, sym)
					}
				}
			}
			for i := range x.Pattern {
				if sym := v.info.Idents[&x.Pattern[i]]; sym != nil {
					if _, isCand := v.candidates[sym]; isCand {
						declared = append(declared, sym)
					}
				}
			}

			// Trace references in value expression
			if x.Value != nil {
				refs := v.collectRefs(x.Value)
				for _, decSym := range declared {
					for _, refSym := range refs {
						// refSym flows into decSym (refSym -> decSym)
						v.deps[decSym] = append(v.deps[decSym], refSym)
					}
				}
				// Also inspect inside value to handle calls/lambdas
				v.buildConstraints(x.Value)
			}
			return false // Already inspected value

		case *ast.AssignStmt:
			// Trace RHS references flowing into LHS targets
			var declared []*Symbol
			var lhsIsGlobalOrEscaping bool

			for i := range x.LHS {
				lhsExpr := x.LHS[i]
				if id, ok := lhsExpr.(*ast.Ident); ok {
					if sym := v.info.Idents[id]; sym != nil {
						if _, isCand := v.candidates[sym]; isCand {
							declared = append(declared, sym)
						} else {
							lhsIsGlobalOrEscaping = true
						}
					} else {
						lhsIsGlobalOrEscaping = true
					}
				} else {
					lhsIsGlobalOrEscaping = true
				}
			}

			for i := range x.RHS {
				rhsExpr := x.RHS[i]
				refs := v.collectRefs(rhsExpr)
				if lhsIsGlobalOrEscaping {
					// Stored in a global, property, or index -> escapes
					for _, refSym := range refs {
						v.escapes[refSym] = true
					}
				} else {
					for _, decSym := range declared {
						for _, refSym := range refs {
							v.deps[decSym] = append(v.deps[decSym], refSym)
						}
					}
				}
				v.buildConstraints(rhsExpr)
			}
			return false

		case *ast.ReturnStmt:
			if x.Value != nil {
				for _, refSym := range v.collectRefs(x.Value) {
					v.escapes[refSym] = true
				}
				v.buildConstraints(x.Value)
			}
			return false

		case *ast.CallExpr:
			// Analyze escaping behavior of call arguments
			v.analyzeCallEscape(x)
			return true

		case *ast.ForStmt:
			// Loop variables alias the iterable's interior. Track them as
			// graph nodes: if a loop var escapes (stored, returned, passed
			// to a retaining call), the iterable must escape too — otherwise
			// the iterable gets arena-allocated while an interior pointer
			// outlives the function.
			iterRefs := v.collectRefs(x.Iter)
			bindTarget := func(id *ast.Ident) {
				if id == nil {
					return
				}
				if sym := v.info.Idents[id]; sym != nil {
					v.tracked[sym] = true
					v.deps[sym] = append(v.deps[sym], iterRefs...)
				}
			}
			for i := range x.Targets {
				bindTarget(x.Targets[i].Name)
			}
			if id, ok := x.Target.(*ast.Ident); ok {
				bindTarget(id)
			} else if tup, ok := x.Target.(*ast.TupleLit); ok {
				for _, el := range tup.Elems {
					if id, ok := el.(*ast.Ident); ok {
						bindTarget(id)
					}
				}
			}
			return true

		case *ast.MatchExpr:
			// Arm pattern bindings (Some(x), Circle(r), catch-all x) alias
			// the scrutinee's payload — an escaping binding must pull the
			// scrutinee along.
			scrRefs := v.collectRefs(x.Scrutinee)
			for _, arm := range x.Arms {
				var bindIdents []*ast.Ident
				switch p := arm.Pattern.(type) {
				case *ast.CallExpr: // variant with payload bindings
					for _, a := range p.Args {
						if id, ok := a.(*ast.Ident); ok {
							bindIdents = append(bindIdents, id)
						}
					}
				case *ast.Ident: // catch-all binding aliases the whole value
					bindIdents = append(bindIdents, p)
				}
				for _, id := range bindIdents {
					if sym := v.info.Idents[id]; sym != nil {
						v.tracked[sym] = true
						v.deps[sym] = append(v.deps[sym], scrRefs...)
					}
				}
			}
			return true

		case *ast.LambdaExpr:
			// Any local candidate (or tracked alias) captured inside a
			// lambda escapes — the closure may outlive the function.
			inspect(x.Body, func(subNode ast.Node) bool {
				if id, ok := subNode.(*ast.Ident); ok {
					if sym := v.info.Idents[id]; sym != nil {
						_, isCand := v.candidates[sym]
						if isCand || v.tracked[sym] {
							v.escapes[sym] = true
						}
					}
				}
				return true
			})
			return false
		}
		return true
	})
}

func (v *escapeVisitor) collectRefs(expr ast.Expr) []*Symbol {
	var refs []*Symbol
	inspect(expr, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if sym := v.info.Idents[id]; sym != nil {
				_, isCand := v.candidates[sym]
				if isCand || v.tracked[sym] {
					refs = append(refs, sym)
				}
			}
		}
		return true
	})
	return refs
}

func (v *escapeVisitor) analyzeCallEscape(call *ast.CallExpr) {
	// 1. Determine if this is a method call or regular function call
	if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		// Method call: receiver.method(args)
		receiverRefs := v.collectRefs(fe.X)
		if len(receiverRefs) > 0 {
			// Receiver is a local candidate. Storing args inside the receiver:
			// e.g. list.append(item) -> item flows into list (item -> list)
			for _, recSym := range receiverRefs {
				for _, arg := range call.Args {
					for _, argSym := range v.collectRefs(arg) {
						v.deps[recSym] = append(v.deps[recSym], argSym)
					}
				}
			}
			return
		}
	}

	// 2. Regular function call or method call on non-candidate
	// Is it a safe built-in?
	calleeName := ""
	if id, ok := call.Callee.(*ast.Ident); ok {
		calleeName = id.Name
	} else if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		calleeName = fe.Name.Name
	}

	if isSafeBuiltin(calleeName) {
		return
	}

	// Any non-safe call site causes its arguments to escape
	for _, arg := range call.Args {
		for _, sym := range v.collectRefs(arg) {
			v.escapes[sym] = true
		}
	}
}

func isSafeBuiltin(name string) bool {
	switch name {
	case "print", "len", "ord", "chr", "range", "type", "str", "int", "float", "bool", "dbg", "dumps":
		return true
	}
	return false
}

// inspect is a complete AST depth-first traversal helper. It walks the node
// graph generically via reflection instead of enumerating node types: any
// struct field, slice element, or nested value struct that contains an
// ast.Node is visited. An escape analysis with an incomplete traversal is
// unsound — a reference inside a skipped construct (ternary, comprehension,
// try-expr, …) would silently not create an escape edge, and the value
// would be arena-allocated while an alias outlives the function.
func inspect(node ast.Node, fn func(ast.Node) bool) {
	if node == nil {
		return
	}
	rv := reflect.ValueOf(node)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return
	}
	if !fn(node) {
		return
	}
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	walkChildren(rv, fn)
}

// walkChildren visits every field/element of a struct or slice value,
// dispatching ast.Node values back through inspect.
func walkChildren(rv reflect.Value, fn func(ast.Node) bool) {
	switch rv.Kind() {
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			visitChild(rv.Field(i), fn)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			visitChild(rv.Index(i), fn)
		}
	}
}

// visitChild inspects a single reflect value: ast.Node values recurse
// through inspect (so fn fires on them); bare value structs and slices
// (MatchArm, CompClause, ForTarget, …) are walked through for the nodes
// they contain.
func visitChild(fv reflect.Value, fn func(ast.Node) bool) {
	if !fv.IsValid() || !fv.CanInterface() {
		return
	}
	switch fv.Kind() {
	case reflect.Interface, reflect.Ptr:
		if fv.IsNil() {
			return
		}
		if n, ok := fv.Interface().(ast.Node); ok {
			inspect(n, fn)
			return
		}
		if fv.Kind() == reflect.Ptr {
			walkChildren(fv.Elem(), fn)
		} else {
			visitChild(fv.Elem(), fn)
		}
	case reflect.Struct:
		// Value-embedded nodes (e.g. LetStmt.Name is an ast.Ident value):
		// use the address so identity matches the checker's Idents map keys.
		if fv.CanAddr() {
			if n, ok := fv.Addr().Interface().(ast.Node); ok {
				inspect(n, fn)
				return
			}
		}
		walkChildren(fv, fn)
	case reflect.Slice, reflect.Array:
		for i := 0; i < fv.Len(); i++ {
			visitChild(fv.Index(i), fn)
		}
	}
}
