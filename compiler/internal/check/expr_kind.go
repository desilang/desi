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
	case *ast.FloatLit:
		return KindFloat
	case *ast.StrLit:
		return KindStr
	case *ast.BoolLit:
		return KindBool
	case *ast.StructLit:
		// struct literal has a known, named struct type
		return KindStruct

	case *ast.AwaitExpr:
		// Feature gate + context rule
		if !c.features.Async || !c.fnSig.Async {
			// Prefer span-carrying error when available.
			if (v.Span != ast.Span{}) {
				c.errors = append(c.errors, ErrAwaitOutsideAsyncAt(v.Span))
			} else {
				c.errors = append(c.errors, ErrAwaitOutsideAsyncAt(ast.Span{}))
			}
			return KindUnknown
		}
		inner := c.kindOfExpr(v.Expr)
		if inner != KindFuture {
			if (v.Span != ast.Span{}) {
				c.errors = append(c.errors, ErrAwaitNonFutureAt(v.Span, inner))
			} else {
				c.errors = append(c.errors, ErrAwaitNonFutureAt(ast.Span{}, inner))
			}
			return KindUnknown
		}
		// try to recover the inner element kind from common shapes (calls, etc.)
		elem := c.futureElemOfExpr(v.Expr)
		if elem == KindUnknown {
			// best-effort: still type as unknown if we cannot infer
			return KindUnknown
		}
		return elem

	case *ast.IdentExpr:
		// Prefer locals first (a local var named like a module alias should win)
		if vi, ok := c.scope.lookup(v.Name); ok {
			vi.read = true
			return vi.kind
		}
		// If this identifier is a from-import binding (alias or plain),
		// enforce visibility on the imported symbol and return its kind if it’s a const.
		if orig, ok := c.aliases[v.Name]; ok {
			// function usages are handled in CallExpr; bare function-as-value is not allowed
			// Check constant import
			if ci, ok := c.info.Consts[orig]; ok {
				return ci.Kind
			}
			// Structs used as values are not supported — fallthrough to unknown.
			if _, ok := c.info.Funcs[orig]; ok {
				// Bare function name as a value: error (kept as original behavior).
				c.errors = append(c.errors, fmt.Errorf("module alias or from-import function %q is not a value; call it as %s(...)", orig, v.Name))
				return KindUnknown
			}
		}
		// Bare module alias used as a value? That's an error.
		if modPath, ok := c.modAliases[v.Name]; ok {
			c.errors = append(c.errors, fmt.Errorf("module alias %q (from %q) is not a value; use %s.<symbol>", v.Name, modPath, v.Name))
			return KindUnknown
		}
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
		switch v.Op {
		case "-":
			// numeric negation: preserve int/float/unknown
			if k == KindInt || k == KindFloat || k == KindUnknown {
				return k
			}
			return KindUnknown
		case "!", "not":
			// logical: collapse to int (truthy) unless unknown
			if k == KindUnknown {
				return KindUnknown
			}
			return KindInt
		default:
			return KindUnknown
		}

	case *ast.BinaryExpr:
		lk := c.kindOfExpr(v.Left)
		rk := c.kindOfExpr(v.Right)
		switch v.Op {
		case "+":
			if lk == KindStr || rk == KindStr {
				return KindStr
			}
			if uk, ok := unifyKinds(lk, rk); ok {
				if uk == KindFloat {
					return KindFloat
				}
				if uk == KindInt {
					return KindInt
				}
			}
			return KindUnknown
		case "-", "*", "/", "%":
			if uk, ok := unifyKinds(lk, rk); ok {
				if uk == KindFloat {
					return KindFloat
				}
				if uk == KindInt {
					return KindInt
				}
			}
			return KindUnknown
		case "<", "<=", ">", ">=":
			if _, ok := unifyKinds(lk, rk); ok {
				return KindInt
			}
			return KindUnknown
		case "==", "!=":
			// allow str==str, float==float, int/bool mixes
			if _, ok := unifyKinds(lk, rk); ok || (lk == KindStr && rk == KindStr) {
				return KindInt
			}
			return KindUnknown
		case "and", "or", "|>":
			return KindInt
		default:
			return KindUnknown
		}

	case *ast.FieldExpr:
		// handled below (kept from existing code)
	case *ast.IndexExpr:
		return KindUnknown
	case *ast.CallExpr:
		// handled below (kept from existing code)
	default:
		return KindUnknown
	}

	// ---------------- existing logic preserved below ----------------

	switch v := e.(type) {
	case *ast.FieldExpr:
		// Module-alias constant access: m.CONST
		if id, ok := v.X.(*ast.IdentExpr); ok {
			if _, isAlias := c.modAliases[id.Name]; isAlias {
				// constant?
				if ci, ok := c.info.Consts[v.Name]; ok {
					if !c.isPublicConst(v.Name) {
						c.errors = append(c.errors, ErrNotPublic(v.Name, "module constant access"))
					}
					return ci.Kind
				}
				// function selection without call (m.fn) is not a value.
				if _, ok := c.info.Funcs[v.Name]; ok {
					c.errors = append(c.errors, fmt.Errorf("module symbol %q is a function; call it with arguments", v.Name))
					return KindUnknown
				}
				// struct names aren’t values either at expression position.
				if _, ok := c.info.Structs[v.Name]; ok {
					c.errors = append(c.errors, fmt.Errorf("module symbol %q is a type; cannot be used as a value", v.Name))
					return KindUnknown
				}
				return KindUnknown
			}
		}

		// Struct field chains (a.b.c) for typing
		baseName, path, ok := decomposeFieldExpr(v)
		if ok && baseName != "" && len(path) > 0 {
			// LOCAL shadowing beats module aliasing here.
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

			// If it's NOT a local var but matches a module alias, we treat it in the call/const path.
			if _, isAlias := c.modAliases[baseName]; isAlias {
				// return unknown here; the CallExpr path handles m.func(...), and consts handled above.
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
					case KindInt, KindStr, KindBool, KindFloat, KindUnknown:
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
								c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", wantK), fmt.Sprintf("%s", gotK), "call argument"))
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
					if _, isAlias := c.modAliases[id.Name]; isAlias {
						if sig, ok := c.info.Funcs[fe.Name]; ok {
							// visibility enforcement
							if !c.isPublicFunc(fe.Name) {
								c.errors = append(c.errors, ErrNotPublic(fe.Name, "module alias call"))
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
							// async functions return Future[T]
							return sig.Ret
						}
						// no symbol found in that module alias
						c.errors = append(c.errors, fmt.Errorf("unknown symbol %q in module alias %q", fe.Name, id.Name))
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
				case KindInt, KindStr, KindBool, KindFloat, KindUnknown:
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
			wasAlias := false
			if orig, ok := c.aliases[name]; ok {
				// visibility already enforced at import site
				name = orig
				wasAlias = true
			}
			if sig, ok := c.info.Funcs[name]; ok {
				// visibility for unqualified cross-module calls.
				if !wasAlias && !c.info.FuncsLocal[name] {
					if (id.Span != ast.Span{}) {
						c.errors = append(c.errors, ErrNotPublicAt(id.Span, name, "unqualified cross-module use"))
					} else {
						c.errors = append(c.errors, ErrNotPublic(name, "unqualified cross-module use"))
					}
				}

				if len(sig.Params) != len(v.Args) {
					c.errors = append(c.errors, fmt.Errorf("call to %s: want %d args, got %d", name, len(sig.Params), len(v.Args)))
				}
				n := _min(len(sig.Params), len(v.Args))
				for i := 0; i < n; i++ {
					ak := c.kindOfExpr(v.Args[i])
					pk := sig.Params[i]
					if _, ok := unifyKinds(pk, ak); !ok {
						c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", pk), fmt.Sprintf("%s", ak), "call argument"))
					}
				}
				// async functions return Future[T]
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
