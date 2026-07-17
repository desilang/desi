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
	escapes    map[*Symbol]bool       // Track if a symbol escapes
	deps       map[*Symbol][]*Symbol  // Dependency edges: if B escapes, A in deps[B] escapes
}

func analyzeFuncEscape(fd *ast.FuncDecl, info *Info) {
	v := &escapeVisitor{
		info:       info,
		candidates: make(map[*Symbol]*ast.Ident),
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

	// Step 3: Propagate escape status backward along dependency edges
	changed := true
	for changed {
		changed = false
		for sym := range v.candidates {
			if v.escapes[sym] {
				// If sym escapes, all variables depending on sym also escape
				for _, dep := range v.deps[sym] {
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

		case *ast.LambdaExpr:
			// Any local candidate captured inside a lambda escapes
			inspect(x.Body, func(subNode ast.Node) bool {
				if id, ok := subNode.(*ast.Ident); ok {
					if sym := v.info.Idents[id]; sym != nil {
						if _, isCand := v.candidates[sym]; isCand {
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
				if _, isCand := v.candidates[sym]; isCand {
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

// inspect is a simple, complete AST depth-first traversal helper.
func inspect(node ast.Node, fn func(ast.Node) bool) {
	if node == nil {
		return
	}
	val := reflect.ValueOf(node)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return
	}
	if !fn(node) {
		return
	}
	switch x := node.(type) {
	case *ast.Block:
		for _, s := range x.Stmts {
			inspect(s, fn)
		}
	case *ast.LetStmt:
		inspect(x.Value, fn)
	case *ast.AssignStmt:
		for _, lhs := range x.LHS {
			inspect(lhs, fn)
		}
		for _, rhs := range x.RHS {
			inspect(rhs, fn)
		}
	case *ast.ExprStmt:
		inspect(x.Expr, fn)
	case *ast.ReturnStmt:
		inspect(x.Value, fn)
	case *ast.IfStmt:
		inspect(x.Cond, fn)
		inspect(x.Then, fn)
		for _, elif := range x.Elifs {
			inspect(elif.Cond, fn)
			inspect(elif.Body, fn)
		}
		inspect(x.Else, fn)
	case *ast.WhileStmt:
		inspect(x.Cond, fn)
		inspect(x.Body, fn)
	case *ast.ForStmt:
		inspect(x.Iter, fn)
		inspect(x.Body, fn)
	case *ast.MatchExpr:
		inspect(x.Scrutinee, fn)
		for _, arm := range x.Arms {
			inspect(arm.Result, fn)
		}
	case *ast.BinaryExpr:
		inspect(x.Lhs, fn)
		inspect(x.Rhs, fn)
	case *ast.UnaryExpr:
		inspect(x.X, fn)
	case *ast.CallExpr:
		inspect(x.Callee, fn)
		for _, arg := range x.Args {
			inspect(arg, fn)
		}
	case *ast.IndexExpr:
		inspect(x.X, fn)
		inspect(x.Idx, fn)
	case *ast.FieldExpr:
		inspect(x.X, fn)
	case *ast.TupleLit:
		for _, el := range x.Elems {
			inspect(el, fn)
		}
	case *ast.ListLit:
		for _, el := range x.Elems {
			inspect(el, fn)
		}
	case *ast.DictLit:
		for _, k := range x.Keys {
			inspect(k, fn)
		}
		for _, v := range x.Values {
			inspect(v, fn)
		}
	case *ast.CastExpr:
		inspect(x.X, fn)
	case *ast.LambdaExpr:
		inspect(x.Body, fn)
	case *ast.TryStmt:
		inspect(x.Body, fn)
		inspect(x.Except, fn)
		inspect(x.Finally, fn)
	case *ast.UnsafeBlock:
		inspect(x.Body, fn)
	case *ast.UsingStmt:
		inspect(x.Bind, fn)
		inspect(x.Init, fn)
		inspect(x.Body, fn)
	case *ast.DeferStmt:
		inspect(x.Call, fn)
	case *ast.FString:
		for _, part := range x.Parts {
			inspect(part, fn)
		}
	case *ast.FStringExpr:
		inspect(x.X, fn)
	}
}
