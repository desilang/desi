package c

import (
  "bytes"
  "strconv"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/check"
  "github.com/desilang/desi/compiler/internal/term"
)

type env struct {
  fn          *ast.FuncDecl
  info        *check.Info
  sigs        map[string]sig
  vars        map[string]string // name -> textual type ("int"/"str"/struct name/enum name/...)
  retKind     string            // "void"|"int"|"str"|"struct:<Name>"|"enum:<Name>"
  defers      []ast.Expr        // function-scope defers (LIFO)
  tempCounter int

  // NEW: alias map from from-imports (alias -> original)
  aliases map[string]string
}

func (e *env) newTemp() string {
  e.tempCounter++
  return "__tmp" + strconv.Itoa(e.tempCounter)
}

func emitFunc(b *bytes.Buffer, fn *ast.FuncDecl, sigs map[string]sig, info *check.Info, isMain bool) {
  // Async lowering path
  if fn.Async && !isMain {
    emitAsyncFunc(b, fn, sigs, info)
    return
  }

  e := &env{
    fn:      fn,
    info:    info,
    sigs:    sigs,
    vars:    map[string]string{},
    retKind: typeToKindOrStruct(fn.Ret, info),
    defers:  nil,
    aliases: nil,
  }

  // plumb alias map from checker info
  if info != nil && info.Aliases != nil {
    e.aliases = info.Aliases
  }

  for _, p := range fn.Params {
    if strings.TrimSpace(p.Type) == "" {
      e.vars[p.Name] = "int"
    } else {
      e.vars[p.Name] = strings.TrimSpace(p.Type)
    }
  }

  // signature
  if isMain {
    term.Wprintf(b, "int main(void) {\n")
  } else {
    term.Wprintf(b, "static %s %s(%s) {\n",
      cType(e.retKind), fn.Name, cParamList(fn, info))
  }

  // ---- SPECIAL-CASE: task shims ----
  // If the user imported compiler/lib/task.desi, its bodies are just stubs.
  // Emit real C wrappers that call the runtime instead of lowering the stub body.
  //   pub def sleep_ms(ms: int) -> future
  //   pub def block_on(fut: future) -> int
  if fn.Name == "sleep_ms" && len(fn.Params) == 1 && e.retKind == "future" {
    // Return a future via runtime shim.
    pn := fn.Params[0].Name
    term.Wprintf(b, "  return desi_task_sleep_ms(%s);\n", pn)
    term.Wprintf(b, "}\n")
    return
  }
  if fn.Name == "block_on" && len(fn.Params) == 1 && e.retKind == "int" {
    // Block and return int result via runtime shim.
    pn := fn.Params[0].Name
    term.Wprintf(b, "  return desi_task_block_on_int(%s);\n", pn)
    term.Wprintf(b, "}\n")
    return
  }

  // body (with implicit tail-expression return lowering)
  tailReturned := false
  for i, s := range fn.Body {
    if i == len(fn.Body)-1 && e.retKind != "void" {
      if es, ok := s.(*ast.ExprStmt); ok {
        cExpr, kind := cExprFor(es.Expr, e)
        switch {
        case e.retKind == "int" && kind != "int":
          term.Wprintf(b, "%s/* non-int tail expr; force 0 */\n", "  ")
          term.Wprintf(b, "%sreturn 0;\n", "  ")
        case e.retKind == "str" && kind != "str":
          term.Wprintf(b, "%s/* non-str tail expr; force \"\" */\n", "  ")
          term.Wprintf(b, "%sreturn \"\";\n", "  ")
        default:
          term.Wprintf(b, "%sreturn %s;\n", "  ", cExpr)
        }
        tailReturned = true
        continue
      }
    }
    emitStmt(b, 2, s, e)
  }

  if len(e.defers) > 0 {
    emitDefers(b, 2, e)
  }
  if !tailReturned && !hasTailReturn(fn.Body) {
    switch e.retKind {
    case "void":
      // no-op
    case "int":
      term.Wprintf(b, "  return 0;\n")
    case "str":
      term.Wprintf(b, "  return \"\";\n")
    default:
      switch {
      case strings.HasPrefix(e.retKind, "struct:"):
        name := strings.TrimPrefix(e.retKind, "struct:")
        term.Wprintf(b, "  return (%s){0};\n", name)
      case strings.HasPrefix(e.retKind, "enum:"):
        name := strings.TrimPrefix(e.retKind, "enum:")
        term.Wprintf(b, "  return (%s){0};\n", name)
      default:
        term.Wprintf(b, "  return 0;\n")
      }
    }
  }
  term.Wprintf(b, "}\n")
}

func hasTailReturn(body []ast.Stmt) bool {
  if len(body) == 0 {
    return false
  }
  _, ok := body[len(body)-1].(*ast.ReturnStmt)
  return ok
}

/* ---------- Minimal async lowering (single await sleep_ms + tail expr) ---------- */

func emitAsyncFunc(b *bytes.Buffer, fn *ast.FuncDecl, sigs map[string]sig, info *check.Info) {
  // Gather a single await on task.sleep_ms(ms) if present, and a tail expr value.
  var msExpr string
  var hasAwait bool
  var retExpr string

  // Last expression (tail) to compute result; default "0"
  retExpr = "0"
  {
    // Tail return/expr
    if n := len(fn.Body); n > 0 {
      switch last := fn.Body[n-1].(type) {
      case *ast.ReturnStmt:
        if last.Expr != nil {
          ex, _ := cExprFor(last.Expr, &env{fn: fn, info: info, sigs: sigs, vars: map[string]string{}})
          retExpr = ex
        }
      case *ast.ExprStmt:
        ex, _ := cExprFor(last.Expr, &env{fn: fn, info: info, sigs: sigs, vars: map[string]string{}})
        retExpr = ex
      }
    }
  }
  // Find first await sleep_ms
scan:
  for _, st := range fn.Body {
    es, ok := st.(*ast.ExprStmt)
    if !ok {
      continue
    }
    // We only recognize: await task.sleep_ms(expr)
    if aw, ok := es.Expr.(*ast.AwaitExpr); ok {
      if call, ok := aw.Expr.(*ast.CallExpr); ok {
        if fe, ok := call.Callee.(*ast.FieldExpr); ok {
          if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "task" && fe.Name == "sleep_ms" {
            // One arg (ms)
            if len(call.Args) > 0 {
              ax, _ := cExprFor(call.Args[0], &env{fn: fn, info: info, sigs: sigs, vars: map[string]string{}})
              msExpr = ax
            } else {
              msExpr = "0"
            }
            hasAwait = true
            break scan
          }
        }
      }
    }
  }

  // Types: minimal support for Future[int] for now.
  // Generate:
  //   struct <fn>_Future { int __state; int __result; struct desi_future __child; int __ms; };
  //   static int <fn>__poll(void* selfv) { ... }
  //   static void <fn>__destroy(void* selfv) { free(selfv); }
  //   static int <fn>__get_int(void* selfv) { return self->__result; }
  //   static struct desi_future <fn>(params...) { allocate; init; return desi_future_make(...); }
  typeName := fn.Name + "_Future"

  term.Wprintf(b, "typedef struct { int __state; int __result; struct desi_future __child; int __ms; } %s;\n", typeName)

  // poll
  term.Wprintf(b, "static int %s__poll(void* selfv) {\n", fn.Name)
  term.Wprintf(b, "  %s* self = (%s*)selfv;\n", typeName, typeName)
  term.Wprintf(b, "  switch (self->__state) {\n")
  term.Wprintf(b, "  case 0:\n")
  if hasAwait {
    term.Wprintf(b, "    self->__child = desi_task_sleep_ms(self->__ms);\n")
    term.Wprintf(b, "    self->__state = 1;\n")
    term.Wprintf(b, "    return 0;\n")
  } else {
    // no await: ready immediately
    term.Wprintf(b, "    self->__result = (%s);\n", retExpr)
    term.Wprintf(b, "    return 1;\n")
  }
  term.Wprintf(b, "  case 1:\n")
  if hasAwait {
    term.Wprintf(b, "    if (!desi_future_poll(self->__child)) return 0;\n")
    term.Wprintf(b, "    /* child ready; compute result */\n")
    term.Wprintf(b, "    self->__result = (%s);\n", retExpr)
    term.Wprintf(b, "    return 1;\n")
  } else {
    term.Wprintf(b, "    return 1;\n")
  }
  term.Wprintf(b, "  default: return 1;\n")
  term.Wprintf(b, "  }\n")
  term.Wprintf(b, "}\n")

  // destroy/get
  term.Wprintf(b, "static void %s__destroy(void* selfv) { if (selfv) free(selfv); }\n", fn.Name)
  term.Wprintf(b, "static int %s__get_int(void* selfv) { %s* self=(%s*)selfv; return self->__result; }\n",
    fn.Name, typeName, typeName)

  // constructor
  term.Wprintf(b, "static struct desi_future %s(%s) {\n", fn.Name, cParamList(fn, info))
  term.Wprintf(b, "  %s* self = (%s*)malloc(sizeof(%s));\n", typeName, typeName, typeName)
  term.Wprintf(b, "  if (!self) { %s tmp={0}; return desi_future_make(%s__poll, %s__destroy, %s__get_int, NULL, NULL); }\n",
    typeName, fn.Name, fn.Name, fn.Name)
  term.Wprintf(b, "  self->__state = 0; self->__result = 0; self->__child = (struct desi_future){0};\n")
  if hasAwait {
    if msExpr == "" {
      msExpr = "0"
    }
    term.Wprintf(b, "  self->__ms = (%s);\n", msExpr)
  } else {
    term.Wprintf(b, "  self->__ms = 0;\n")
  }
  term.Wprintf(b, "  return desi_future_make(%s__poll, %s__destroy, %s__get_int, NULL, self);\n", fn.Name, fn.Name, fn.Name)
  term.Wprintf(b, "}\n")
}
