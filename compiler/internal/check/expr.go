package check

import (
  "fmt"

  "github.com/desilang/desi/compiler/internal/ast"
)

/* ---------- expressions ---------- */

func (c *checker) kindOfExpr(e ast.Expr) Kind {
  switch v := e.(type) {
  case *ast.IntLit:
    return KindInt
  case *ast.StrLit:
    return KindStr
  case *ast.BoolLit:
    return KindBool
  case *ast.IdentExpr:
    if vi, ok := c.scope.lookup(v.Name); ok {
      vi.read = true
      return vi.kind
    }
    if _, isFn := c.info.Funcs[v.Name]; isFn {
      return KindUnknown
    }
    c.errors = append(c.errors, fmt.Errorf("use of undeclared identifier %q", v.Name))
    return KindUnknown
  case *ast.UnaryExpr:
    k := c.kindOfExpr(v.X)
    if v.Op == "-" || v.Op == "!" || v.Op == "not" {
      if k == KindInt || k == KindBool || k == KindUnknown {
        return KindInt
      }
    }
    return KindUnknown
  case *ast.BinaryExpr:
    lk := c.kindOfExpr(v.Left)
    rk := c.kindOfExpr(v.Right)
    switch v.Op {
    case "+":
      if lk == KindStr || rk == KindStr {
        return KindStr
      }
      if lk == KindInt && rk == KindInt {
        return KindInt
      }
      return KindUnknown
    case "-", "*", "/", "%", "<", "<=", ">", ">=", "==", "!=":
      if _, ok := unifyKinds(lk, rk); ok {
        return KindInt
      }
      return KindUnknown
    case "and", "or", "|>":
      return KindInt
    default:
      return KindUnknown
    }
  case *ast.FieldExpr:
    return KindUnknown
  case *ast.IndexExpr:
    return KindUnknown
  case *ast.CallExpr:
    // std.io.println
    if fe, ok := v.Callee.(*ast.FieldExpr); ok {
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "io" && fe.Name == "println" {
        for i, a := range v.Args {
          ak := c.kindOfExpr(a)
          switch ak {
          case KindInt, KindStr, KindBool:
          case KindVoid:
            c.errors = append(c.errors, fmt.Errorf("io.println arg %d is void (no value)", i+1))
          default:
            c.errors = append(c.errors, fmt.Errorf("io.println arg %d has unsupported kind %s", i+1, ak))
          }
        }
        return KindVoid
      }
      // std.fs.read_all(path: str) -> str
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "fs" && fe.Name == "read_all" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("fs.read_all: want 1 arg (path: str), got %d", len(v.Args)))
        } else if ak := c.kindOfExpr(v.Args[0]); ak != KindStr && ak != KindUnknown {
          c.errors = append(c.errors, fmt.Errorf("fs.read_all: path must be str, got %s", ak))
        }
        return KindStr
      }
      // std.os.exit(code: int) -> void
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "os" && fe.Name == "exit" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("os.exit: want 1 arg (code: int), got %d", len(v.Args)))
        } else if ak := c.kindOfExpr(v.Args[0]); ak != KindInt && ak != KindUnknown {
          c.errors = append(c.errors, fmt.Errorf("os.exit: code must be int, got %s", ak))
        }
        return KindVoid
      }
      // std.mem.free(x) -> void   (accepts str or unknown)
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "mem" && fe.Name == "free" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("mem.free: want 1 arg, got %d", len(v.Args)))
        } else {
          ak := c.kindOfExpr(v.Args[0])
          if ak != KindStr && ak != KindUnknown && ak != KindVoid {
            c.errors = append(c.errors, fmt.Errorf("mem.free: arg must be str, got %s", ak))
          }
        }
        return KindVoid
      }
      // std.str.len(s: str) -> int
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "len" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("str.len: want 1 arg (str), got %d", len(v.Args)))
        } else if ak := c.kindOfExpr(v.Args[0]); ak != KindStr && ak != KindUnknown {
          c.errors = append(c.errors, fmt.Errorf("str.len: arg must be str, got %s", ak))
        }
        return KindInt
      }
      // std.str.at(s: str, i: int) -> int
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "at" {
        if len(v.Args) != 2 {
          c.errors = append(c.errors, fmt.Errorf("str.at: want 2 args (str,int), got %d", len(v.Args)))
        } else {
          if ak := c.kindOfExpr(v.Args[0]); ak != KindStr && ak != KindUnknown {
            c.errors = append(c.errors, fmt.Errorf("str.at: first arg must be str, got %s", ak))
          }
          if ik := c.kindOfExpr(v.Args[1]); ik != KindInt && ik != KindUnknown {
            c.errors = append(c.errors, fmt.Errorf("str.at: second arg must be int, got %s", ik))
          }
        }
        return KindInt
      }
      // std.str.from_code(i: int) -> str
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "from_code" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("str.from_code: want 1 arg (int), got %d", len(v.Args)))
        } else if ak := c.kindOfExpr(v.Args[0]); ak != KindInt && ak != KindUnknown {
          c.errors = append(c.errors, fmt.Errorf("str.from_code: arg must be int, got %s", ak))
        }
        return KindStr
      }
    }
    // user function call
    if id, ok := v.Callee.(*ast.IdentExpr); ok {
      if sig, ok := c.info.Funcs[id.Name]; ok {
        if len(sig.Params) != len(v.Args) {
          c.errors = append(c.errors, fmt.Errorf("call to %s: want %d args, got %d", id.Name, len(sig.Params), len(v.Args)))
        }
        n := min(len(sig.Params), len(v.Args))
        for i := 0; i < n; i++ {
          ak := c.kindOfExpr(v.Args[i])
          pk := sig.Params[i]
          if _, ok := unifyKinds(pk, ak); !ok {
            c.errors = append(c.errors, fmt.Errorf("call to %s: arg %d kind mismatch (want %s, got %s)", id.Name, i+1, pk, ak))
          }
        }
        return sig.Ret
      }
      c.errors = append(c.errors, fmt.Errorf("call to unknown function %q", id.Name))
      return KindUnknown
    }
    return KindUnknown
  default:
    return KindUnknown
  }
}
