package check

import (
  "fmt"
  "strings"

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
  case *ast.StructLit:
    // struct literal has a known, named struct type
    return KindStruct

  case *ast.IdentExpr:
    // Prefer locals first (a local var named like a module alias should win)
    if vi, ok := c.scope.lookup(v.Name); ok {
      vi.read = true
      return vi.kind
    }

    // From-import alias? (alias -> original symbol)
    if orig, ok := c.aliases[v.Name]; ok {
      // If it resolves to a known CONST, enforce public and return its kind.
      if ci, ok := c.info.Consts[orig]; ok {
        if !c.isPublicConst(orig) {
          // span preferred when available
          if (v.Span != ast.Span{}) {
            c.errors = append(c.errors, ErrNotPublicAt(v.Span, orig, "identifier"))
          } else {
            c.errors = append(c.errors, ErrNotPublic(orig, "identifier"))
          }
          return KindUnknown
        }
        // const is public, yield its type
        return ci.Kind
      }
      // If it resolves to a function, treat like a function name (Unknown as a value),
      // real checking happens at call sites.
      if _, isFn := c.info.Funcs[orig]; isFn {
        return KindUnknown
      }
      // If it resolves to a struct name, as a bare value that’s not valid (no struct value),
      // keep Unknown and let other checks surface better errors if used incorrectly.
      if _, isStruct := c.info.Structs[orig]; isStruct {
        return KindUnknown
      }
      // Otherwise unknown symbol under alias.
      if (v.Span != ast.Span{}) {
        c.errors = append(c.errors, ErrUndefinedNameAt(v.Span, v.Name, "identifier"))
      } else {
        c.errors = append(c.errors, ErrUndefinedName(v.Name, "identifier"))
      }
      return KindUnknown
    }

    // Bare module alias used as a value? That's an error.
    if modPath, ok := c.modAliases[v.Name]; ok {
      c.errors = append(c.errors, fmt.Errorf("module alias %q (from %q) is not a value; use %s.<symbol>", v.Name, modPath, v.Name))
      return KindUnknown
    }

    // If the identifier matches a top-level CONST in this module, allow it as a value.
    if ci, ok := c.info.Consts[v.Name]; ok {
      return ci.Kind
    }

    // Function name by itself isn’t a value but we don’t fail hard here; calls handle it.
    if _, isFn := c.info.Funcs[v.Name]; isFn {
      return KindUnknown
    }

    // undefined name — attach span when available
    if (v.Span != ast.Span{}) {
      c.errors = append(c.errors, ErrUndefinedNameAt(v.Span, v.Name, "identifier"))
    } else {
      c.errors = append(c.errors, ErrUndefinedName(v.Name, "identifier"))
    }
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
    // Two big cases:
    // (A) Struct field chains a.b.c (locals beat module aliases)
    // (B) Module-alias value access: m.CONST  (non-call)
    //
    // First, try struct-field path.
    baseName, path, ok := decomposeFieldExpr(v)
    if ok && baseName != "" && len(path) > 0 {
      // (A) LOCAL shadowing beats module aliasing here.
      if vi, ok := c.scope.lookup(baseName); ok {
        vi.read = true
        if vi.kind != KindStruct || vi.structName == "" {
          c.errors = append(c.errors, fmt.Errorf("field access on non-struct %q", baseName))
          return KindUnknown
        }
        current := vi.structName
        for i, seg := range path {
          si, ok := c.info.Structs[current]
          if !ok {
            c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", current))
            return KindUnknown
          }
          tText, ok := si.Fields[seg]
          if !ok {
            c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", seg, current))
            return KindUnknown
          }
          k, sname := mapTypeOrStruct(tText, c.info)
          if i < len(path)-1 {
            if k != KindStruct || sname == "" {
              c.errors = append(c.errors, fmt.Errorf("field %q on %q is not a struct", seg, current))
              return KindUnknown
            }
            current = sname
            continue
          }
          return k
        }
        return KindUnknown
      }

      // (B) Module alias value access e.g. m.CONST (non-call).
      if modPath, isAlias := c.modAliases[baseName]; isAlias {
        // Only 1-segment allowed for value (m.X). If it’s deeper (m.a.b), leave Unknown for now.
        if len(path) == 1 {
          sym := path[0]
          // CONST?
          if ci, ok := c.info.Consts[sym]; ok {
            if !c.isPublicConst(sym) {
              // not public
              if (v.Span != ast.Span{}) {
                c.errors = append(c.errors, ErrNotPublicAt(v.Span, sym, fmt.Sprintf("module alias %q (from %q)", baseName, modPath)))
              } else {
                c.errors = append(c.errors, ErrNotPublic(sym, fmt.Sprintf("module alias %q (from %q)", baseName, modPath)))
              }
              return KindUnknown
            }
            return ci.Kind
          }
          // STRUCT name used as value is not a value; keep Unknown (calls/ctors handled elsewhere).
          if _, ok := c.info.Structs[sym]; ok {
            return KindUnknown
          }
          // Unknown symbol under alias.
          c.errors = append(c.errors, fmt.Errorf("unknown symbol %q in module alias %q (from %q)", sym, baseName, modPath))
          return KindUnknown
        }
        // Deeper chains off an alias (m.a.b) are not yet supported as values.
        return KindUnknown
      }
    }
    return KindUnknown

  case *ast.IndexExpr:
    return KindUnknown

  case *ast.CallExpr:
    // std shims first
    if fe, ok := v.Callee.(*ast.FieldExpr); ok {
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "io" && fe.Name == "println" {
        for i, a := range v.Args {
          ak := c.kindOfExpr(a)
          switch ak {
          case KindInt, KindStr, KindBool, KindUnknown:
          case KindVoid:
            c.errors = append(c.errors, fmt.Errorf("io.println arg %d is void (no value)", i+1))
          default:
            c.errors = append(c.errors, fmt.Errorf("io.println arg %d has unsupported kind %s", i+1, ak))
          }
        }
        return KindVoid
      }
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "fs" && fe.Name == "read_all" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("fs.read_all: want 1 arg (path: str), got %d", len(v.Args)))
        } else if ak := c.kindOfExpr(v.Args[0]); ak != KindStr && ak != KindUnknown {
          c.errors = append(c.errors, fmt.Errorf("fs.read_all: path must be str, got %s", ak))
        }
        return KindStr
      }
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "os" && fe.Name == "exit" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("os.exit: want 1 arg (code: int), got %d", len(v.Args)))
        } else if ak := c.kindOfExpr(v.Args[0]); ak != KindInt && ak != KindUnknown {
          c.errors = append(c.errors, fmt.Errorf("os.exit: code must be int, got %s", ak))
        }
        return KindVoid
      }
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "mem" && fe.Name == "free" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("mem.free: want 1 arg, got %d", len(v.Args)))
        }
        return KindVoid
      }
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "len" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("str.len: want 1 arg (str), got %d", len(v.Args)))
        }
        return KindInt
      }
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "at" {
        if len(v.Args) != 2 {
          c.errors = append(c.errors, fmt.Errorf("str.at: want 2 args (str,int), got %d", len(v.Args)))
        }
        return KindInt
      }
      if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "from_code" {
        if len(v.Args) != 1 {
          c.errors = append(c.errors, fmt.Errorf("str.from_code: want 1 arg (int), got %d", len(v.Args)))
        }
        return KindStr
      }

      // enum constructors: Enum.Variant(payload?)
      if id, ok := fe.X.(*ast.IdentExpr); ok {
        if einfo, ok := c.info.Enums[id.Name]; ok {
          vt, ok := einfo.Variants[fe.Name]
          if !ok {
            c.errors = append(c.errors, fmt.Errorf("unknown variant %q on enum %q", fe.Name, id.Name))
            return KindUnknown
          }
          vtLower := strings.TrimSpace(strings.ToLower(vt))
          expect := 0
          if vtLower != "" && vtLower != "none" && vtLower != "void" {
            expect = 1
          }
          if len(v.Args) != expect {
            c.errors = append(c.errors, fmt.Errorf("%s.%s expects %d arg(s), got %d", id.Name, fe.Name, expect, len(v.Args)))
          } else if expect == 1 {
            wantK, _ := mapTypeOrStruct(vt, c.info)
            gotK := c.kindOfExpr(v.Args[0])
            if wantK != KindUnknown {
              if _, ok := unifyKinds(wantK, gotK); !ok {
                c.errors = append(c.errors, fmt.Errorf("wrong payload type for %s.%s: expected %s, got %s", id.Name, fe.Name, wantK, gotK))
              }
            }
          }
          return KindEnum
        }
      }

      // module alias call: m.add(...)
      if id, ok := fe.X.(*ast.IdentExpr); ok {
        // local shadowing wins over module alias
        if _, isLocal := c.scope.lookup(id.Name); !isLocal {
          if modPath, isAlias := c.modAliases[id.Name]; isAlias {
            if sig, ok := c.info.Funcs[fe.Name]; ok {
              // ENFORCE PUBLIC
              if !c.isPublicFunc(fe.Name) {
                if (v.Span != ast.Span{}) {
                  c.errors = append(c.errors, ErrNotPublicAt(v.Span, fe.Name, fmt.Sprintf("call via module alias %q (from %q)", id.Name, modPath)))
                } else {
                  c.errors = append(c.errors, ErrNotPublic(fe.Name, fmt.Sprintf("call via module alias %q (from %q)", id.Name, modPath)))
                }
                return KindUnknown
              }
              if len(sig.Params) != len(v.Args) {
                c.errors = append(c.errors, fmt.Errorf("call to %s via %s: want %d args, got %d", fe.Name, id.Name, len(sig.Params), len(v.Args)))
              }
              n := _min(len(sig.Params), len(v.Args))
              for i := 0; i < n; i++ {
                ak := c.kindOfExpr(v.Args[i])
                pk := sig.Params[i]
                if _, ok := unifyKinds(pk, ak); !ok {
                  c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", pk), fmt.Sprintf("%s", ak), "call argument"))
                }
              }
              return sig.Ret
            }
            // no symbol found in that module alias
            c.errors = append(c.errors, fmt.Errorf("unknown symbol %q in module alias %q (from %q)", fe.Name, id.Name, modPath))
            return KindUnknown
          }
        }
      }
    }

    // builtin print(...)
    if id, ok := v.Callee.(*ast.IdentExpr); ok && id.Name == "print" {
      for i, a := range v.Args {
        ak := c.kindOfExpr(a)
        switch ak {
        case KindInt, KindStr, KindBool, KindUnknown:
        case KindVoid:
          c.errors = append(c.errors, fmt.Errorf("print arg %d is void (no value)", i+1))
        default:
          c.errors = append(c.errors, fmt.Errorf("print arg %d has unsupported kind %s", i+1, ak))
        }
      }
      return KindVoid
    }

    // user function call (supports from-import aliasing)
    if id, ok := v.Callee.(*ast.IdentExpr); ok {
      name := id.Name
      orig := name
      if o, ok := c.aliases[name]; ok {
        orig = o
      }
      if sig, ok := c.info.Funcs[orig]; ok {
        // ENFORCE PUBLIC on from-imported calls too.
        if orig != name { // i.e., came from an alias
          if !c.isPublicFunc(orig) {
            if (id.Span != ast.Span{}) {
              c.errors = append(c.errors, ErrNotPublicAt(id.Span, orig, "call"))
            } else {
              c.errors = append(c.errors, ErrNotPublic(orig, "call"))
            }
            return KindUnknown
          }
        }
        if len(sig.Params) != len(v.Args) {
          c.errors = append(c.errors, fmt.Errorf("call to %s: want %d args, got %d", orig, len(sig.Params), len(v.Args)))
        }
        n := _min(len(sig.Params), len(v.Args))
        for i := 0; i < n; i++ {
          ak := c.kindOfExpr(v.Args[i])
          pk := sig.Params[i]
          if _, ok := unifyKinds(pk, ak); !ok {
            c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", pk), fmt.Sprintf("%s", ak), "call argument"))
          }
        }
        return sig.Ret
      }
      c.errors = append(c.errors, ErrUndefinedName(id.Name, "call"))
      return KindUnknown
    }
    return KindUnknown

  default:
    return KindUnknown
  }
}

/* ---------- helpers ---------- */

// decomposeFieldExpr flattens a FieldExpr chain a.b.c -> ("a", ["b","c"], true).
func decomposeFieldExpr(e *ast.FieldExpr) (string, []string, bool) {
  var parts []string
  cur := e
  parts = append(parts, cur.Name)
  for {
    if id, ok := cur.X.(*ast.IdentExpr); ok {
      // reverse parts so they are in left-to-right order
      for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
        parts[i], parts[j] = parts[j], parts[i]
      }
      return id.Name, parts, true
    }
    if fe, ok := cur.X.(*ast.FieldExpr); ok {
      parts = append(parts, fe.Name)
      cur = fe
      continue
    }
    return "", nil, false
  }
}

// enumNameOfExpr: when an expr denotes a value of a particular enum, return its enum name.
func (c *checker) enumNameOfExpr(e ast.Expr) string {
  switch v := e.(type) {
  case *ast.IdentExpr:
    if vi, ok := c.scope.lookup(v.Name); ok && vi.kind == KindEnum {
      return vi.structName
    }
  case *ast.CallExpr:
    if fe, ok := v.Callee.(*ast.FieldExpr); ok {
      if id, ok := fe.X.(*ast.IdentExpr); ok {
        if _, ok := c.info.Enums[id.Name]; ok {
          return id.Name
        }
      }
    }
  }
  return ""
}

/* ---------- public visibility hooks (M10/B) ---------- */

func (c *checker) isPublicFunc(name string) bool {
  return c.info != nil && c.info.FuncsPublic[name]
}
func (c *checker) isPublicConst(name string) bool {
  return c.info != nil && c.info.ConstsPublic[name]
}
func (c *checker) isPublicStruct(name string) bool {
  return c.info != nil && c.info.StructsPublic[name]
}
