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

  case *ast.StrLit:
    return ensureCStringLiteral(v.Value), "str"

  case *ast.BoolLit:
    if v.Value {
      return "1", "int"
    }
    return "0", "int"

  case *ast.IdentExpr:
    if t, ok := env.vars[v.Name]; ok {
      if isStrText(t) {
        return v.Name, "str"
      }
      if isStructType(t, env.info) {
        return v.Name, "struct:" + strings.TrimSpace(t)
      }
      return v.Name, "int"
    }
    return v.Name, "int"

  case *ast.FieldExpr:
    // Only handle base identifiers (and nested) that are known struct-typed variables.
    if ft, ok := exprTextualType(v, env); ok {
      ex := fieldAccessChain(v)
      if isStrText(ft) {
        return ex, "str"
      }
      if isStructType(ft, env.info) {
        return ex, "struct:" + strings.TrimSpace(ft)
      }
      return ex, "int"
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

    k := ""
    if lk == "str" || rk == "str" {
      k = "str"
    } else if lk == "int" && rk == "int" {
      k = "int"
    }
    return "(" + l + " " + v.Op + " " + r + ")", k

  case *ast.IndexExpr:
    return "0", ""

  case *ast.CallExpr:
    // Builtins / std shims
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
        }
      }
    }
    // user function call
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
