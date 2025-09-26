package ast

import (
  "encoding/json"
  "errors"
  "fmt"
)

func asMap(v any) (map[string]any, bool) {
  m, ok := v.(map[string]any)
  return m, ok
}
func getString(m map[string]any, k string) string {
  if v, ok := m[k]; ok {
    if s, ok := v.(string); ok {
      return s
    }
  }
  return ""
}
func getBool(m map[string]any, k string) bool {
  if v, ok := m[k]; ok {
    if b, ok := v.(bool); ok {
      return b
    }
  }
  return false
}
func getSlice(m map[string]any, k string) []any {
  if v, ok := m[k]; ok {
    if a, ok := v.([]any); ok {
      return a
    }
  }
  return nil
}
func getMap(m map[string]any, k string) map[string]any {
  if v, ok := m[k]; ok {
    if mm, ok := v.(map[string]any); ok {
      return mm
    }
  }
  return nil
}
func getIntFromAny(v any) int {
  switch t := v.(type) {
  case float64:
    return int(t)
  case int:
    return t
  default:
    return 0
  }
}
func parseSpan(mm map[string]any) Span {
  if mm == nil {
    return Span{}
  }
  st := getMap(mm, "start")
  en := getMap(mm, "end")
  return Span{
    Start: Pos{Line: getIntFromAny(st["line"]), Col: getIntFromAny(st["col"])},
    End:   Pos{Line: getIntFromAny(en["line"]), Col: getIntFromAny(en["col"])},
  }
}

/* ---------- entrypoint ---------- */

// UnmarshalFileJSON reconstructs a *File from JSON produced by MarshalFileJSON.
func UnmarshalFileJSON(data []byte) (*File, error) {
  var root any
  if err := json.Unmarshal(data, &root); err != nil {
    return nil, err
  }
  m, ok := asMap(root)
  if !ok || getString(m, "kind") != "File" {
    return nil, errors.New("AST JSON: root is not kind=File")
  }
  var f File

  // package
  if pm := getMap(m, "package"); pm != nil && getString(pm, "kind") == "PackageDecl" {
    f.Pkg = &PackageDecl{Name: getString(pm, "name")}
  }

  // imports
  if arr := getSlice(m, "imports"); arr != nil {
    for _, it := range arr {
      im, ok := asMap(it)
      if !ok || getString(im, "kind") != "ImportDecl" {
        continue
      }
      f.Imports = append(f.Imports, ImportDecl{
        Path: getString(im, "path"),
        As:   getString(im, "as"),
        Span: parseSpan(getMap(im, "span")),
      })
    }
  }

  // from_imports (optional)
  if arr := getSlice(m, "from_imports"); arr != nil {
    for _, it := range arr {
      fm, ok := asMap(it)
      if !ok || getString(fm, "kind") != "FromImportDecl" {
        continue
      }
      fi := FromImportDecl{
        Module: getString(fm, "module"),
        Span:   parseSpan(getMap(fm, "span")),
      }
      if items := getSlice(fm, "items"); items != nil {
        for _, iv := range items {
          im, ok := asMap(iv)
          if !ok || getString(im, "kind") != "ImportItem" {
            continue
          }
          fi.Items = append(fi.Items, ImportItem{
            Name: getString(im, "name"),
            As:   getString(im, "as"),
            Span: parseSpan(getMap(im, "span")),
          })
        }
      }
      f.FromImports = append(f.FromImports, fi)
    }
  }

  // decls
  if arr := getSlice(m, "decls"); arr != nil {
    for _, d := range arr {
      dec, err := fromJDecl(d)
      if err != nil {
        return nil, err
      }
      if dec != nil {
        f.Decls = append(f.Decls, dec)
      }
    }
  }
  return &f, nil
}

/* ---------- decls ---------- */

func fromJDecl(v any) (Decl, error) {
  m, ok := asMap(v)
  if !ok {
    return nil, fmt.Errorf("AST JSON: decl not object")
  }
  switch getString(m, "kind") {
  case "FuncDecl":
    return fromJFunc(m)
  default:
    // ignore unknown decl kinds for now
    return nil, nil
  }
}

func fromJFunc(m map[string]any) (*FuncDecl, error) {
  fd := &FuncDecl{
    Name: getString(m, "name"),
    Ret:  getString(m, "ret"),
    Span: parseSpan(getMap(m, "span")),
  }
  // params
  if arr := getSlice(m, "params"); arr != nil {
    for _, p := range arr {
      pm, ok := asMap(p)
      if !ok || getString(pm, "kind") != "Param" {
        continue
      }
      fd.Params = append(fd.Params, Param{
        Name: getString(pm, "name"),
        Type: getString(pm, "type"),
        Span: parseSpan(getMap(pm, "span")),
      })
    }
  }
  // body
  if arr := getSlice(m, "body"); arr != nil {
    for _, s := range arr {
      st, err := fromJStmt(s)
      if err != nil {
        return nil, err
      }
      if st != nil {
        fd.Body = append(fd.Body, st)
      }
    }
  }
  return fd, nil
}

/* ---------- statements ---------- */

func fromJStmt(v any) (Stmt, error) {
  m, ok := asMap(v)
  if !ok {
    return nil, fmt.Errorf("AST JSON: stmt not object")
  }
  switch getString(m, "kind") {
  case "LetStmt":
    st := &LetStmt{
      Mutable:   getBool(m, "mutable"),
      GroupType: getString(m, "groupType"),
      Span:      parseSpan(getMap(m, "span")),
    }
    // binds
    if arr := getSlice(m, "binds"); arr != nil {
      for _, b := range arr {
        bm, ok := asMap(b)
        if !ok || getString(bm, "kind") != "LetBind" {
          continue
        }
        st.Binds = append(st.Binds, LetBind{
          Name: getString(bm, "name"),
          Type: getString(bm, "type"),
          Span: parseSpan(getMap(bm, "span")),
        })
      }
    }
    // values
    if arr := getSlice(m, "values"); arr != nil {
      for _, e := range arr {
        ex, err := fromJExpr(e)
        if err != nil {
          return nil, err
        }
        if ex != nil {
          st.Values = append(st.Values, ex)
        }
      }
    }
    return st, nil

  case "AssignStmt":
    st := &AssignStmt{
      Span: parseSpan(getMap(m, "span")),
    }
    // names
    if arr, ok := m["names"].([]any); ok {
      for _, nv := range arr {
        if s, ok := nv.(string); ok {
          st.Names = append(st.Names, s)
        }
      }
    }
    // exprs
    if arr := getSlice(m, "exprs"); arr != nil {
      for _, e := range arr {
        ex, err := fromJExpr(e)
        if err != nil {
          return nil, err
        }
        if ex != nil {
          st.Exprs = append(st.Exprs, ex)
        }
      }
    }
    return st, nil

  case "ReturnStmt":
    st := &ReturnStmt{Span: parseSpan(getMap(m, "span"))}
    if ev := m["expr"]; ev != nil {
      ex, err := fromJExpr(ev)
      if err != nil {
        return nil, err
      }
      st.Expr = ex
    }
    return st, nil

  case "ExprStmt":
    ex, err := fromJExpr(m["expr"])
    if err != nil {
      return nil, err
    }
    return &ExprStmt{Expr: ex, Span: parseSpan(getMap(m, "span"))}, nil

  case "IfStmt":
    st := &IfStmt{
      Span: parseSpan(getMap(m, "span")),
    }
    // cond
    if c, err := fromJExpr(m["cond"]); err == nil {
      st.Cond = c
    } else {
      return nil, err
    }
    // then
    if arr := getSlice(m, "then"); arr != nil {
      for _, s := range arr {
        ss, err := fromJStmt(s)
        if err != nil {
          return nil, err
        }
        if ss != nil {
          st.Then = append(st.Then, ss)
        }
      }
    }
    // elifs
    if arr := getSlice(m, "elifs"); arr != nil {
      for _, e := range arr {
        em, ok := asMap(e)
        if !ok || getString(em, "kind") != "ElseIf" {
          continue
        }
        el := ElseIf{
          Span: parseSpan(getMap(em, "span")),
        }
        if c, err := fromJExpr(em["cond"]); err == nil {
          el.Cond = c
        } else {
          return nil, err
        }
        if body := getSlice(em, "body"); body != nil {
          for _, s := range body {
            ss, err := fromJStmt(s)
            if err != nil {
              return nil, err
            }
            if ss != nil {
              el.Body = append(el.Body, ss)
            }
          }
        }
        st.Elifs = append(st.Elifs, el)
      }
    }
    // else
    if arr := getSlice(m, "else"); arr != nil {
      for _, s := range arr {
        ss, err := fromJStmt(s)
        if err != nil {
          return nil, err
        }
        if ss != nil {
          st.Else = append(st.Else, ss)
        }
      }
    }
    return st, nil

  case "WhileStmt":
    st := &WhileStmt{Span: parseSpan(getMap(m, "span"))}
    if c, err := fromJExpr(m["cond"]); err == nil {
      st.Cond = c
    } else {
      return nil, err
    }
    if arr := getSlice(m, "body"); arr != nil {
      for _, s := range arr {
        ss, err := fromJStmt(s)
        if err != nil {
          return nil, err
        }
        if ss != nil {
          st.Body = append(st.Body, ss)
        }
      }
    }
    return st, nil

  case "DeferStmt":
    ex, err := fromJExpr(m["call"])
    if err != nil {
      return nil, err
    }
    return &DeferStmt{Call: ex, Span: parseSpan(getMap(m, "span"))}, nil

  default:
    return nil, fmt.Errorf("AST JSON: unknown stmt kind %q", getString(m, "kind"))
  }
}

/* ---------- expressions ---------- */

func fromJExpr(v any) (Expr, error) {
  m, ok := asMap(v)
  if !ok {
    return nil, fmt.Errorf("AST JSON: expr not object")
  }
  switch getString(m, "kind") {
  case "IdentExpr":
    return &IdentExpr{Name: getString(m, "name"), Span: parseSpan(getMap(m, "span"))}, nil
  case "IntLit":
    return &IntLit{Value: getString(m, "value"), Span: parseSpan(getMap(m, "span"))}, nil
  case "StrLit":
    return &StrLit{Value: getString(m, "value"), Span: parseSpan(getMap(m, "span"))}, nil
  case "BoolLit":
    return &BoolLit{Value: getBool(m, "value"), Span: parseSpan(getMap(m, "span"))}, nil
  case "CallExpr":
    c := &CallExpr{Span: parseSpan(getMap(m, "span"))}
    cv, err := fromJExpr(m["callee"])
    if err != nil {
      return nil, err
    }
    c.Callee = cv
    if arr := getSlice(m, "args"); arr != nil {
      for _, a := range arr {
        ax, err := fromJExpr(a)
        if err != nil {
          return nil, err
        }
        c.Args = append(c.Args, ax)
      }
    }
    return c, nil
  case "IndexExpr":
    e := &IndexExpr{Span: parseSpan(getMap(m, "span"))}
    sv, err := fromJExpr(m["seq"])
    if err != nil {
      return nil, err
    }
    iv, err := fromJExpr(m["index"])
    if err != nil {
      return nil, err
    }
    e.Seq, e.Index = sv, iv
    return e, nil
  case "FieldExpr":
    return &FieldExpr{
      X:    mustExpr(fromJExpr(m["x"])),
      Name: getString(m, "name"),
      Span: parseSpan(getMap(m, "span")),
    }, nil
  case "UnaryExpr":
    return &UnaryExpr{
      Op:   getString(m, "op"),
      X:    mustExpr(fromJExpr(m["x"])),
      Span: parseSpan(getMap(m, "span")),
    }, nil
  case "BinaryExpr":
    return &BinaryExpr{
      Op:    getString(m, "op"),
      Left:  mustExpr(fromJExpr(m["left"])),
      Right: mustExpr(fromJExpr(m["right"])),
      Span:  parseSpan(getMap(m, "span")),
    }, nil
  default:
    return nil, fmt.Errorf("AST JSON: unknown expr kind %q", getString(m, "kind"))
  }
}

func mustExpr(e Expr, err error) Expr {
  if err != nil {
    return nil
  }
  return e
}
