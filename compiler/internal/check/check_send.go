package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// checkTaskGroupRunSend checks that all captured variables in a lambda
// passed to TaskGroup.run() implement the Send trait (are safe to transfer
// across thread boundaries).
//
// Called when type-checking tg.run(lambda) where lambda has captures.
func (c *checker) checkTaskGroupRunSend(call *ast.CallExpr, arg ast.Expr) {
	// Only check LambdaExpr arguments
	lambda, ok := arg.(*ast.LambdaExpr)
	if !ok {
		return // Named functions are always Send
	}

	// Get captured variables by walking the lambda body
	// Captures are identifiers that reference outer scope variables
	captures := c.collectLambdaCaptures(lambda)

	for _, cap := range captures {
		// Look up the type of the captured variable
		varType := c.lookupVarType(cap.Name)
		if varType == nil {
			continue // Unknown type, skip
		}

		// Check if type is Send
		if !types.IsSend(varType) {
			c.add(diagAt("DSE0010", cap.Span,
				"captured variable '"+cap.Name+"' is not Send - cannot use in tg.run()"+
					" (type "+varType.String()+" cannot be safely transferred across threads)"))
		}
	}
}

// collectLambdaCaptures returns identifiers in the lambda body that reference
// variables from outer scopes (captures).
func (c *checker) collectLambdaCaptures(lambda *ast.LambdaExpr) []*ast.Ident {
	// Build set of parameter names
	paramNames := make(map[string]bool)
	for _, p := range lambda.Params {
		paramNames[p.Name.Name] = true
	}

	// Walk the body and collect identifiers not in params
	var captures []*ast.Ident
	c.walkExprForCaptures(lambda.Body, paramNames, &captures)
	return captures
}

// walkExprForCaptures recursively walks an expression collecting identifiers
// that are not in the local scope (captures).
func (c *checker) walkExprForCaptures(expr ast.Expr, locals map[string]bool, captures *[]*ast.Ident) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.Ident:
		// Check if this identifier is a captured variable
		if !locals[e.Name] && !isBuiltinName(e.Name) {
			// Check if it's a variable (not a function name)
			if sym := c.scope.Lookup(e.Name); sym != nil {
				if sym.Kind == SymVar || sym.Kind == SymParam {
					*captures = append(*captures, e)
				}
			}
		}

	case *ast.CallExpr:
		c.walkExprForCaptures(e.Callee, locals, captures)
		for _, arg := range e.Args {
			c.walkExprForCaptures(arg, locals, captures)
		}

	case *ast.BinaryExpr:
		c.walkExprForCaptures(e.Lhs, locals, captures)
		c.walkExprForCaptures(e.Rhs, locals, captures)

	case *ast.UnaryExpr:
		c.walkExprForCaptures(e.X, locals, captures)

	case *ast.FieldExpr:
		c.walkExprForCaptures(e.X, locals, captures)

	case *ast.IndexExpr:
		c.walkExprForCaptures(e.X, locals, captures)
		c.walkExprForCaptures(e.Idx, locals, captures)

	case *ast.ListLit:
		for _, elem := range e.Elems {
			c.walkExprForCaptures(elem, locals, captures)
		}

	case *ast.TupleLit:
		for _, elem := range e.Elems {
			c.walkExprForCaptures(elem, locals, captures)
		}
	}
}

// lookupVarType returns the type of a variable by name.
func (c *checker) lookupVarType(name string) types.T {
	if sym := c.scope.Lookup(name); sym != nil {
		return sym.Type
	}
	return nil
}

// isBuiltinName returns true for built-in names like print, len, etc.
func isBuiltinName(name string) bool {
	builtins := map[string]bool{
		"print": true, "len": true, "range": true, "str": true,
		"int": true, "float": true, "bool": true, "type": true,
		"list": true, "dict": true, "set": true, "tuple": true,
		"sum": true, "min": true, "max": true, "abs": true,
		"enumerate": true, "zip": true, "map": true, "filter": true,
		"reversed": true, "sorted": true, "any": true, "all": true,
		"channel_new": true, "Option": true, "Result": true,
	}
	return builtins[name]
}
