package macro

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// LoadMacrosFromModule scans a parsed AST module for @macro class declarations
// and registers each one as a MacroProtocol in the global registry.
//
// This is called BEFORE type checking so that macros are available for
// decorator resolution on user classes.
//
// Returns the number of macros loaded and any errors.
func LoadMacrosFromModule(mod *ast.Module) (int, []error) {
	var errs []error
	loaded := 0

	for _, decl := range mod.Decls {
		cd, ok := decl.(*ast.ClassDecl)
		if !ok {
			continue
		}

		// Check for @macro decorator
		macroInfo := getMacroDecoratorInfo(cd)
		if macroInfo == nil {
			continue
		}

		// Parse the class body as macro configuration data
		proto, err := parseMacroClass(cd, macroInfo)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Skip if already registered (e.g., by Go init() fallback)
		if Registry.HasProtocol(proto.Name) {
			continue
		}

		// Register in the global registry
		Registry.Register(proto)
		loaded++
	}

	return loaded, errs
}

// macroDecInfo holds parsed info from @macro(target="class")
type macroDecInfo struct {
	Target string // "class", "func", "field", "struct"
}

// getMacroDecoratorInfo checks if a class has @macro(...) decorator and extracts target.
func getMacroDecoratorInfo(cd *ast.ClassDecl) *macroDecInfo {
	for _, dec := range cd.Decorators {
		if dec.Name.Name != "macro" {
			continue
		}

		info := &macroDecInfo{Target: "class"} // default

		// Check target kwarg: @macro(target="class")
		if targetExpr, ok := dec.KwArgs["target"]; ok {
			if sl, ok := targetExpr.(*ast.StrLit); ok {
				info.Target = sl.Value
			}
		}

		// Also check positional arg: @macro("class")
		if len(dec.Args) > 0 {
			if sl, ok := dec.Args[0].(*ast.StrLit); ok {
				if sl.Value == "class" || sl.Value == "func" || sl.Value == "field" || sl.Value == "struct" {
					info.Target = sl.Value
				}
			}
		}

		return info
	}
	return nil
}

// parseMacroClass extracts macro protocol configuration from a class declaration.
// It reads class constants as configuration: property_name, chainable, terminal, etc.
func parseMacroClass(cd *ast.ClassDecl, info *macroDecInfo) (*MacroProtocol, error) {
	macroName := cd.Name.Name

	// Don't allow macros to be named the same as built-in decorators
	// (Registry.Register also checks, but catch it early with a better error)
	if IsBuiltinDecorator(macroName) && macroName != "macro" {
		return nil, fmt.Errorf("macro definition '%s' conflicts with built-in decorator", macroName)
	}

	// Extract all constant values from the class body
	consts := extractConstants(cd)

	// Parse target type
	target := ClassMacro
	switch info.Target {
	case "func":
		target = FuncMacro
	case "field":
		target = FieldMacro
	case "struct":
		target = StructMacro
	}

	proto := &MacroProtocol{
		Name:           macroName,
		Target:         target,
		Properties:     make(map[string]*PropertySpec),
		Builtins:       make(map[string]*MacroBuiltin),
		Operators:      make(map[string]*MacroOp),
		UnaryOperators: make(map[string]*MacroUnaryOp),
	}

	// --- property_name ---
	propName := extractStr(consts, "property_name")
	if propName == "" {
		propName = macroName + "_property" // default
	}

	// --- chainable / terminal methods ---
	chainableMethods := extractStrList(consts, "chainable")
	terminalMethods := extractStrList(consts, "terminal")

	methods := make(map[string]MethodSpec)
	for _, m := range chainableMethods {
		methods[m] = MethodSpec{Name: m, IsChainable: true}
	}
	for _, m := range terminalMethods {
		methods[m] = MethodSpec{Name: m, IsTerminal: true}
	}

	// --- runtime mapping ---
	runtimeFuncs := extractStrDict(consts, "runtime")

	// --- runtime_type ---
	runtimeType := extractStr(consts, "runtime_type")
	if runtimeType == "" {
		runtimeType = "c" // default: C runtime functions
	}

	prop := &PropertySpec{
		Name:         propName,
		Methods:      methods,
		RuntimeFuncs: runtimeFuncs,
		RuntimeType:  runtimeType,
	}
	proto.Properties[propName] = prop

	// --- builtins ---
	builtinsMap := extractStrDict(consts, "builtins")
	for name, retTypeName := range builtinsMap {
		retType := resolveTypeName(retTypeName)
		proto.Builtins[name] = &MacroBuiltin{
			Name:    name,
			RetType: retType,
		}
	}

	// --- operators ---
	operatorsMap := extractStrDict(consts, "operators")
	for pattern, cFunc := range operatorsMap {
		parseAndRegisterOperator(proto, pattern, cFunc)
	}

	// --- validation rules ---
	requireFields := extractBool(consts, "require_fields")
	autoPK := extractBool(consts, "auto_pk")
	forbiddenNames := extractStrList(consts, "forbidden_field_names")

	if requireFields || autoPK || len(forbiddenNames) > 0 {
		proto.Validation = &ValidationRules{
			RequireFields:       requireFields,
			AutoPK:              autoPK,
			ForbiddenFieldNames: forbiddenNames,
		}
	}

	// --- OnCollect callback (generic) ---
	proto.OnCollect = func(ctx *MacroContext) error {
		if ctx.Class == nil || ctx.Decorator == nil {
			return nil
		}
		ctx.Class.MacroDecorator = macroName
		// For ORM-compatible macros, also set IsModel for backward compat
		ctx.Class.IsModel = true

		// Extract table name from decorator args: @model("tablename")
		if len(ctx.Decorator.Args) > 0 {
			if sl, ok := ctx.Decorator.Args[0].(*ast.StrLit); ok {
				ctx.Class.TableName = sl.Value
			}
		}
		if ctx.Class.TableName == "" && ctx.ClassDecl != nil {
			ctx.Class.TableName = strings.ToLower(ctx.ClassDecl.Name.Name) + "s"
		}

		// Extract db connections from decorator kwarg: @model(db=["analytics", "replica"])
		if dbExpr, ok := ctx.Decorator.KwArgs["db"]; ok {
			if ll, ok := dbExpr.(*ast.ListLit); ok {
				for _, elem := range ll.Elems {
					if sl, ok := elem.(*ast.StrLit); ok {
						ctx.Class.ModelDB = append(ctx.Class.ModelDB, sl.Value)
					}
				}
			} else if sl, ok := dbExpr.(*ast.StrLit); ok {
				// Also accept single string: @model(db="analytics")
				ctx.Class.ModelDB = append(ctx.Class.ModelDB, sl.Value)
			}
		}

		return nil
	}

	// --- OnCheck sentinel ---
	proto.OnCheck = func(ctx *MacroContext) error { return nil }

	// --- OnLower sentinel ---
	proto.OnLower = func(ctx *LowerContext) *hir.Func { return nil }

	return proto, nil
}

// ============================================================
// AST Value Extraction Helpers
// ============================================================

// constMap holds extracted constant name→value pairs from a class body
type constMap map[string]ast.Expr

// extractConstants reads all ClassConstDecl from a class body into a map.
func extractConstants(cd *ast.ClassDecl) constMap {
	m := make(constMap)
	for _, c := range cd.Constants {
		m[c.Name.Name] = c.Value
	}
	return m
}

// extractStr gets a string constant value.
func extractStr(consts constMap, name string) string {
	expr, ok := consts[name]
	if !ok {
		return ""
	}
	if sl, ok := expr.(*ast.StrLit); ok {
		return sl.Value
	}
	return ""
}

// extractBool gets a boolean constant value.
func extractBool(consts constMap, name string) bool {
	expr, ok := consts[name]
	if !ok {
		return false
	}
	if bl, ok := expr.(*ast.BoolLit); ok {
		return bl.Value
	}
	return false
}

// extractStrList gets a list[str] constant value.
func extractStrList(consts constMap, name string) []string {
	expr, ok := consts[name]
	if !ok {
		return nil
	}
	ll, ok := expr.(*ast.ListLit)
	if !ok {
		return nil
	}
	var result []string
	for _, elem := range ll.Elems {
		if sl, ok := elem.(*ast.StrLit); ok {
			result = append(result, sl.Value)
		}
	}
	return result
}

// extractStrDict gets a dict[str, str] constant value.
func extractStrDict(consts constMap, name string) map[string]string {
	expr, ok := consts[name]
	if !ok {
		return nil
	}
	dl, ok := expr.(*ast.DictLit)
	if !ok {
		return nil
	}
	result := make(map[string]string)
	for i, key := range dl.Keys {
		if i >= len(dl.Values) {
			break
		}
		keyStr, ok1 := key.(*ast.StrLit)
		valStr, ok2 := dl.Values[i].(*ast.StrLit)
		if ok1 && ok2 {
			result[keyStr.Value] = valStr.Value
		}
	}
	return result
}

// ============================================================
// Operator Pattern Parsing
// ============================================================

// parseAndRegisterOperator parses an operator pattern like "Q|Q" or "~Q"
// and registers it in the protocol.
func parseAndRegisterOperator(proto *MacroProtocol, pattern, cFunc string) {
	pattern = strings.TrimSpace(pattern)
	cFunc = strings.TrimSpace(cFunc)

	// Unary: "~Q" → prefix operator + builtin name
	if len(pattern) >= 2 && (pattern[0] == '~' || pattern[0] == '!' || pattern[0] == '-') {
		op := string(pattern[0])
		builtinName := pattern[1:]
		proto.UnaryOperators[op] = &MacroUnaryOp{
			Op:      op,
			Check:   makeBuiltinExprCheck(builtinName),
			RetType: types.Str,
			LowerFunc: func(operand hir.Value) *hir.Call {
				return &hir.Call{Fn: cFunc, Args: []hir.Value{operand}, Type: "ptr"}
			},
		}
		return
	}

	// Binary: "Q|Q", "F+F", "Q&Q"
	// Find the operator character(s) between two builtin names
	for i, ch := range pattern {
		op := ""
		switch ch {
		case '|':
			op = "|"
		case '&':
			op = "&"
		case '+':
			op = "+"
		case '-':
			if i > 0 { // not unary minus
				op = "-"
			}
		case '*':
			op = "*"
		case '/':
			op = "/"
		}

		if op != "" && i > 0 && i < len(pattern)-1 {
			lhsBuiltin := pattern[:i]
			// rhsBuiltin := pattern[i+1:]  // not used directly, but validates pattern
			capturedOp := op
			capturedCFunc := cFunc

			proto.Operators[capturedOp] = &MacroOp{
				Op:       capturedOp,
				LhsCheck: makeBuiltinExprCheck(lhsBuiltin),
				RetType:  types.Str,
				LowerFunc: func(lhs, rhs hir.Value) *hir.Call {
					return &hir.Call{Fn: capturedCFunc, Args: []hir.Value{lhs, rhs}, Type: "ptr"}
				},
			}
			return
		}
	}
}

// makeBuiltinExprCheck creates a generic expression checker that returns true
// if the expression involves a call to the given builtin name.
// This is the fully generic replacement for hardcoded IsQExpression/IsFExpression.
func makeBuiltinExprCheck(builtinName string) func(ast.Expr, interface{}) bool {
	return func(expr ast.Expr, info interface{}) bool {
		return isBuiltinExpression(expr, builtinName)
	}
}

// isBuiltinExpression recursively checks if an AST expression involves
// a call to the specified builtin function name.
// This is 100% generic — the builtin name comes from the macro definition file.
func isBuiltinExpression(expr ast.Expr, builtinName string) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.CallExpr:
		if id, ok := e.Callee.(*ast.Ident); ok {
			return id.Name == builtinName
		}
	case *ast.BinaryExpr:
		return isBuiltinExpression(e.Lhs, builtinName) || isBuiltinExpression(e.Rhs, builtinName)
	case *ast.UnaryExpr:
		return isBuiltinExpression(e.X, builtinName)
	}
	return false
}

// resolveTypeName converts a type name string to a types.T.
func resolveTypeName(name string) types.T {
	switch name {
	case "str":
		return types.Str
	case "int":
		return types.Int
	case "float":
		return types.Float
	case "bool":
		return types.Bool
	default:
		return types.Str // default to str for macro builtins
	}
}
