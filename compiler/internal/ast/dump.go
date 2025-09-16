package ast

import (
  "fmt"
  "strings"
)

/*** DUMP (pretty outline for CLI) ***/

func DumpFile(f *File) string {
  var b strings.Builder
  if f.Pkg != nil {
    fmt.Fprintf(&b, "package %s\n", f.Pkg.Name)
  }
  for _, im := range f.Imports {
    fmt.Fprintf(&b, "import %s\n", im.Path)
  }
  for _, d := range f.Decls {
    switch fn := d.(type) {
    case *FuncDecl:
      fmt.Fprintf(&b, "\ndef %s(", fn.Name)
      for i, p := range fn.Params {
        if i > 0 {
          b.WriteString(", ")
        }
        fmt.Fprintf(&b, "%s: %s", p.Name, p.Type)
      }
      fmt.Fprintf(&b, ") -> %s:\n", orDefault(fn.Ret, "void"))
      for _, s := range fn.Body {
        switch st := s.(type) {
        case *LetStmt:
          // let / let mut
          if st.Mutable {
            fmt.Fprintf(&b, "  let mut ")
          } else {
            fmt.Fprintf(&b, "  let ")
          }
          // print binds (show per-name types when present)
          for i, bd := range st.Binds {
            if i > 0 {
              b.WriteString(", ")
            }
            if strings.TrimSpace(bd.Type) == "" {
              fmt.Fprintf(&b, "%s", bd.Name)
            } else {
              fmt.Fprintf(&b, "%s: %s", bd.Name, bd.Type)
            }
          }
          if strings.TrimSpace(st.GroupType) != "" {
            fmt.Fprintf(&b, " : %s", st.GroupType)
          }
          b.WriteString(" = ")
          for i, e := range st.Values {
            if i > 0 {
              b.WriteString(", ")
            }
            b.WriteString(exprString(e))
          }
          b.WriteString("\n")
        case *AssignStmt:
          fmt.Fprintf(&b, "  ")
          for i, n := range st.Names {
            if i > 0 {
              b.WriteString(", ")
            }
            b.WriteString(n)
          }
          b.WriteString(" := ")
          for i, e := range st.Exprs {
            if i > 0 {
              b.WriteString(", ")
            }
            b.WriteString(exprString(e))
          }
          b.WriteString("\n")
        case *ReturnStmt:
          if st.Expr == nil {
            fmt.Fprintf(&b, "  return\n")
          } else {
            fmt.Fprintf(&b, "  return %s\n", exprString(st.Expr))
          }
        case *ExprStmt:
          fmt.Fprintf(&b, "  %s\n", exprString(st.Expr))
        case *IfStmt:
          fmt.Fprintf(&b, "  if %s:\n", exprString(st.Cond))
          for _, s2 := range st.Then {
            fmt.Fprintf(&b, "    %s\n", stmtString(s2))
          }
          for _, e := range st.Elifs {
            fmt.Fprintf(&b, "  elif %s:\n", exprString(e.Cond))
            for _, s2 := range e.Body {
              fmt.Fprintf(&b, "    %s\n", stmtString(s2))
            }
          }
          if st.Else != nil {
            fmt.Fprintf(&b, "  else:\n")
            for _, s2 := range st.Else {
              fmt.Fprintf(&b, "    %s\n", stmtString(s2))
            }
          }
        case *WhileStmt:
          fmt.Fprintf(&b, "  while %s:\n", exprString(st.Cond))
          for _, s2 := range st.Body {
            fmt.Fprintf(&b, "    %s\n", stmtString(s2))
          }
        case *DeferStmt:
          fmt.Fprintf(&b, "  defer %s\n", exprString(st.Call))
        }
      }
    }
  }
  return b.String()
}

func orDefault(s, d string) string {
  if strings.TrimSpace(s) == "" {
    return d
  }
  return s
}

func exprString(e Expr) string {
  switch v := e.(type) {
  case *IdentExpr:
    return v.Name
  case *IntLit:
    return v.Value
  case *StrLit:
    return v.Value
  case *BoolLit:
    if v.Value {
      return "true"
    }
    return "false"
  case *CallExpr:
    var parts []string
    for _, a := range v.Args {
      parts = append(parts, exprString(a))
    }
    return exprString(v.Callee) + "(" + strings.Join(parts, ", ") + ")"
  case *IndexExpr:
    return exprString(v.Seq) + "[" + exprString(v.Index) + "]"
  case *FieldExpr:
    return exprString(v.X) + "." + v.Name
  case *UnaryExpr:
    return v.Op + " " + exprString(v.X)
  case *BinaryExpr:
    return "(" + exprString(v.Left) + " " + v.Op + " " + exprString(v.Right) + ")"
  default:
    return "<expr>"
  }
}

func stmtString(s Stmt) string {
  switch st := s.(type) {
  case *LetStmt:
    var b strings.Builder
    if st.Mutable {
      b.WriteString("let mut ")
    } else {
      b.WriteString("let ")
    }
    for i, bd := range st.Binds {
      if i > 0 {
        b.WriteString(", ")
      }
      if strings.TrimSpace(bd.Type) == "" {
        b.WriteString(bd.Name)
      } else {
        fmt.Fprintf(&b, "%s: %s", bd.Name, bd.Type)
      }
    }
    if strings.TrimSpace(st.GroupType) != "" {
      fmt.Fprintf(&b, " : %s", st.GroupType)
    }
    b.WriteString(" = ")
    for i, e := range st.Values {
      if i > 0 {
        b.WriteString(", ")
      }
      b.WriteString(exprString(e))
    }
    return b.String()
  case *AssignStmt:
    var b strings.Builder
    for i, n := range st.Names {
      if i > 0 {
        b.WriteString(", ")
      }
      b.WriteString(n)
    }
    b.WriteString(" := ")
    for i, e := range st.Exprs {
      if i > 0 {
        b.WriteString(", ")
      }
      b.WriteString(exprString(e))
    }
    return b.String()
  case *ReturnStmt:
    if st.Expr == nil {
      return "return"
    }
    return "return " + exprString(st.Expr)
  case *ExprStmt:
    return exprString(st.Expr)
  case *IfStmt:
    return "if …:"
  case *WhileStmt:
    return "while …:"
  case *DeferStmt:
    return "defer " + exprString(st.Call)
  default:
    return "<stmt>"
  }
}
