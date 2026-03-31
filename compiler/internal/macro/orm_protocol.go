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
		"all":            {Name: "all", IsChainable: true},
		"filter":         {Name: "filter", IsChainable: true},
		"exclude":        {Name: "exclude", IsChainable: true},
		"get":            {Name: "get", IsTerminal: true},
		"create":         {Name: "create", IsTerminal: true},
		"update":         {Name: "update", IsTerminal: true},
		"delete":         {Name: "delete", IsTerminal: true},
		"count":          {Name: "count", RetType: types.Int, IsTerminal: true},
		"exists":         {Name: "exists", RetType: types.Bool, IsTerminal: true},
		"first":          {Name: "first", IsTerminal: true},
		"last":           {Name: "last", IsTerminal: true},
		"order_by":       {Name: "order_by", IsChainable: true},
		"values":         {Name: "values", IsChainable: true},
		"distinct":       {Name: "distinct", IsChainable: true},
		"limit":          {Name: "limit", IsChainable: true},
		"offset":         {Name: "offset", IsChainable: true},
		"select_related": {Name: "select_related", IsChainable: true},
		"aggregate":      {Name: "aggregate", IsTerminal: true},
		"annotate":       {Name: "annotate", IsChainable: true},
		"bulk_create":    {Name: "bulk_create", IsTerminal: true},
		"bulk_update":    {Name: "bulk_update", IsTerminal: true},
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
