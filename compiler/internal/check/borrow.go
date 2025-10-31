package check

import (
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/diag"
)

// detectAwait reports whether a subtree contains any 'await' expression.
func detectAwait(n ast.Node) bool {
  if n == nil {
    return false
  }
  return len(collectAwaitSpans(n)) > 0
}

// collectAwaitSpans walks the subtree and returns spans for all UnaryExpr(op=="await").
func collectAwaitSpans(n ast.Node) []diag.Span {
  var out []diag.Span

  var walkExpr func(e ast.Expr)
  var walkStmt func(s ast.Stmt)
  var walkBlock func(b *ast.Block)

  walkExpr = func(e ast.Expr) {
    if e == nil {
      return
    }
    switch x := e.(type) {
    case *ast.UnaryExpr:
      if x.Op == "await" {
        out = append(out, x.Span)
      }
      walkExpr(x.X)
    case *ast.BinaryExpr:
      walkExpr(x.X)
      walkExpr(x.Y)
    case *ast.CallExpr:
      walkExpr(x.Callee)
      for _, a := range x.Args {
        walkExpr(a)
      }
    case *ast.FieldExpr:
      walkExpr(x.X)
    case *ast.IndexExpr:
      walkExpr(x.X)
      walkExpr(x.Idx)
    case *ast.SliceExpr:
      walkExpr(x.X)
      walkExpr(x.I)
      walkExpr(x.J)
      walkExpr(x.K)
    case *ast.LambdaExpr:
      walkExpr(x.Body)
    case *ast.ListComp:
      walkExpr(x.Elem)
      for _, c := range x.Clauses {
        walkExpr(c.Target)
        walkExpr(c.Iter)
        walkExpr(c.If)
      }
    case *ast.DictComp:
      walkExpr(x.Key)
      walkExpr(x.Val)
      for _, c := range x.Clauses {
        walkExpr(c.Target)
        walkExpr(c.Iter)
        walkExpr(c.If)
      }
    case *ast.SetComp:
      walkExpr(x.Elem)
      for _, c := range x.Clauses {
        walkExpr(c.Target)
        walkExpr(c.Iter)
        walkExpr(c.If)
      }
    case *ast.Ident, *ast.IntLit, *ast.FloatLit, *ast.BoolLit, *ast.StrLit, *ast.NoneLit:
      // leaf
    default:
      // unmodeled node kinds
    }
  }

  walkStmt = func(s ast.Stmt) {
    if s == nil {
      return
    }
    switch st := s.(type) {
    case *ast.AssignStmt:
      walkExpr(st.LHS)
      walkExpr(st.RHS)
    case *ast.LetStmt:
      walkExpr(st.Value)
    case *ast.ReturnStmt:
      walkExpr(st.Value)
    case *ast.ExprStmt:
      walkExpr(st.Expr)
    case *ast.IfStmt:
      walkExpr(st.Cond)
      walkBlock(st.Then)
      for _, arm := range st.Elifs {
        walkExpr(arm.Cond)
        walkBlock(arm.Body)
      }
      walkBlock(st.Else)
    case *ast.WhileStmt:
      walkExpr(st.Cond)
      walkBlock(st.Body)
    case *ast.ForStmt:
      walkExpr(st.Target)
      walkExpr(st.Iter)
      walkBlock(st.Body)
    case *ast.UsingStmt:
      walkExpr(st.Bind)
      walkExpr(st.Init)
      walkBlock(st.Body)
    case *ast.DeferStmt:
      if st.Call != nil {
        walkExpr(st.Call)
      }
    case *ast.MatchStmt:
      walkExpr(st.Scrutinee)
      for _, arm := range st.Arms {
        walkExpr(arm.Pattern)
        walkExpr(arm.Result)
      }
    case *ast.DocStringStmt:
      // ignore
    default:
      // unmodeled
    }
  }

  walkBlock = func(b *ast.Block) {
    if b == nil {
      return
    }
    for _, s := range b.Stmts {
      walkStmt(s)
    }
  }

  switch n := n.(type) {
  case *ast.Block:
    walkBlock(n)
  case ast.Stmt:
    walkStmt(n)
  case ast.Expr:
    walkExpr(n)
  }
  return out
}

// awaitRecord tracks an 'await' site with a linearization index to compare against last-uses.
type awaitRecord struct {
  idx  int
  span diag.Span
}

// collectLastUsesAndAwaits walks the body and computes:
//   - lastUse[name]: last index where an expression touched the base storage of an inout param
//   - awaits: all await sites with their index
//
// It uses a simple pre-order linearization counter.
func collectLastUsesAndAwaits(body *ast.Block, inoutSet map[string]struct{}, baseOf func(ast.Expr) (string, bool)) (map[string]int, []awaitRecord, int) {
  lastUse := make(map[string]int)
  var awaits []awaitRecord
  idx := 0

  var walkExpr func(e ast.Expr)
  var walkStmt func(s ast.Stmt)
  var walkBlock func(b *ast.Block)

  markIfInout := func(e ast.Expr) {
    if e == nil {
      return
    }
    if name, ok := baseOf(e); ok {
      if _, isInout := inoutSet[name]; isInout {
        lastUse[name] = idx
      }
    }
  }

  walkExpr = func(e ast.Expr) {
    if e == nil {
      return
    }
    idx++
    // pre-order: mark base use first
    markIfInout(e)
    switch x := e.(type) {
    case *ast.UnaryExpr:
      if x.Op == "await" {
        awaits = append(awaits, awaitRecord{idx: idx, span: x.Span})
      }
      walkExpr(x.X)
    case *ast.BinaryExpr:
      walkExpr(x.X)
      walkExpr(x.Y)
    case *ast.CallExpr:
      walkExpr(x.Callee)
      for _, a := range x.Args {
        walkExpr(a)
      }
    case *ast.FieldExpr:
      walkExpr(x.X)
    case *ast.IndexExpr:
      walkExpr(x.X)
      walkExpr(x.Idx)
    case *ast.SliceExpr:
      walkExpr(x.X)
      walkExpr(x.I)
      walkExpr(x.J)
      walkExpr(x.K)
    case *ast.LambdaExpr:
      walkExpr(x.Body)
    case *ast.ListComp:
      walkExpr(x.Elem)
      for _, c := range x.Clauses {
        walkExpr(c.Target)
        walkExpr(c.Iter)
        walkExpr(c.If)
      }
    case *ast.DictComp:
      walkExpr(x.Key)
      walkExpr(x.Val)
      for _, c := range x.Clauses {
        walkExpr(c.Target)
        walkExpr(c.Iter)
        walkExpr(c.If)
      }
    case *ast.SetComp:
      walkExpr(x.Elem)
      for _, c := range x.Clauses {
        walkExpr(c.Target)
        walkExpr(c.Iter)
        walkExpr(c.If)
      }
    default:
      // literals/idents already handled by markIfInout
    }
  }

  walkStmt = func(s ast.Stmt) {
    if s == nil {
      return
    }
    switch st := s.(type) {
    case *ast.AssignStmt:
      walkExpr(st.LHS)
      walkExpr(st.RHS)
    case *ast.LetStmt:
      walkExpr(st.Value)
    case *ast.ReturnStmt:
      walkExpr(st.Value)
    case *ast.ExprStmt:
      walkExpr(st.Expr)
    case *ast.IfStmt:
      walkExpr(st.Cond)
      walkBlock(st.Then)
      for _, arm := range st.Elifs {
        walkExpr(arm.Cond)
        walkBlock(arm.Body)
      }
      walkBlock(st.Else)
    case *ast.WhileStmt:
      walkExpr(st.Cond)
      walkBlock(st.Body)
    case *ast.ForStmt:
      walkExpr(st.Target)
      walkExpr(st.Iter)
      walkBlock(st.Body)
    case *ast.UsingStmt:
      walkExpr(st.Bind)
      walkExpr(st.Init)
      walkBlock(st.Body)
    case *ast.DeferStmt:
      if st.Call != nil {
        walkExpr(st.Call)
      }
    case *ast.MatchStmt:
      walkExpr(st.Scrutinee)
      for _, arm := range st.Arms {
        walkExpr(arm.Pattern)
        walkExpr(arm.Result)
      }
    case *ast.DocStringStmt:
      // ignore
    default:
      // unmodeled
    }
  }

  walkBlock = func(b *ast.Block) {
    if b != nil {
      for _, s := range b.Stmts {
        walkStmt(s)
      }
    }
  }

  walkBlock(body)
  return lastUse, awaits, idx
}

// Phase-2 rule: In async functions with inout params, an 'await' is illegal only
// if it occurs BEFORE the last use of any inout parameter. We conservatively treat
// parameters with no observed uses as potentially used at end-of-body to avoid false negatives
// (keeps Phase-1 tests green); we may relax this default later.
func (c *checker) checkAsyncInoutAwait(fn *ast.FuncDecl) {
  if fn == nil || !fn.Async || fn.Body == nil {
    return
  }

  // Collect inout parameters (names).
  inoutSet := make(map[string]struct{})
  var inoutList []string
  for i := range fn.Params {
    p := fn.Params[i]
    if p.Mode == ast.ParamInout {
      name := p.Name.Name
      inoutSet[name] = struct{}{}
      inoutList = append(inoutList, name)
    }
  }
  if len(inoutList) == 0 {
    return
  }

  // Compute last uses and await points.
  lastUse, awaits, maxIdx := collectLastUsesAndAwaits(fn.Body, inoutSet, func(e ast.Expr) (string, bool) {
    return c.baseLvalue(e)
  })
  if len(awaits) == 0 {
    return
  }

  // Conservatively default last-use to end-of-body if never seen.
  for name := range inoutSet {
    if _, ok := lastUse[name]; !ok {
      lastUse[name] = maxIdx
    }
  }

  // For each await, find implicated params (those with lastUse after this await).
  for _, ar := range awaits {
    var implicated []string
    for name := range inoutSet {
      if ar.idx < lastUse[name] {
        implicated = append(implicated, name)
      }
    }
    if len(implicated) > 0 {
      c.add(diagAt("DBR0001", ar.span, "cannot hold 'inout' borrow across 'await' (parameter(s): "+strings.Join(implicated, ", ")+")"))
    }
  }
}
