package c

import (
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

func cExprFor(e ast.Expr, env *env) (string, string) {
	switch v := e.(type) {
	case *ast.IntLit:
		if hasPrefixAny(v.Value, "0b", "0B") {
			if n, err := strconv.ParseInt(v.Value[2:], 2, 64); err == nil {
				return strconv.FormatInt(n, 10), "int"
			}
			return "0", "int"
		}
		return v.Value, "int"

	case *ast.FloatLit:
		return v.Value, "double"

	case *ast.StrLit:
		return ensureCStringLiteral(v.Value), "str"

	case *ast.BoolLit:
		if v.Value {
			return "1", "int"
		}
		return "0", "int"

	case *ast.IdentExpr:
		// Local/param first
		if t, ok := env.vars[v.Name]; ok {
			tt := strings.TrimSpace(t)
			if isStrText(tt) {
				return v.Name, "str"
			}
			if tt == "f32" {
				return v.Name, "float"
			}
			if tt == "f64" {
				return v.Name, "double"
			}
			if isStructType(tt, env.info) {
				return v.Name, "struct:" + strings.TrimSpace(tt)
			}
			if isEnumType(tt, env.info) {
				return v.Name, "enum:" + strings.TrimSpace(tt)
			}
			return v.Name, "int"
		}
		// NEW: top-level constant macro in this unit or known const in Info.
		if env != nil && env.info != nil && env.info.Consts != nil {
			if ci, ok := env.info.Consts[v.Name]; ok {
				switch ci.Kind {
				case checkKindStr:
					return v.Name, "str"
				case checkKindInt, checkKindBool:
					return v.Name, "int"
				default:
					return v.Name, "int"
				}
			}
		}
		return v.Name, "int"

	case *ast.FieldExpr:
		// ---------- ENUM CONSTRUCTOR (no-call form): Enum.Variant ----------
		if id, ok := v.X.(*ast.IdentExpr); ok {
			enumName := strings.TrimSpace(id.Name)
			if isEnumType(enumName, env.info) {
				if ei, ok := env.info.Enums[enumName]; ok {
					if vt, exists := ei.Variants[v.Name]; exists && isNoneText(vt) {
						tag := enumName + "_" + v.Name
						return "(" + enumName + "){ .tag = " + tag + " }", "enum:" + enumName
					}
				}
			}
		}

		// ---------- Struct field chain ----------
		if ft, ok := exprTextualType(v, env); ok {
			ex := fieldAccessChain(v)
			if isStrText(ft) {
				return ex, "str"
			}
			if strings.TrimSpace(ft) == "f32" {
				return ex, "float"
			}
			if strings.TrimSpace(ft) == "f64" {
				return ex, "double"
			}
			if isStructType(ft, env.info) {
				return ex, "struct:" + strings.TrimSpace(ft)
			}
			return ex, "int"
		}

		// ---------- Module-alias / from-alias constant access ----------
		if _, ok := v.X.(*ast.IdentExpr); ok {
			if env != nil && env.info != nil && env.info.Consts != nil {
				if ci, ok := env.info.Consts[v.Name]; ok {
					switch ci.Kind {
					case checkKindStr:
						return v.Name, "str"
					case checkKindInt, checkKindBool:
						return v.Name, "int"
					default:
						return v.Name, "int"
					}
				}
			}
			return v.Name, "int"
		}
		return "0", ""

	case *ast.UnaryExpr:
		x, k := cExprFor(v.X, env)
		op := v.Op
		if op == "not" {
			op = "!"
		}
		return "(" + op + " " + x + ")", k

	case *ast.BinaryExpr:
		l, lk := cExprFor(v.Left, env)
		r, rk := cExprFor(v.Right, env)

		if (v.Op == "==" || v.Op == "!=") && (lk == "str" || rk == "str") {
			cmp := "strcmp(" + l + ", " + r + ")"
			if v.Op == "==" {
				return "(" + cmp + " == 0)", "int"
			}
			return "(" + cmp + " != 0)", "int"
		}
		if v.Op == "+" && (lk == "str" || rk == "str") {
			return "desi_str_concat(" + l + ", " + r + ")", "str"
		}

		if v.Op == "and" {
			return "(" + l + " && " + r + ")", "int"
		}
		if v.Op == "or" {
			return "(" + l + " || " + r + ")", "int"
		}

		// Result "kind" is a coarse bucket for downstream (e.g., print formatting)
		k := ""
		if lk == "str" || rk == "str" {
			k = "str"
		} else if lk == "double" || rk == "double" || lk == "float" || rk == "float" {
			k = "double"
		} else if lk == "int" && rk == "int" {
			k = "int"
		}
		return "(" + l + " " + v.Op + " " + r + ")", k

	case *ast.IndexExpr:
		return "0", ""

	case *ast.CallExpr:
		// Enum constructors
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				enumName := id.Name
				if isEnumType(enumName, env.info) {
					if ei, ok := env.info.Enums[enumName]; ok {
						vtype := ei.Variants[fe.Name]
						tag := enumName + "_" + fe.Name
						if isNoneText(vtype) {
							return "(" + enumName + "){ .tag = " + tag + " }", "enum:" + enumName
						}
						var ax string
						if len(v.Args) > 0 {
							ax, _ = cExprFor(v.Args[0], env)
						} else {
							ax = "0"
						}
						return "(" + enumName + "){ .tag = " + tag + ", .as." + fe.Name + " = " + ax + " }", "enum:" + enumName
					}
				}
			}
		}

		// builtin print(...)
		if id, ok := v.Callee.(*ast.IdentExpr); ok && id.Name == "print" {
			var specs []string
			var args []string
			for _, a := range v.Args {
				ax, ak := cExprFor(a, env)
				switch ak {
				case "str":
					specs = append(specs, "%s")
				case "double", "float":
					specs = append(specs, "%f")
				default:
					specs = append(specs, "%d")
				}
				args = append(args, ax)
			}
			if len(specs) == 0 {
				// print() with no args — just newline
				return "(printf(\"\\n\"), 0)", "void"
			}
			fmt := "\"" + strings.Join(specs, " ") + "\\n\""
			call := "printf(" + fmt
			if len(args) > 0 {
				call += ", " + strings.Join(args, ", ")
			}
			call += ")"
			return "(" + call + ", 0)", "void"
		}

		// std shims: fs/os/mem/str
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				switch id.Name + "." + fe.Name {
				case "fs.read_all":
					var args []string
					for _, a := range v.Args {
						ax, _ := cExprFor(a, env)
						args = append(args, ax)
					}
					return "desi_fs_read_all(" + strings.Join(args, ", ") + ")", "str"
				case "os.exit":
					var args []string
					for _, a := range v.Args {
						ax, _ := cExprFor(a, env)
						args = append(args, ax)
					}
					return "desi_os_exit(" + strings.Join(args, ", ") + ")", "void"
				case "mem.free":
					var args []string
					for _, a := range v.Args {
						ax, _ := cExprFor(a, env)
						args = append(args, ax)
					}
					return "(desi_mem_free(" + strings.Join(args, ", ") + "), 0)", "void"
				case "str.len":
					var args []string
					for _, a := range v.Args {
						ax, _ := cExprFor(a, env)
						args = append(args, ax)
					}
					return "desi_str_len(" + strings.Join(args, ", ") + ")", "int"
				case "str.at":
					var args []string
					for _, a := range v.Args {
						ax, _ := cExprFor(a, env)
						args = append(args, ax)
					}
					return "desi_str_at(" + strings.Join(args, ", ") + ")", "int"
				case "str.from_code":
					var args []string
					for _, a := range v.Args {
						ax, _ := cExprFor(a, env)
						args = append(args, ax)
					}
					return "desi_str_from_code(" + strings.Join(args, ", ") + ")", "str"
				/* NEW: task.* */
				case "task.sleep_ms":
					{
						var args []string
						for _, a := range v.Args {
							ax, _ := cExprFor(a, env)
							args = append(args, ax)
						}
						return "desi_task_sleep_ms(" + strings.Join(args, ", ") + ")", "future"
					}
				case "task.block_on":
					{
						// Minimal v1: assume Future[int]
						var args []string
						for _, a := range v.Args {
							ax, _ := cExprFor(a, env)
							args = append(args, ax)
						}
						return "desi_task_block_on_int(" + strings.Join(args, ", ") + ")", "int"
					}
				}
			}
		}

		// NEW: module alias calls m.add(...) → add(...)
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if _, ok := fe.X.(*ast.IdentExpr); ok {
				if fs, ok := env.sigs[fe.Name]; ok {
					var args []string
					for _, a := range v.Args {
						ax, _ := cExprFor(a, env)
						args = append(args, ax)
					}
					return fe.Name + "(" + strings.Join(args, ", ") + ")", fs.ret
				}
			}
		}

		// user function call (unqualified)
		if id, ok := v.Callee.(*ast.IdentExpr); ok {
			if fs, ok := env.sigs[id.Name]; ok {
				var args []string
				for _, a := range v.Args {
					ax, _ := cExprFor(a, env)
					args = append(args, ax)
				}
				return id.Name + "(" + strings.Join(args, ", ") + ")", fs.ret
			}
		}
		return "0", ""

	case *ast.StructLit:
		var parts []string
		for _, f := range v.Fields {
			cv, _ := cExprFor(f.Value, env)
			parts = append(parts, "."+f.Name+" = "+cv)
		}
		return "(" + v.Name + "){" + strings.Join(parts, ", ") + "}", "struct:" + v.Name

	default:
		return "0", ""
	}
}

// ---- helpers for expr typing / lvalues ----

func exprTextualType(e ast.Expr, env *env) (string, bool) {
	switch v := e.(type) {
	case *ast.IdentExpr:
		if t, ok := env.vars[v.Name]; ok {
			return strings.TrimSpace(t), true
		}
		return "", false
	case *ast.FieldExpr:
		bt, ok := exprTextualType(v.X, env)
		if !ok {
			return "", false
		}
		if !isStructType(bt, env.info) {
			return "", false
		}
		if ft, ok := fieldTypeOf(env.info, strings.TrimSpace(bt), v.Name); ok {
			return strings.TrimSpace(ft), true
		}
		return "", false
	default:
		return "", false
	}
}

func fieldAccessChain(fe *ast.FieldExpr) string {
	switch base := fe.X.(type) {
	case *ast.IdentExpr:
		return base.Name + "." + fe.Name
	case *ast.FieldExpr:
		return fieldAccessChain(base) + "." + fe.Name
	default:
		return ""
	}
}

// cLValueFor converts an LHS expression into a C lvalue string.
// Supports identifiers and nested field chains (a.b.c).
func cLValueFor(e ast.Expr, env *env) (string, bool) {
	switch v := e.(type) {
	case *ast.IdentExpr:
		return v.Name, true
	case *ast.FieldExpr:
		s := fieldAccessChain(v)
		if s == "" {
			return "", false
		}
		return s, true
	default:
		return "", false
	}
}

// Small shim to keep Kind mapping localized for consts (int/bool/str only here).
// We don't import check.Kind directly in this file, so mirror the cases we need.
const (
	checkKindUnknown = 0
	checkKindInt     = 1
	checkKindStr     = 2
	checkKindBool    = 3
)
