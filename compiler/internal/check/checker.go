package check

import (
  "fmt"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
)

type checker struct {
  info  *Info
  fnSig FuncSig

  scope *scope

  errors   []error
  warnings []Warning

  locals        []*varInfo
  blockReturned []bool
}

// CheckFile performs semantic checks and returns info, errors, and warnings.
func CheckFile(f *ast.File) (*Info, []error, []Warning) {
  info := &Info{
    Funcs:   map[string]FuncSig{},
    Types:   map[string]string{},
    Structs: map[string]StructInfo{},
    Enums:   map[string]EnumInfo{}, // NEW
  }
  var errs []error
  var warns []Warning

  // collect structs (M7)
  for _, d := range f.Decls {
    if sd, ok := d.(*ast.StructDecl); ok {
      fields := map[string]string{}
      for _, ft := range sd.Fields {
        fields[ft.Name] = ft.Type
      }
      info.Structs[sd.Name] = StructInfo{Fields: fields}
    }
  }

  // collect enums (M8)
  for _, d := range f.Decls {
    if ed, ok := d.(*ast.EnumDecl); ok {
      variants := map[string]string{}
      for _, v := range ed.Variants {
        variants[v.Name] = v.Payload
      }
      info.Enums[ed.Name] = EnumInfo{Variants: variants}
    }
  }

  // collect function signatures
  for _, d := range f.Decls {
    fn, ok := d.(*ast.FuncDecl)
    if !ok {
      continue
    }
    if _, exists := info.Funcs[fn.Name]; exists {
      errs = append(errs, fmt.Errorf("duplicate function %q", fn.Name))
      continue
    }
    var ps []Kind
    for _, p := range fn.Params {
      k, _ := mapTypeOrStruct(p.Type, info) // now also maps enums
      ps = append(ps, k)
    }
    retK, _ := mapTypeOrStruct(fn.Ret, info)
    info.Funcs[fn.Name] = FuncSig{Name: fn.Name, Params: ps, Ret: retK}
  }

  // check bodies
  for _, d := range f.Decls {
    if fn, ok := d.(*ast.FuncDecl); ok {
      fnErrs, fnWarns := checkFunc(info, fn)
      errs = append(errs, fnErrs...)
      warns = append(warns, fnWarns...)
    }
  }
  return info, errs, warns
}

func checkFunc(info *Info, fn *ast.FuncDecl) ([]error, []Warning) {
  c := &checker{
    info:   info,
    fnSig:  info.Funcs[fn.Name],
    scope:  &scope{vars: map[string]*varInfo{}},
    locals: nil,
  }
  // params (immutable)
  for i, p := range fn.Params {
    k, sname := mapTypeOrStruct(p.Type, info)
    v := &varInfo{
      kind:       k,
      mutable:    false,
      declName:   p.Name,
      structName: sname, // reused for both struct+enum
      read:       false,
      written:    true,
    }
    if err := c.scope.define(p.Name, v); err != nil {
      c.errors = append(c.errors, fmt.Errorf("parameter %d %q: %v", i, p.Name, err))
    }
    c.locals = append(c.locals, v)
  }

  c.blockReturned = push(c.blockReturned, false)
  for _, s := range fn.Body {
    c.checkStmt(s)
  }
  hasReturn := *top(c.blockReturned)
  c.blockReturned = pop(c.blockReturned)

  // Non-void fallthrough check:
  // Accept either an explicit return OR a tail expression stmt as satisfying.
  if fnRet := c.fnSig.Ret; fnRet != KindVoid && !hasReturn {
    tailExprOK := false
    if len(fn.Body) > 0 {
      if es, ok := fn.Body[len(fn.Body)-1].(*ast.ExprStmt); ok {
        // quick kind check: tail expr kind should unify with return kind
        tk := c.kindOfExpr(es.Expr)
        if _, ok := unifyKinds(fnRet, tk); ok {
          tailExprOK = true
        }
      }
    }
    if !tailExprOK {
      c.warnings = append(c.warnings, Warning{
        Code: warnCode("warn", "missing_explicit_return", "DW0006"),
        Msg:  fmt.Sprintf("function %q returns %s but may fall through without an explicit return", fn.Name, fnRet),
      })
    }
  }

  // Unused vars/params (ignore names starting with "_")
  for _, v := range c.locals {
    if strings.HasPrefix(v.declName, "_") {
      continue
    }
    if !v.read {
      c.warnings = append(c.warnings, Warning{
        Code: warnCode("warn", "unused_variable", "DW0001"),
        Msg:  fmt.Sprintf("unused variable or parameter %q", v.declName),
      })
    }
  }

  return c.errors, c.warnings
}
