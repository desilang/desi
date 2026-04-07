package macro

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// ORM protocol fallback — registers @model via Go init() if the loader
// hasn't already loaded it from stdlib/macros/orm.desi.
//
// This ensures backward compatibility during the transition from Go-based
// to Desi-based macro definitions. Once the loader pipeline is fully wired,
// this file can be deleted.

func init() {
	// Only register if not already loaded from .desi file
	if Registry.HasProtocol("model") {
		return
	}
	Registry.Register(ormProtocolFallback())
}

func ormProtocolFallback() *MacroProtocol {
	propName := "objects"

	methods := map[string]MethodSpec{
		// Terminal methods — execute queries
		"all":       {Name: "all", ArgStyle: "none", TerminalFunc: "__qs_all", IsTerminal: true},
		"get":       {Name: "get", ArgStyle: "kwargs_filter", KwargsFunc: "__qs_filter", TerminalFunc: "__qs_first", IsTerminal: true, ReturnsModel: true},
		"first":     {Name: "first", ArgStyle: "none", TerminalFunc: "__qs_first", IsTerminal: true, ReturnsModel: true},
		"last":      {Name: "last", ArgStyle: "none", TerminalFunc: "__qs_last", IsTerminal: true, ReturnsModel: true},
		"count":     {Name: "count", RetType: types.Int, ArgStyle: "none", TerminalFunc: "__qs_count", IsTerminal: true},
		"exists":    {Name: "exists", RetType: types.Bool, ArgStyle: "none", TerminalFunc: "__qs_exists", IsTerminal: true},
		"delete":    {Name: "delete", ArgStyle: "none", TerminalFunc: "__qs_delete", IsTerminal: true},
		"create":    {Name: "create", ArgStyle: "kwargs_set", KwargsFunc: "__qs_set_field", TerminalFunc: "__qs_do_insert", IsTerminal: true},
		"update":    {Name: "update", ArgStyle: "kwargs_set", KwargsFunc: "__qs_update", TerminalFunc: "__qs_row_count", IsTerminal: true},
		"aggregate": {Name: "aggregate", ArgStyle: "none", TerminalFunc: "__qs_fetch", IsTerminal: true},

		// Phase 1 terminal methods
		"latest":        {Name: "latest", ArgStyle: "positional", TerminalFunc: "__qs_latest", IsTerminal: true, ReturnsModel: true},
		"earliest":      {Name: "earliest", ArgStyle: "positional", TerminalFunc: "__qs_earliest", IsTerminal: true, ReturnsModel: true},
		"get_or_create": {Name: "get_or_create", ArgStyle: "kwargs_set", KwargsFunc: "__qs_set_field", TerminalFunc: "__qs_get_or_create", IsTerminal: true},
		"explain":       {Name: "explain", ArgStyle: "none", TerminalFunc: "__qs_explain", IsTerminal: true},

		// Chainable methods — build query, don't execute
		"filter":         {Name: "filter", ArgStyle: "kwargs_filter", KwargsFunc: "__qs_filter", IsChainable: true},
		"exclude":        {Name: "exclude", ArgStyle: "kwargs_filter", KwargsFunc: "__qs_exclude", IsChainable: true},
		"order_by":       {Name: "order_by", ArgStyle: "positional", KwargsFunc: "__qs_order_by", IsChainable: true},
		"limit":          {Name: "limit", ArgStyle: "positional", KwargsFunc: "__qs_limit", IsChainable: true},
		"offset":         {Name: "offset", ArgStyle: "positional", KwargsFunc: "__qs_offset", IsChainable: true},
		"distinct":       {Name: "distinct", ArgStyle: "none", KwargsFunc: "__qs_distinct", IsChainable: true},
		"values":         {Name: "values", ArgStyle: "none", IsChainable: true},
		"select_related": {Name: "select_related", ArgStyle: "positional", IsChainable: true},
		"annotate":       {Name: "annotate", ArgStyle: "none", IsChainable: true},
		"using":          {Name: "using", ArgStyle: "positional", KwargsFunc: "__db_using", IsChainable: true},

		// Phase 1 chainable methods
		"only":         {Name: "only", ArgStyle: "positional", KwargsFunc: "__qs_only", IsChainable: true},
		"defer_fields": {Name: "defer_fields", ArgStyle: "positional", KwargsFunc: "__qs_defer", IsChainable: true},

		// Phase 2 terminal methods
		"update_or_create": {Name: "update_or_create", ArgStyle: "kwargs_set", KwargsFunc: "__qs_set_field", TerminalFunc: "__qs_update_or_create", IsTerminal: true},

		// Bulk operations
		"bulk_create": {Name: "bulk_create", ArgStyle: "none", TerminalFunc: "__qs_bulk_create", IsTerminal: true},
		"bulk_update": {Name: "bulk_update", ArgStyle: "none", TerminalFunc: "__qs_bulk_update", IsTerminal: true},
	}

	runtimeFuncs := map[string]string{
		"reset":     "__qs_reset",
		"filter":    "__qs_filter",
		"exclude":   "__qs_exclude",
		"order_by":  "__qs_order_by",
		"fetch":     "__qs_fetch",
		"create":    "__qs_do_insert",
		"update":    "__qs_update",
		"delete":    "__qs_delete",
		"count":     "__qs_count",
		"exists":    "__qs_exists",
		"limit":     "__qs_limit",
		"offset":    "__qs_offset",
		"set_field": "__qs_set_field",
		"filter_q":  "__qs_filter_q",
		"row_count": "__qs_row_count",
		"distinct":  "__qs_distinct",
		"using":     "__db_using",
		"latest":    "__qs_latest",
		"earliest":  "__qs_earliest",
		"only":      "__qs_only",
		"defer":     "__qs_defer",
		"explain":   "__qs_explain",
		"get_or_create":    "__qs_get_or_create",
		"update_or_create": "__qs_update_or_create",
		"on_conflict":      "__qs_on_conflict",
		"upsert":           "__qs_do_upsert",
		"window":           "__qs_window",
		"cursor_declare":   "__qs_cursor_declare",
		"cursor_fetch":     "__qs_cursor_fetch",
		"cursor_close":     "__qs_cursor_close",
		"json_set":         "__qs_json_set",
		"json_get":         "__qs_json_get",
	}

	return &MacroProtocol{
		Name:   "model",
		Target: ClassMacro,

		OnCollect: func(ctx *MacroContext) error {
			if ctx.Class == nil || ctx.Decorator == nil {
				return nil
			}
			ctx.Class.IsModel = true
			ctx.Class.MacroDecorator = "model"
			if len(ctx.Decorator.Args) > 0 {
				if sl, ok := ctx.Decorator.Args[0].(*ast.StrLit); ok {
					ctx.Class.TableName = sl.Value
				}
			}
			if ctx.Class.TableName == "" && ctx.ClassDecl != nil {
				ctx.Class.TableName = toLower(ctx.ClassDecl.Name.Name) + "s"
			}
			return nil
		},

		OnCheck: func(ctx *MacroContext) error { return nil },
		OnLower: func(ctx *LowerContext) *hir.Func { return nil },

		Properties: map[string]*PropertySpec{
			propName: {
				Name:         propName,
				Methods:      methods,
				RuntimeFuncs: runtimeFuncs,
				RuntimeType:  "c",
			},
		},

		Builtins: map[string]*MacroBuiltin{
			"Q":     {Name: "Q", RetType: types.Str},
			"F":     {Name: "F", RetType: types.Str},
			"Count": {Name: "Count", RetType: types.Str},
			"Sum":   {Name: "Sum", RetType: types.Str},
			"Avg":   {Name: "Avg", RetType: types.Str},
			"Min":   {Name: "Min", RetType: types.Str},
			"Max":   {Name: "Max", RetType: types.Str},
		},

		Operators: map[string]*MacroOp{
			"|": {Op: "|", LhsCheck: makeBuiltinExprCheck("Q"), RetType: types.Str,
				LowerFunc: func(lhs, rhs hir.Value) *hir.Call {
					return &hir.Call{Fn: "__q_or", Args: []hir.Value{lhs, rhs}, Type: "ptr"}
				}},
			"&": {Op: "&", LhsCheck: makeBuiltinExprCheck("Q"), RetType: types.Str,
				LowerFunc: func(lhs, rhs hir.Value) *hir.Call {
					return &hir.Call{Fn: "__q_and", Args: []hir.Value{lhs, rhs}, Type: "ptr"}
				}},
			"+": {Op: "+", LhsCheck: makeBuiltinExprCheck("F"), RetType: types.Str,
				LowerFunc: func(lhs, rhs hir.Value) *hir.Call {
					return &hir.Call{Fn: "__f_add", Args: []hir.Value{lhs, rhs}, Type: "ptr"}
				}},
			"-": {Op: "-", LhsCheck: makeBuiltinExprCheck("F"), RetType: types.Str,
				LowerFunc: func(lhs, rhs hir.Value) *hir.Call {
					return &hir.Call{Fn: "__f_sub", Args: []hir.Value{lhs, rhs}, Type: "ptr"}
				}},
			"*": {Op: "*", LhsCheck: makeBuiltinExprCheck("F"), RetType: types.Str,
				LowerFunc: func(lhs, rhs hir.Value) *hir.Call {
					return &hir.Call{Fn: "__f_mul", Args: []hir.Value{lhs, rhs}, Type: "ptr"}
				}},
			"/": {Op: "/", LhsCheck: makeBuiltinExprCheck("F"), RetType: types.Str,
				LowerFunc: func(lhs, rhs hir.Value) *hir.Call {
					return &hir.Call{Fn: "__f_div", Args: []hir.Value{lhs, rhs}, Type: "ptr"}
				}},
		},

		UnaryOperators: map[string]*MacroUnaryOp{
			"~": {Op: "~", Check: makeBuiltinExprCheck("Q"), RetType: types.Str,
				LowerFunc: func(operand hir.Value) *hir.Call {
					return &hir.Call{Fn: "__q_not", Args: []hir.Value{operand}, Type: "ptr"}
				}},
		},

		Validation: &ValidationRules{
			RequireFields:       true,
			AutoPK:              true,
			ForbiddenFieldNames: []string{"id"},
		},
	}
}

// ============================================================
// Generic Expression Detection (used by checker and lowerer)
// ============================================================

// IsQExpression checks if an AST expression is a Q expression.
// Delegates to the generic isBuiltinExpression with builtin name "Q".
func IsQExpression(expr ast.Expr, info interface{}) bool {
	return isBuiltinExpression(expr, "Q")
}

// IsFExpression checks if an AST expression involves F() calls.
func IsFExpression(expr ast.Expr, info interface{}) bool {
	return isBuiltinExpression(expr, "F")
}

// IsFCall checks if an expression is a direct F() call.
func IsFCall(expr ast.Expr) bool {
	return isBuiltinExpression(expr, "F")
}

// toLower converts ASCII string to lowercase.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
