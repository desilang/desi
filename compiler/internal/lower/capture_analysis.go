package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// CollectFreeVars finds variables referenced in expr that are not in scope.
// Returns a deduplicated list of captured variable names in order of first occurrence.
//
// Parameters:
//   - expr: The expression to analyze (typically a lambda body)
//   - scope: Map of variable names that are in scope (params, local lets)
//
// Returns: List of free variable names that need to be captured
func CollectFreeVars(expr ast.Expr, scope map[string]bool) []string {
	seen := make(map[string]bool)
	var captures []string

	collectFreeVarsImpl(expr, scope, seen, &captures)
	return captures
}

// collectFreeVarsImpl recursively walks the AST to find free variables
func collectFreeVarsImpl(expr ast.Expr, scope map[string]bool, seen map[string]bool, captures *[]string) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.Ident:
		// Skip builtins and already-seen identifiers
		if isBuiltin(e.Name) {
			return
		}
		// If not in scope and not already captured, add to captures
		if !scope[e.Name] && !seen[e.Name] {
			seen[e.Name] = true
			*captures = append(*captures, e.Name)
		}

	case *ast.CallExpr:
		// For call expressions, the callee (if an Ident) is a function name, not a captured var.
		// Only collect from args. But if callee is a FieldExpr (method call), collect from X.
		switch callee := e.Callee.(type) {
		case *ast.Ident:
			// Simple function name like inc(x) - don't capture 'inc'
			// (it's a function, not a variable)
		case *ast.FieldExpr:
			// Method call like obj.method() - collect from 'obj' (but not 'method')
			collectFreeVarsImpl(callee.X, scope, seen, captures)
		default:
			// Other callable expressions (e.g., higher-order function returning function)
			collectFreeVarsImpl(e.Callee, scope, seen, captures)
		}
		for _, arg := range e.Args {
			collectFreeVarsImpl(arg, scope, seen, captures)
		}
		for _, an := range e.ArgNodes {
			collectFreeVarsImpl(an.Expr, scope, seen, captures)
		}

	case *ast.BinaryExpr:
		collectFreeVarsImpl(e.Lhs, scope, seen, captures)
		collectFreeVarsImpl(e.Rhs, scope, seen, captures)

	case *ast.UnaryExpr:
		collectFreeVarsImpl(e.X, scope, seen, captures)

	case *ast.FieldExpr:
		collectFreeVarsImpl(e.X, scope, seen, captures)
		// Don't collect field names - they're not free vars

	case *ast.IndexExpr:
		collectFreeVarsImpl(e.X, scope, seen, captures)
		collectFreeVarsImpl(e.Idx, scope, seen, captures)

	case *ast.ListLit:
		for _, elem := range e.Elems {
			collectFreeVarsImpl(elem, scope, seen, captures)
		}

	case *ast.DictLit:
		// DictLit has Keys and Values (not Pairs)
		for _, key := range e.Keys {
			collectFreeVarsImpl(key, scope, seen, captures)
		}
		for _, val := range e.Values {
			collectFreeVarsImpl(val, scope, seen, captures)
		}

	case *ast.TupleLit:
		for _, elem := range e.Elems {
			collectFreeVarsImpl(elem, scope, seen, captures)
		}

	case *ast.LambdaExpr:
		// Create new scope with lambda params
		innerScope := make(map[string]bool)
		for k, v := range scope {
			innerScope[k] = v
		}
		for _, p := range e.Params {
			innerScope[p.Name.Name] = true
		}
		// Collect from lambda body with extended scope
		collectFreeVarsImpl(e.Body, innerScope, seen, captures)

	case *ast.MatchExpr:
		// MatchExpr has Scrutinee and Arms with Pattern/Result
		collectFreeVarsImpl(e.Scrutinee, scope, seen, captures)
		for _, arm := range e.Arms {
			// Arm pattern bindings create new scope
			innerScope := make(map[string]bool)
			for k, v := range scope {
				innerScope[k] = v
			}
			// Add pattern bindings to scope
			addPatternBindings(arm.Pattern, innerScope)
			collectFreeVarsImpl(arm.Result, innerScope, seen, captures)
		}

	case *ast.SliceExpr:
		// SliceExpr has X, I (start), J (stop), K (step)
		collectFreeVarsImpl(e.X, scope, seen, captures)
		collectFreeVarsImpl(e.I, scope, seen, captures)
		collectFreeVarsImpl(e.J, scope, seen, captures)
		collectFreeVarsImpl(e.K, scope, seen, captures)

	case *ast.TryExpr:
		collectFreeVarsImpl(e.X, scope, seen, captures)

	case *ast.FString:
		for _, part := range e.Parts {
			collectFreeVarsImpl(part, scope, seen, captures)
		}

	case *ast.FStringExpr:
		collectFreeVarsImpl(e.X, scope, seen, captures)

	// Literals don't contain free variables
	case *ast.IntLit, *ast.FloatLit, *ast.StrLit, *ast.BoolLit, *ast.NoneLit:
		// Nothing to collect
	}
}

// addPatternBindings adds bindings from a match pattern to scope
func addPatternBindings(pattern ast.Expr, scope map[string]bool) {
	switch p := pattern.(type) {
	case *ast.Ident:
		// Simple binding (but not _ which is wildcard)
		if p.Name != "_" {
			scope[p.Name] = true
		}
	case *ast.CallExpr:
		// Enum variant with payload: Option.Some(x)
		for _, arg := range p.Args {
			addPatternBindings(arg, scope)
		}
	case *ast.TupleLit:
		for _, elem := range p.Elems {
			addPatternBindings(elem, scope)
		}
	}
}

// isBuiltin returns true for known builtin function names
func isBuiltin(name string) bool {
	builtins := map[string]bool{
		"print": true, "len": true, "str": true, "int": true,
		"float": true, "bool": true, "list": true, "dict": true,
		"set": true, "range": true, "enumerate": true, "zip": true,
		"map": true, "filter": true, "reduce": true, "sum": true,
		"min": true, "max": true, "abs": true, "sorted": true,
		"reversed": true, "any": true, "all": true, "type": true,
		"input": true, "open": true, "assert": true, "panic": true,
		"dbg": true, "spawn": true, "rc": true, "arc": true,
		"Option": true, "Result": true, "Some": true, "Nothing": true,
		"Ok": true, "Err": true, "true": true, "false": true,
	}
	return builtins[name]
}
