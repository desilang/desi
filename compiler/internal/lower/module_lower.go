package lower

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	_ "github.com/desilang/desi/compiler/internal/macro" // ensure ORM protocol registers via init()
	"github.com/desilang/desi/compiler/internal/project"
	"github.com/desilang/desi/compiler/internal/types"
)

// LowerModuleOptions controls module lowering behavior.
type LowerModuleOptions struct {
	SkipBuiltinEnums bool                        // Don't generate Option/Result constructors
	IsImportedModule bool                        // Force-mangle all non-extern function definitions
	DbEngine         string                      // "postgres" or "mysql" — auto-injects dialect call in __top__
	DbDebugQueries   bool                        // if true, inject __db_set_debug_queries(1) in __top__
	NamedDatabases   map[string]project.Database // [database.name] sections from desi.mod
}

// LowerModuleFromSource lowers all top-level function declarations in 'mod'.
// - Synchronous def: 1 HIR func with the same name
// - Async def:       2 HIR funcs: wrapper "<name>" and poll "<name>$poll"
//
// 'src' is used for literal materialization (strings).
// NOTE: For M9B, we also skip functions decorated with @extern(...), even if a body is present.
func LowerModuleFromSource(mod *ast.Module, info *check.Info, src []byte) *hir.Module {
	return LowerModuleFromSourceWithOptions(mod, info, src, LowerModuleOptions{})
}

// LowerModuleFromSourceWithOptions is like LowerModuleFromSource but accepts options.
func LowerModuleFromSourceWithOptions(mod *ast.Module, info *check.Info, src []byte, opts LowerModuleOptions) *hir.Module {
	lambdaAliases := DesugarAsyncLambdas(mod, info)
	// Store aliases in info for calleeName resolution
	if info != nil && info.LambdaAliases == nil {
		info.LambdaAliases = make(map[string]string)
	}
	for k, v := range lambdaAliases {
		info.LambdaAliases[k] = v
	}

	out := &hir.Module{Name: mod.File, Globals: make(map[string]hir.Global)}
	globalNames := make(map[string]bool)

	// Phase 1: Extract globals from __top__ function
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "__top__" || fd.Body == nil {
			continue
		}
		// Identify globals (LetStmt with Pub=true)
		var keptStmts []ast.Stmt
		for _, s := range fd.Body.Stmts {
			ls, ok := s.(*ast.LetStmt)
			if ok {
				// This is a global constant
				globalNames[ls.Name.Name] = true

				// Determine type and value for HIR global
				// For now, assume integer/string literals or simple initializers
				// We rely on LowerIdent to emit loads from these globals
				val := "0" // placeholder default
				typ := "i64"
				isConst := false

				// Simple heuristic for type/value from literal
				if ls.Value != nil {
					switch v := ls.Value.(type) {
					case *ast.IntLit:
						val = v.Text
						typ = "i64"
					case *ast.BoolLit:
						if v.Value {
							val = "1"
						} else {
							val = "0"
						}
						typ = "i1"
					case *ast.StrLit:
						// Handle F-string parts or source-extracted strings
						if v.Value != "" {
							val = unescapeString(v.Value)
						} else if src != nil {
							text, ok := scanStringLiteral(src, v.Span.Start.Line, v.Span.Start.Col, v.Long)
							if ok {
								val = unescapeString(text)
							}
						}
						typ = "ptr"
					case *ast.FloatLit:
						val = v.Text
						typ = "double"
					case *ast.UnaryExpr:
						// Handle negative numbers: -100, -3.14
						if v.Op == "-" {
							switch inner := v.X.(type) {
							case *ast.IntLit:
								val = "-" + inner.Text
								typ = "i64"
							case *ast.FloatLit:
								val = "-" + inner.Text
								typ = "double"
							}
						}
					case *ast.CallExpr:
						// Struct/class constructor call: Point(x=0, y=0)
						// These are heap-allocated pointers, initialized at runtime
						typ = "ptr"
						val = "null"
					case *ast.BinaryExpr:
						// Arithmetic over literals: `let SECONDS_PER_DAY = 60 * 60 * 24`,
						// and concatenation: `let BANNER = "desi " + "0.1.0"`.
						// These have to be folded here, not left to __top__: an
						// imported module's __top__ only runs when one of its
						// *functions* is called, so a module that merely reads the
						// constant saw the "0" placeholder instead of the real value.
						if fv, ft, ok := foldConstExpr(v); ok {
							val, typ = fv, ft
						} else if sv, ok := foldConstStr(v, src); ok {
							val, typ = sv, "ptr"
						}
					}
				}

				if ls.Type != nil {
					typ = llvmTypeFromAST(ls.Type.Name)
				}

				out.Globals[ls.Name.Name] = hir.Global{
					Name:    ls.Name.Name,
					Type:    typ,
					Value:   val,
					IsConst: isConst,
				}
				// Don't keep this stmt in __top__ if we treated it as static global init?
				// But we might need run-time init for complex exprs.
				// For now, if we emit Global definition, we assume we don't re-declare local.
				// We should transform it to Assignment if it has side effects.
				// For simple constants, we can skip execution in __top__.
				// Actually, strict globals are usually statics.
				// Let's keep it in __top__ but transformed to Assign so it stores to global?
				// But LowerStmt will handle Assign to global correctly if we implement it.
				// So we should replace LetStmt with AssignStmt in the AST for __top__?
				// Construct AssignStmt: ls.Name = ls.Value
				assign := &ast.AssignStmt{
					LHS:  []ast.Expr{&ls.Name},
					RHS:  []ast.Expr{ls.Value},
					Span: ls.Span,
				}
				keptStmts = append(keptStmts, assign)
			} else {
				keptStmts = append(keptStmts, s)
			}
		}
		fd.Body.Stmts = keptStmts
	}

	// Globals reached by name through "from X import CONST" are globals too, even
	// though they were not declared here. Without this they lowered to a bare
	// %CONST SSA name — an undefined value — because only this module's own
	// top-level lets were in globalNames.
	for name := range fromImportedGlobals(info) {
		globalNames[name] = true
	}

	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Skip prototypes (no body) and any function decorated with @extern(...).
		// This lets extern decls survive parsing/resolution but produce no HIR.
		if fd.Body == nil || isExternDecorated(fd) {
			continue
		}
		// Skip @macro functions: they run at compile time in the macro
		// interpreter (eval package) and their bodies reference
		// interpreter-only symbols (ast_get_name, ast_insert_stmt, …) that
		// have no runtime definition — emitting them breaks the link.
		if isMacroDecorated(fd) {
			continue
		}

		if fd.Async {
			w, p := LowerAsyncFunc(fd, src, info, globalNames) // Need to update signature
			if w == nil || p == nil {
				// Barrier or other early-abort: skip this function, continue module.
				continue
			}
			out.Funcs = append(out.Funcs, w, p)
			continue
		}
		out.Funcs = append(out.Funcs, LowerFuncFromDeclEx(fd, info, src, globalNames, opts.IsImportedModule))
	}

	// Generate constructors for struct declarations
	for _, d := range mod.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			constructor := LowerStructConstructor(sd, info)
			if constructor != nil {
				out.Funcs = append(out.Funcs, constructor)
			}
		}
	}

	// Generate constructors for enum declarations
	for _, d := range mod.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			constructors := LowerEnumConstructors(ed, info)
			out.Funcs = append(out.Funcs, constructors...)
		}
	}

	// Generate constructors for built-in Option and Result types (unless skipped).
	// These are generated once using "str" as the placeholder type because it lowers to "ptr".
	// This matches the type erasure strategy where generic enum constructors take "ptr".
	if !opts.SkipBuiltinEnums {
		// Option<T> has variants: Some(T), Nothing
		optionGeneric := types.OptionOf(types.Str)
		out.Funcs = append(out.Funcs, LowerEnumConstructorsFromType("Option", optionGeneric)...)

		// Result<T, E> has variants: Ok(T), Err(E)
		resultGeneric := types.ResultOf(types.Str, types.Str)
		out.Funcs = append(out.Funcs, LowerEnumConstructorsFromType("Result", resultGeneric)...)
	}

	// Generate constructors and methods for class declarations
	for _, d := range mod.Decls {
		if cd, ok := d.(*ast.ClassDecl); ok {
			// Constructor
			constructors := LowerClassConstructor(cd, info, src, globalNames)
			out.Funcs = append(out.Funcs, constructors...)

			// Methods
			methods := LowerClassMethods(cd, info, src, globalNames)
			out.Funcs = append(out.Funcs, methods...)

			// @model ORM init: generate __orm_init_ClassName for model registration
			if modelInit := LowerModelInit(cd, info); modelInit != nil {
				out.Funcs = append(out.Funcs, modelInit)
			}

			// Monomorphization: Generate specialized versions for generic classes
			if len(cd.TypeParams) > 0 {
				monomorphized := LowerMonomorphizedClass(cd, info, src, globalNames)
				out.Funcs = append(out.Funcs, monomorphized...)
			}

			// Lower nested classes (constructors + methods)
			for _, nested := range cd.Nested {
				// Use qualified name: Parent_Nested
				qualifiedName := fmt.Sprintf("%s_%s", cd.Name.Name, nested.Name.Name)
				nestedConstructors := LowerClassConstructorWithName(nested, info, src, qualifiedName, globalNames)
				out.Funcs = append(out.Funcs, nestedConstructors...)

				nestedMethods := LowerClassMethodsWithName(nested, info, src, qualifiedName, globalNames)
				out.Funcs = append(out.Funcs, nestedMethods...)

				// Monomorphization for generic nested classes
				if len(nested.TypeParams) > 0 {
					nestedMonomorphized := LowerMonomorphizedClass(nested, info, src, globalNames)
					out.Funcs = append(out.Funcs, nestedMonomorphized...)
				}
			}
		}
	}

	// Lower explicit ImplDecl methods
	for _, d := range mod.Decls {
		if impl, ok := d.(*ast.ImplDecl); ok {
			typeName := impl.ForType.Name
			for _, m := range impl.Methods {
				// Mangle: Type_Method
				name := fmt.Sprintf("%s_%s", typeName, m.Name.Name)
				// Use LowerFuncFromDecl to properly handle parameters and return types
				fn := LowerFuncFromDecl(m, info, src, globalNames)
				fn.Name = name
				out.Funcs = append(out.Funcs, fn)
			}
		}
	}

	// Lower synthesized default Display methods
	if info != nil {
		for _, d := range mod.Decls {
			var typeName string
			if s, ok := d.(*ast.StructDecl); ok {
				typeName = s.Name.Name
			} else if c, ok := d.(*ast.ClassDecl); ok {
				typeName = c.Name.Name
			}

			if typeName != "" {
				// Check if this type has __str__ or __repr__ defined
				hasStrMethod := false
				reprFuncName := "" // If __repr__ is defined, we'll delegate to it
				if c, ok := d.(*ast.ClassDecl); ok {
					for _, m := range c.Methods {
						if m.Name.Name == "__str__" {
							hasStrMethod = true
							break
						}
						if m.Name.Name == "__repr__" {
							reprFuncName = fmt.Sprintf("%s___repr__", typeName)
						}
					}
				}

				if !hasStrMethod {
					if impls, ok := info.Impls[typeName]; ok {
						if methods, ok := impls["Display"]; ok {
							for _, m := range methods {
								// If body is nil, it's the synthesized default
								if m.Name.Name == "to_str" && m.Body == nil {
									name := fmt.Sprintf("%s_to_str", typeName)
									out.Funcs = append(out.Funcs, LowerDefaultToStr(name, typeName, reprFuncName))
								}
							}
						}
					}
				}
			}
		}
	}

	// Inject @model init calls into __top__ function so models register before main()
	var modelInitNames []string
	for _, d := range mod.Decls {
		if cd, ok := d.(*ast.ClassDecl); ok {
			if t := info.Types[cd]; t != nil {
				if cls, ok := t.(*types.Class); ok && cls.MacroDecorator != "" {
					modelInitNames = append(modelInitNames, fmt.Sprintf("__orm_init_%s", cd.Name.Name))
				}
			}
		}
	}
	if len(modelInitNames) > 0 {
		// Find __top__ function and prepend init calls
		for _, fn := range out.Funcs {
			if fn.Name == "__top__" && len(fn.Blocks) > 0 {
				var initStmts []hir.Stmt

				// Auto-inject DB dialect from desi.mod [database].engine
				if opts.DbEngine != "" {
					dialect := "0" // postgres
					if opts.DbEngine == "mysql" {
						dialect = "1"
					}
					initStmts = append(initStmts, &hir.Call{
						Fn:   "__orm_set_dialect",
						Args: []hir.Value{hir.ConstInt{Text: dialect}},
						Type: "i32",
					})
					initStmts = append(initStmts, &hir.Call{
						Fn:   "__db_set_dialect",
						Args: []hir.Value{hir.ConstInt{Text: dialect}},
						Type: "i32",
					})
					initStmts = append(initStmts, &hir.Call{
						Fn:   "__crud_set_dialect",
						Args: []hir.Value{hir.ConstInt{Text: dialect}},
						Type: "i32",
					})
				}

				// Debug queries: emit __db_set_debug_queries(1) from desi.mod
				if opts.DbDebugQueries {
					initStmts = append(initStmts, &hir.Call{
						Fn:   "__db_set_debug_queries",
						Args: []hir.Value{hir.ConstInt{Text: "1"}},
						Type: "i32",
					})
				}

				// Register named database connections from [database.name] sections
				if len(opts.NamedDatabases) > 0 {
					// Sort names for deterministic emit order
					var dbNames []string
					for name := range opts.NamedDatabases {
						dbNames = append(dbNames, name)
					}
					sort.Strings(dbNames)

					for _, name := range dbNames {
						db := opts.NamedDatabases[name]
						if db.Engine == "" || db.SchemaOnly {
							continue
						}
						// __db_register_conn(name, driver, host, port, dbname, user, password)
						initStmts = append(initStmts, &hir.Call{
							Fn: "__db_register_conn",
							Args: []hir.Value{
								hir.ConstStr{Text: name},
								hir.ConstStr{Text: db.Engine},
								hir.ConstStr{Text: db.Host},
								hir.ConstInt{Text: db.Port},
								hir.ConstStr{Text: db.Name},
								hir.ConstStr{Text: db.User},
								hir.ConstStr{Text: db.Password},
							},
							Type: "i32",
						})
					}
				}

				for _, initName := range modelInitNames {
					initStmts = append(initStmts, &hir.Call{
						Fn:   initName,
						Type: "void",
					})
				}
				fn.Blocks[0].Stmts = append(initStmts, fn.Blocks[0].Stmts...)
				break
			}
		}
	}

	return out
}

// isExternDecorated reports whether the function has an @extern(...) decorator.
func isExternDecorated(fd *ast.FuncDecl) bool {
	for _, dec := range fd.Decorators {
		if dec.Name.Name == "extern" {
			return true
		}
	}
	return false
}

// isMacroDecorated reports whether the function has an @macro decorator.
// Macro functions are compile-time-only (interpreted by the eval package).
func isMacroDecorated(fd *ast.FuncDecl) bool {
	for _, dec := range fd.Decorators {
		if dec.Name.Name == "macro" {
			return true
		}
	}
	return false
}

// LowerStructConstructor generates a constructor function for a struct.
// Constructor signature: StructName(field1_type %field1, ...) -> ptr
// Implementation: malloc struct, store fields, return pointer
func LowerStructConstructor(sd *ast.StructDecl, info *check.Info) *hir.Func {
	structName := sd.Name.Name
	b := hir.NewFunc(structName)

	// Look up struct type to get resolved field types
	var st *types.Struct
	if t := info.Types[sd]; t != nil {
		st, _ = t.(*types.Struct)
	}

	// Calculate layout and create parameters with proper alignment
	var params []hir.Param
	var offsets []int
	currentOffset := 0

	for i, field := range sd.Fields {
		var fieldType types.T
		if st != nil && i < len(st.Fields) {
			fieldType = st.Fields[i].Type
		}

		// Calculate size and alignment
		size := getSize(fieldType)
		align := getAlign(fieldType)

		// Align currentOffset to field's alignment requirement
		if align > 0 && currentOffset%align != 0 {
			currentOffset += align - (currentOffset % align)
		}

		offsets = append(offsets, currentOffset)
		currentOffset += size

		// Determine LLVM type for parameter
		llvmType := lowerType(fieldType)

		params = append(params, hir.Param{
			Name: field.Name.Name,
			Type: llvmType,
		})
	}
	b.Func().Params = params
	b.Func().RetType = "ptr"

	structSize := currentOffset
	if structSize == 0 {
		structSize = 1 // Minimum allocation
	}

	// Create entry block
	entry := hir.NewBlock("entry")

	// Allocate space for struct using malloc
	structPtr := hir.Temp{Name: "%struct_ptr"}
	entry.Stmts = append(entry.Stmts, &hir.Call{
		Dst:  structPtr,
		Fn:   "malloc",
		Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", structSize)}},
		Type: "ptr",
	})

	// Store each field
	for i, field := range sd.Fields {
		paramVar := hir.Var{Name: field.Name.Name}
		offset := offsets[i]

		// GEP to field offset
		fieldPtr := hir.Temp{Name: fmt.Sprintf("%%field_%s_ptr", field.Name.Name)}
		entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
			Type:    "i8",
			Base:    structPtr,
			Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
			Dst:     fieldPtr,
		})

		// Store parameter value to field
		entry.Stmts = append(entry.Stmts, &hir.Store{
			Dst: fieldPtr,
			Val: paramVar,
		})
	}

	// Return struct pointer
	entry.Stmts = append(entry.Stmts, &hir.Ret{Val: structPtr})

	b.Func().Blocks = []*hir.Block{entry}
	return b.Func()
}

// getSize returns the size in bytes for a given type
func getSize(t types.T) int {
	if t == nil {
		return 8 // default to pointer size
	}

	// Handle struct types (pointers)
	if _, ok := t.(*types.Struct); ok {
		return 8
	}

	name := t.String()
	switch name {
	case "int", "i32", "u32":
		return 4
	case "i64", "u64", "isize", "usize":
		return 8
	case "bool":
		return 1
	case "float", "f64":
		return 8
	case "f32":
		return 4
	case "str":
		return 8 // ptr
	default:
		return 8 // pointers, etc.
	}
}

// getAlign returns the alignment requirement in bytes for a given type
func getAlign(t types.T) int {
	if t == nil {
		return 8 // default to pointer alignment
	}

	// Handle struct/class types (pointers - 8-byte aligned)
	if _, ok := t.(*types.Struct); ok {
		return 8
	}
	if _, ok := t.(*types.Class); ok {
		return 8
	}

	name := t.String()
	switch name {
	case "int", "i32", "u32", "f32":
		return 4
	case "i64", "u64", "isize", "usize", "float", "f64":
		return 8
	case "bool":
		return 1
	case "str":
		return 8 // ptr
	default:
		return 8 // pointers, etc.
	}
}

// getClassSize calculates the total size needed for a class/struct instance
// accounting for field alignment requirements.
// This ensures fields are properly aligned for their type (e.g., pointers need 8-byte alignment).
func getClassSize(fields []types.Field) int {
	offset := 0
	for _, field := range fields {
		fieldSize := getSize(field.Type)
		fieldAlign := getAlign(field.Type)

		// Align offset to field's alignment requirement
		if fieldAlign > 0 && offset%fieldAlign != 0 {
			offset += fieldAlign - (offset % fieldAlign)
		}
		offset += fieldSize
	}
	if offset == 0 {
		return 1 // Minimum size
	}
	return offset
}

// getClassSizeFromTypes calculates the total size needed for a slice of types
// accounting for alignment requirements.
func getClassSizeFromTypes(fieldTypes []types.T) int {
	offset := 0
	for _, t := range fieldTypes {
		fieldSize := getSize(t)
		fieldAlign := getAlign(t)

		// Align offset to field's alignment requirement
		if fieldAlign > 0 && offset%fieldAlign != 0 {
			offset += fieldAlign - (offset % fieldAlign)
		}
		offset += fieldSize
	}
	if offset == 0 {
		return 1 // Minimum size
	}
	return offset
}

// constNum is a folded compile-time number. Exactly one of the fields is
// meaningful, selected by isFloat.
type constNum struct {
	i       int64
	f       float64
	isFloat bool
}

func (c constNum) asFloat() float64 {
	if c.isFloat {
		return c.f
	}
	return float64(c.i)
}

// foldConstExpr evaluates a numeric expression built only from literals and the
// arithmetic operators, returning the LLVM initializer text and type. ok is
// false for anything that is not a compile-time constant, which leaves the
// caller's placeholder in place.
//
// Module-level globals need this because an imported module's __top__ — where
// the runtime initializer would otherwise run — is only invoked when one of
// that module's functions is called. A module that just reads the constant
// never triggers it, so an unfolded initializer read as 0.
func foldConstExpr(e ast.Expr) (string, string, bool) {
	c, ok := foldNum(e)
	if !ok {
		return "", "", false
	}
	if c.isFloat {
		return formatDouble(c.f), "double", true
	}
	return strconv.FormatInt(c.i, 10), "i64", true
}

func foldNum(e ast.Expr) (constNum, bool) {
	switch v := e.(type) {
	case *ast.IntLit:
		n, err := strconv.ParseInt(strings.ReplaceAll(v.Text, "_", ""), 0, 64)
		if err != nil {
			return constNum{}, false
		}
		return constNum{i: n}, true

	case *ast.FloatLit:
		f, err := strconv.ParseFloat(strings.ReplaceAll(v.Text, "_", ""), 64)
		if err != nil {
			return constNum{}, false
		}
		return constNum{f: f, isFloat: true}, true

	case *ast.UnaryExpr:
		if v.Op != "-" {
			return constNum{}, false
		}
		c, ok := foldNum(v.X)
		if !ok {
			return constNum{}, false
		}
		if c.isFloat {
			return constNum{f: -c.f, isFloat: true}, true
		}
		return constNum{i: -c.i}, true

	case *ast.BinaryExpr:
		lhs, ok := foldNum(v.Lhs)
		if !ok {
			return constNum{}, false
		}
		rhs, ok := foldNum(v.Rhs)
		if !ok {
			return constNum{}, false
		}
		return foldBinary(v.Op, lhs, rhs)
	}
	return constNum{}, false
}

func foldBinary(op string, lhs, rhs constNum) (constNum, bool) {
	// Division and modulo by zero panic at run time; refuse to fold so that
	// diagnostic is not replaced by a silently wrong constant.
	if op == "/" || op == "%" {
		if (rhs.isFloat && rhs.f == 0) || (!rhs.isFloat && rhs.i == 0) {
			return constNum{}, false
		}
	}

	if lhs.isFloat || rhs.isFloat {
		a, b := lhs.asFloat(), rhs.asFloat()
		switch op {
		case "+":
			return constNum{f: a + b, isFloat: true}, true
		case "-":
			return constNum{f: a - b, isFloat: true}, true
		case "*":
			return constNum{f: a * b, isFloat: true}, true
		case "/":
			return constNum{f: a / b, isFloat: true}, true
		}
		return constNum{}, false
	}

	switch op {
	case "+":
		return constNum{i: lhs.i + rhs.i}, true
	case "-":
		return constNum{i: lhs.i - rhs.i}, true
	case "*":
		return constNum{i: lhs.i * rhs.i}, true
	case "/":
		return constNum{i: lhs.i / rhs.i}, true
	case "%":
		return constNum{i: lhs.i % rhs.i}, true
	}
	return constNum{}, false
}

// formatDouble renders a float64 as an LLVM double initializer. LLVM rejects an
// integer-looking token for a floating-point type, so a decimal point is always
// present.
func formatDouble(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// fromImportedGlobals returns the local names bound by "from X import CONST"
// statements that name an exported global rather than a func, class or alias.
//
// The name is looked up across every module's exports rather than only the one
// FromItems records for it: two "from consts import …" statements load that
// module twice, so the *ast.Module in FromItems is not always the pointer
// LazyModules holds for the same path. Matching by name mirrors how
// check.injectImports resolves imported classes and type aliases.
func fromImportedGlobals(info *check.Info) map[string]bool {
	out := map[string]bool{}
	if info == nil || info.R == nil {
		return out
	}
	for name := range info.R.FromItems {
		for _, ex := range info.R.ModuleExports {
			if ex == nil {
				continue
			}
			if _, isGlobal := ex.Globals[name]; isGlobal {
				out[name] = true
				break
			}
		}
	}
	return out
}

// foldConstStr concatenates an expression built only from string literals joined
// by '+', returning the unescaped text. ok is false for anything else.
//
// The literal's text comes from the same two places the StrLit case above uses:
// Value when the parser kept it, otherwise a re-scan of the source span.
func foldConstStr(e ast.Expr, src []byte) (string, bool) {
	switch v := e.(type) {
	case *ast.StrLit:
		if v.Value != "" {
			return unescapeString(v.Value), true
		}
		if src != nil {
			if text, ok := scanStringLiteral(src, v.Span.Start.Line, v.Span.Start.Col, v.Long); ok {
				return unescapeString(text), true
			}
		}
		return "", false
	case *ast.BinaryExpr:
		if v.Op != "+" {
			return "", false
		}
		lhs, ok := foldConstStr(v.Lhs, src)
		if !ok {
			return "", false
		}
		rhs, ok := foldConstStr(v.Rhs, src)
		if !ok {
			return "", false
		}
		return lhs + rhs, true
	}
	return "", false
}
