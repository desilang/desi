package check

import (
  "fmt"
  "sort"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
)

/* ---------- statements ---------- */

func (c *checker) checkStmt(s ast.Stmt) {
  if br := top(c.blockReturned); br != nil && *br {
    c.warnings = append(c.warnings, Warning{
      Code: CodeUnreachableCode(),
      Msg:  "unreachable code: statement after return",
    })
  }

  switch st := s.(type) {
  case *ast.LetStmt:
    c.checkLet(st)

  case *ast.AssignStmt:
    c.checkAssign(st)

  case *ast.ReturnStmt:
    exp := c.fnSig.Ret
    if st.Expr == nil {
      if exp != KindVoid {
        c.errors = append(c.errors, ErrWrongReturnKind(fmt.Sprintf("%s", exp), "void", "return"))
      }
      if br := top(c.blockReturned); br != nil {
        *br = true
      }
      return
    }
    got := c.kindOfExpr(st.Expr)
    if exp == KindVoid {
      c.errors = append(c.errors, ErrWrongReturnKind("void", fmt.Sprintf("%s", got), "return"))
      if br := top(c.blockReturned); br != nil {
        *br = true
      }
      return
    }
    if _, ok := unifyKinds(exp, got); !ok {
      c.errors = append(c.errors, ErrWrongReturnKind(fmt.Sprintf("%s", exp), fmt.Sprintf("%s", got), "return"))
    }
    if br := top(c.blockReturned); br != nil {
      *br = true
    }

  case *ast.ExprStmt:
    c.kindOfExpr(st.Expr)

  case *ast.IfStmt:
    k := c.kindOfExpr(st.Cond)
    if k != KindBool && k != KindInt && k != KindUnknown {
      c.errors = append(c.errors, fmt.Errorf("if-condition must be bool/int, got %s", k))
    }
    c.withBlock(func() {
      for _, s2 := range st.Then {
        c.checkStmt(s2)
      }
    })
    for _, el := range st.Elifs {
      k := c.kindOfExpr(el.Cond)
      if k != KindBool && k != KindInt && k != KindUnknown {
        c.errors = append(c.errors, fmt.Errorf("elif-condition must be bool/int, got %s", k))
      }
      c.withBlock(func() {
        for _, s2 := range el.Body {
          c.checkStmt(s2)
        }
      })
    }
    if st.Else != nil {
      c.withBlock(func() {
        for _, s2 := range st.Else {
          c.checkStmt(s2)
        }
      })
    }

  case *ast.WhileStmt:
    k := c.kindOfExpr(st.Cond)
    if k != KindBool && k != KindInt && k != KindUnknown {
      c.errors = append(c.errors, fmt.Errorf("while-condition must be bool/int, got %s", k))
    }
    c.withBlock(func() {
      for _, s2 := range st.Body {
        c.checkStmt(s2)
      }
    })

  case *ast.DeferStmt:
    if len(c.blockReturned) > 1 {
      c.errors = append(c.errors, fmt.Errorf("defer is only allowed at function top-level in Stage-0"))
    }
    if _, ok := st.Call.(*ast.CallExpr); !ok {
      c.errors = append(c.errors, fmt.Errorf("defer expects a call expression"))
    }
    c.kindOfExpr(st.Call)

  case *ast.MatchStmt:
    c.checkMatch(st)
  }
}

func (c *checker) checkLet(st *ast.LetStmt) {
  // Arity check
  if len(st.Binds) != len(st.Values) {
    c.errors = append(c.errors, typedErr(
      "type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
      "let", len(st.Binds), len(st.Values),
    ))
  }

  _max := _min(len(st.Binds), len(st.Values))
  for i := 0; i < _max; i++ {
    bd := st.Binds[i]
    rk := c.kindOfExpr(st.Values[i])

    declText := strings.TrimSpace(bd.Type)
    want, sname := mapTypeOrStruct(declText, c.info)

    kind := rk
    if declText != "" {
      if want == KindStruct || want == KindEnum {
        kind = want
      } else if want != KindUnknown {
        if k, ok := unifyKinds(want, rk); ok {
          kind = k
        } else {
          c.errors = append(c.errors, fmt.Errorf("let %q: type mismatch (declared %s, got %s)", bd.Name, want, rk))
        }
      }
    }

    // Shadowing warning
    if _, ok := c.scope.lookupLocal(bd.Name); !ok && c.scope.existsInOuter(bd.Name) {
      c.warnings = append(c.warnings, Warning{
        Code: CodeShadowedVariable(),
        Msg:  fmt.Sprintf("name %q shadows an outer binding", bd.Name),
      })
    }

    v := &varInfo{
      kind:       kindIfDeclOr(kind, want),
      mutable:    st.Mutable,
      declName:   bd.Name,
      structName: snameIfDeclOr(sname, want), // reused for enum name too
      written:    true,
    }
    if err := c.scope.define(bd.Name, v); err != nil {
      c.errors = append(c.errors, err)
    } else {
      c.locals = append(c.locals, v)
    }
  }

  // Optional group type: no-op for now.
  if strings.TrimSpace(st.GroupType) != "" {
  }
}

func (c *checker) checkAssign(st *ast.AssignStmt) {
  // Prefer new LHS (Expr) if available; fall back to legacy Names.
  if len(st.LHS) > 0 {
    if len(st.LHS) != len(st.Exprs) {
      c.errors = append(c.errors, typedErr(
        "type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
        "assignment", len(st.LHS), len(st.Exprs),
      ))
    }

    _max := _min(len(st.LHS), len(st.Exprs))
    for i := 0; i < _max; i++ {
      lhs := st.LHS[i]
      rk := c.kindOfExpr(st.Exprs[i])

      switch lv := lhs.(type) {
      case *ast.IdentExpr:
        v, ok := c.scope.lookup(lv.Name)
        if !ok {
          c.errors = append(c.errors, ErrUndefinedName(lv.Name, "assignment"))
          continue
        }
        if !v.mutable {
          c.errors = append(c.errors, ErrAssignToImmutable(lv.Name, "assignment"))
          continue
        }
        // Struct whole-value assignment.
        if v.kind == KindStruct {
          rhsStruct := c.structNameOfExpr(st.Exprs[i])
          if rhsStruct != "" && rhsStruct == v.structName {
            v.written = true
            continue
          }
          if rk != KindUnknown {
            c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible struct value", lv.Name))
          }
          v.written = true
          continue
        }
        // Enum whole-value assignment.
        if v.kind == KindEnum {
          rhsEnum := c.enumNameOfExpr(st.Exprs[i])
          if rhsEnum != "" && rhsEnum == v.structName {
            v.written = true
            continue
          }
          if rk != KindUnknown {
            c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible enum value", lv.Name))
          }
          v.written = true
          continue
        }

        if k, ok := unifyKinds(v.kind, rk); !ok {
          c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", v.kind), fmt.Sprintf("%s", rk), "assignment"))
        } else if v.kind == KindUnknown {
          v.kind = k
        }
        v.written = true

      case *ast.FieldExpr:
        // Struct field assignment checking.
        base, path := decomposeFieldChain(lv)
        if base == "" || len(path) == 0 {
          c.errors = append(c.errors, fmt.Errorf("unsupported assignment target"))
          continue
        }
        bv, ok := c.scope.lookup(base)
        if !ok {
          c.errors = append(c.errors, ErrUndefinedName(base, "assignment"))
          continue
        }
        if !bv.mutable {
          c.errors = append(c.errors, ErrAssignToImmutable(base, "field assignment"))
          continue
        }
        if bv.kind != KindStruct || bv.structName == "" {
          c.errors = append(c.errors, fmt.Errorf("cannot assign to field on non-struct %q", base))
          continue
        }
        current := bv.structName
        for j := 0; j < len(path)-1; j++ {
          si, ok := c.info.Structs[current]
          if !ok {
            c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", current))
            current = ""
            break
          }
          ftText, ok := si.Fields[path[j]]
          if !ok {
            c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", path[j], current))
            current = ""
            break
          }
          k, sname := mapTypeOrStruct(ftText, c.info)
          if k != KindStruct || sname == "" {
            c.errors = append(c.errors, fmt.Errorf("field %q on %q is not a struct", path[j], current))
            current = ""
            break
          }
          current = sname
        }
        if current == "" {
          continue
        }
        si, ok := c.info.Structs[current]
        if !ok {
          c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", current))
          continue
        }
        last := path[len(path)-1]
        ftText, ok := si.Fields[last]
        if !ok {
          c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", last, current))
          continue
        }
        want, _ := mapTypeOrStruct(ftText, c.info)
        if want != KindUnknown {
          if _, ok := unifyKinds(want, rk); !ok {
            c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", want), fmt.Sprintf("%s", rk), "field assignment"))
          }
        }
        bv.written = true

      default:
        c.errors = append(c.errors, fmt.Errorf("unsupported assignment target"))
      }
    }
    return
  }

  // ---- Legacy path (Names) ----
  if len(st.Names) != len(st.Exprs) {
    c.errors = append(c.errors, typedErr(
      "type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
      "assignment", len(st.Names), len(st.Exprs),
    ))
  }

  _max := _min(len(st.Names), len(st.Exprs))
  for i := 0; i < _max; i++ {
    name := st.Names[i]
    rk := c.kindOfExpr(st.Exprs[i])

    v, ok := c.scope.lookup(name)
    if !ok {
      c.errors = append(c.errors, ErrUndefinedName(name, "assignment"))
      continue
    }
    if !v.mutable {
      c.errors = append(c.errors, ErrAssignToImmutable(name, "assignment"))
      continue
    }

    if v.kind == KindStruct {
      rhsStruct := c.structNameOfExpr(st.Exprs[i])
      if rhsStruct != "" && rhsStruct == v.structName {
        v.written = true
        continue
      }
      if rk != KindUnknown {
        c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible struct value", name))
      }
      v.written = true
      continue
    }
    if v.kind == KindEnum {
      rhsEnum := c.enumNameOfExpr(st.Exprs[i])
      if rhsEnum != "" && rhsEnum == v.structName {
        v.written = true
        continue
      }
      if rk != KindUnknown {
        c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible enum value", name))
      }
      v.written = true
      continue
    }

    if k, ok := unifyKinds(v.kind, rk); !ok {
      c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", v.kind), fmt.Sprintf("%s", rk), "assignment"))
    } else if v.kind == KindUnknown {
      v.kind = k
    }
    v.written = true
  }
}

/* ---------- match checking (enums) ---------- */

func (c *checker) checkMatch(m *ast.MatchStmt) {
  // Scrutinee must be an enum-typed expression.
  sk := c.kindOfExpr(m.Scrut)
  if sk != KindEnum && sk != KindUnknown {
    c.errors = append(c.errors, fmt.Errorf("match expects an enum scrutinee, got %s", sk))
  }

  enumName := c.enumNameOfExpr(m.Scrut)
  // Build variant universe for exhaustiveness.
  var universe map[string]struct{}
  var payloadType = make(map[string]string)
  if enumName != "" {
    if ei, ok := c.info.Enums[enumName]; ok {
      universe = make(map[string]struct{}, len(ei.Variants))
      for vname, p := range ei.Variants {
        universe[vname] = struct{}{}
        payloadType[vname] = p
      }
    }
  }

  // Track duplicates & seen
  seen := map[string]struct{}{}
  wildcardSeen := false

  for i := range m.Arms {
    arm := &m.Arms[i]

    // Wildcard arm: "_:" — no variant name, no binder, just check body.
    if arm.Pat.Variant == "_" {
      if wildcardSeen {
        c.errors = append(c.errors, fmt.Errorf("duplicate wildcard arm '_'"))
      }
      wildcardSeen = true
      c.withBlock(func() {
        for _, s := range arm.Body {
          c.checkStmt(s)
        }
      })
      continue
    }

    // Unknown enum? We still check arm bodies in a block for general errors.
    if enumName == "" {
      c.withBlock(func() {
        if arm.Pat.Bind != "" && arm.Pat.Bind != "_" {
          _ = c.scope.define(arm.Pat.Bind, &varInfo{
            kind:     KindUnknown,
            mutable:  false,
            declName: arm.Pat.Bind,
            written:  true,
          })
        }
        for _, s := range arm.Body {
          c.checkStmt(s)
        }
      })
      continue
    }

    // Validate variant exists
    if _, ok := universe[arm.Pat.Variant]; !ok {
      c.errors = append(c.errors, fmt.Errorf("unknown variant %q for enum %q", arm.Pat.Variant, enumName))
      // still check body
      c.withBlock(func() {
        for _, s := range arm.Body {
          c.checkStmt(s)
        }
      })
      continue
    }

    // Duplicate variant in same match?
    if _, dup := seen[arm.Pat.Variant]; dup {
      c.errors = append(c.errors, fmt.Errorf("duplicate match arm for %q", arm.Pat.Variant))
    }
    seen[arm.Pat.Variant] = struct{}{}

    // Payload binder typing (if any)
    pt := strings.TrimSpace(payloadType[arm.Pat.Variant])
    c.withBlock(func() {
      if !isNoneText(pt) {
        // payloadful variant
        if arm.Pat.Bind != "" && arm.Pat.Bind != "_" {
          k, sname := mapTypeOrStruct(pt, c.info)
          _ = c.scope.define(arm.Pat.Bind, &varInfo{
            kind:       k,
            structName: sname, // reused for struct-or-enum name
            mutable:    false,
            declName:   arm.Pat.Bind,
            written:    true,
          })
        }
      } else {
        // payloadless variant
        if arm.Pat.Bind != "" {
          c.errors = append(c.errors, fmt.Errorf("variant %q has no payload; binder %q is invalid", arm.Pat.Variant, arm.Pat.Bind))
        }
      }

      for _, s := range arm.Body {
        c.checkStmt(s)
      }
    })
  }

  // Exhaustiveness warning (only if we know the enum and no wildcard)
  if enumName != "" && universe != nil && !wildcardSeen {
    missing := make([]string, 0, len(universe))
    for v := range universe {
      if _, ok := seen[v]; !ok {
        missing = append(missing, v)
      }
    }
    if len(missing) > 0 {
      sort.Strings(missing)
      c.warnings = append(c.warnings, Warning{
        Code: warnCode("warn", "non_exhaustive_match", "DW0007"),
        Msg:  fmt.Sprintf("non-exhaustive match on %s: missing %s", enumName, strings.Join(missing, ", ")),
      })
    }
  }
}

func (c *checker) structNameOfExpr(e ast.Expr) string {
  switch v := e.(type) {
  case *ast.IdentExpr:
    if vi, ok := c.scope.lookup(v.Name); ok && vi.kind == KindStruct {
      return vi.structName
    }
  }
  return ""
}

func (c *checker) withChildScope(body func()) {
  prev := c.scope
  c.scope = &scope{parent: prev, vars: map[string]*varInfo{}}
  body()
  c.scope = prev
}

func (c *checker) withBlock(body func()) {
  c.blockReturned = push(c.blockReturned, false)
  c.withChildScope(body)
  c.blockReturned = pop(c.blockReturned)
}

/* ---------- small helpers for let handling ---------- */

func kindIfDeclOr(current Kind, want Kind) Kind {
  if want == KindStruct || want == KindEnum {
    return want
  }
  return current
}

func snameIfDeclOr(sname string, want Kind) string {
  if want == KindStruct || want == KindEnum {
    return sname
  }
  return ""
}

// decomposeFieldChain flattens a.b.c into ("a", ["b","c"]).
func decomposeFieldChain(e *ast.FieldExpr) (string, []string) {
  var parts []string
  cur := e
  parts = append(parts, cur.Name)
  for {
    if id, ok := cur.X.(*ast.IdentExpr); ok {
      // reverse parts
      for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
        parts[i], parts[j] = parts[j], parts[i]
      }
      return id.Name, parts
    }
    if fe, ok := cur.X.(*ast.FieldExpr); ok {
      parts = append(parts, fe.Name)
      cur = fe
      continue
    }
    return "", nil
  }
}

/* ---------- misc helpers ---------- */

func isNoneText(t string) bool {
  switch strings.ToLower(strings.TrimSpace(t)) {
  case "", "none", "void":
    return true
  default:
    return false
  }
}
