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
			return KindUnknown
		}
		return elem

	case *ast.IdentExpr:
		// Prefer locals first
		if vi, ok := c.scope.lookup(v.Name); ok {
			vi.read = true
			return vi.kind
		}
		// from-import binding?
		if orig, ok := c.aliases[v.Name]; ok {
			if ci, ok := c.info.Consts[orig]; ok {
				return ci.Kind
			}
			if _, ok := c.info.Funcs[orig]; ok {
				c.errors = append(c.errors, ErrImportedFuncNotValueAt(v.Span, orig, v.Name))
				return KindUnknown
			}
		}
		// Bare module alias used as a value → error
		if modPath, ok := c.modAliases[v.Name]; ok {
			c.errors = append(c.errors, ErrModuleAliasNotValueAt(v.Span, v.Name, modPath))
			return KindUnknown
		}
		if _, isFn := c.info.Funcs[v.Name]; isFn {
			return KindUnknown
		}
		// undefined
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
			if k == KindInt || k == KindFloat || k == KindUnknown {
				return k
			}
			return KindUnknown
		case "!", "not":
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
		// handled below
	case *ast.IndexExpr:
		return KindUnknown
	case *ast.CallExpr:
		// handled below
	default:
		return KindUnknown
	}

	// ---------------- existing logic preserved below ----------------

	switch v := e.(type) {
	case *ast.FieldExpr:
		// Module-alias constant access: m.CONST
		if id, ok := v.X.(*ast.IdentExpr); ok {
			if _, isAlias := c.modAliases[id.Name]; isAlias {
				if ci, ok := c.info.Consts[v.Name]; ok {
					if !c.isPublicConst(v.Name) {
						c.errors = append(c.errors, ErrNotPublic(v.Name, "module constant access"))
					}
					return ci.Kind
				}
				if _, ok := c.info.Funcs[v.Name]; ok {
					c.errors = append(c.errors, ErrFunctionNotValueAt(v.Span, v.Name))
					return KindUnknown
				}
				if _, ok := c.info.Structs[v.Name]; ok {
					c.errors = append(c.errors, ErrTypeNotValueAt(v.Span, v.Name))
					return KindUnknown
				}
				return KindUnknown
			}
		}

		// Struct field chains (a.b.c)
		baseName, path, ok := decomposeFieldExpr(v)
		if ok && baseName != "" && len(path) > 0 {
			// locals shadow module aliases
			if vi, ok := c.scope.lookup(baseName); ok {
				vi.read = true
				if vi.kind != KindStruct || vi.structName == "" {
					c.errors = append(c.errors, ErrFieldAccessOnNonStructAt(v.Span, baseName))
					return KindUnknown
				}
				current := vi.structName
				for i, seg := range path {
					si, ok := c.info.Structs[current]
					if !ok {
						c.errors = append(c.errors, ErrUnknownStructTypeAt(v.Span, current))
						return KindUnknown
					}
					tText, ok := si.Fields[seg]
					if !ok {
						c.errors = append(c.errors, ErrUnknownFieldOnStructAt(v.Span, seg, current))
						return KindUnknown
					}
					k, sname := mapTypeOrStruct(tText, c.info)
					if i < len(path)-1 {
						if k != KindStruct || sname == "" {
							c.errors = append(c.errors, ErrFieldOnNotStructAt(v.Span, seg, current))
							return KindUnknown
						}
						current = sname
						continue
					}
					return k
				}
				return KindUnknown
			}
			// if base is a module alias, leave handling to call/const paths
			if _, isAlias := c.modAliases[baseName]; isAlias {
				return KindUnknown
			}
		}
		return KindUnknown

	case *ast.IndexExpr:
		return KindUnknown

	case *ast.CallExpr:
		// std shims first
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			// io.println(...)
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "io" && fe.Name == "println" {
				for i, a := range v.Args {
					ak := c.kindOfExpr(a)
					switch ak {
					case KindInt, KindStr, KindBool, KindFloat, KindUnknown:
					case KindVoid:
						c.errors = append(c.errors, ErrBuiltinArgVoidAt(v.Span, "io.println", i+1))
					default:
						c.errors = append(c.errors, ErrBuiltinArgUnsupportedKindAt(v.Span, "io.println", i+1, fmt.Sprintf("%s", ak)))
					}
				}
				return KindVoid
			}
			// fs.read_all(path: str) -> str
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "fs" && fe.Name == "read_all" {
				if len(v.Args) != 1 {
					c.errors = append(c.errors, ErrBuiltinWrongArityAt(v.Span, "fs.read_all (path: str)", 1, len(v.Args)))
				} else if ak := c.kindOfExpr(v.Args[0]); ak != KindStr && ak != KindUnknown {
					c.errors = append(c.errors, ErrTypeMismatch("str", fmt.Sprintf("%s", ak), "fs.read_all path"))
				}
				return KindStr
			}
			// os.exit(code: int) -> void
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "os" && fe.Name == "exit" {
				if len(v.Args) != 1 {
					c.errors = append(c.errors, ErrBuiltinWrongArityAt(v.Span, "os.exit (code: int)", 1, len(v.Args)))
				} else if ak := c.kindOfExpr(v.Args[0]); ak != KindInt && ak != KindUnknown {
					c.errors = append(c.errors, ErrTypeMismatch("int", fmt.Sprintf("%s", ak), "os.exit code"))
				}
				return KindVoid
			}
			// mem.free(x) -> void
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "mem" && fe.Name == "free" {
				if len(v.Args) != 1 {
					c.errors = append(c.errors, ErrBuiltinWrongArityAt(v.Span, "mem.free", 1, len(v.Args)))
				}
				return KindVoid
			}
			// str.len(s) -> int
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "len" {
				if len(v.Args) != 1 {
					c.errors = append(c.errors, ErrBuiltinWrongArityAt(v.Span, "str.len (str)", 1, len(v.Args)))
				}
				return KindInt
			}
			// str.at(s,i) -> int
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "at" {
				if len(v.Args) != 2 {
					c.errors = append(c.errors, ErrBuiltinWrongArityAt(v.Span, "str.at (str,int)", 2, len(v.Args)))
				}
				return KindInt
			}
			// str.from_code(i) -> str
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "str" && fe.Name == "from_code" {
				if len(v.Args) != 1 {
					c.errors = append(c.errors, ErrBuiltinWrongArityAt(v.Span, "str.from_code (int)", 1, len(v.Args)))
				}
				return KindStr
			}

			// enum constructors: Enum.Variant(payload?)
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				if einfo, ok := c.info.Enums[id.Name]; ok {
					vt, ok := einfo.Variants[fe.Name]
					if !ok {
						c.errors = append(c.errors, ErrUnknownEnumVariantAt(v.Span, fe.Name, id.Name))
						return KindUnknown
					}
					vtLower := strings.TrimSpace(strings.ToLower(vt))
					expect := 0
					if vtLower != "" && vtLower != "none" && vtLower != "void" {
						expect = 1
					}
					if len(v.Args) != expect {
						c.errors = append(c.errors, ErrEnumCtorWrongArityAt(v.Span, id.Name, fe.Name, expect, len(v.Args)))
					} else if expect == 1 {
						wantK, _ := mapTypeOrStruct(vt, c.info)
						gotK := c.kindOfExpr(v.Args[0])
						if wantK != KindUnknown {
							if _, ok := unifyKinds(wantK, gotK); !ok {
								c.errors = append(c.errors, ErrTypeMismatch(
									fmt.Sprintf("%s", wantK), fmt.Sprintf("%s", gotK), "call argument"))
							}
						}
					}
					return KindEnum
				}
			}

			// module alias call: m.add(...)
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				if _, isLocal := c.scope.lookup(id.Name); !isLocal {
					if _, isAlias := c.modAliases[id.Name]; isAlias {
						if sig, ok := c.info.Funcs[fe.Name]; ok {
							// visibility
							if !c.isPublicFunc(fe.Name) {
								c.errors = append(c.errors, ErrNotPublic(fe.Name, "module alias call"))
							}
							if len(sig.Params) != len(v.Args) {
								c.errors = append(c.errors, ErrModuleCallWrongArityAt(v.Span, id.Name, fe.Name, len(sig.Params), len(v.Args)))
							}
							n := _min(len(sig.Params), len(v.Args))
							for i := 0; i < n; i++ {
								ak := c.kindOfExpr(v.Args[i])
								pk := sig.Params[i]
								if _, ok := unifyKinds(pk, ak); !ok {
									c.errors = append(c.errors, ErrTypeMismatch(
										fmt.Sprintf("%s", pk), fmt.Sprintf("%s", ak), "call argument"))
								}
							}
							return sig.Ret
						}
						c.errors = append(c.errors, ErrUnknownSymbolInModuleAliasAt(v.Span, fe.Name, id.Name))
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
					c.errors = append(c.errors, ErrBuiltinArgVoidAt(v.Span, "print", i+1))
				default:
					c.errors = append(c.errors, ErrBuiltinArgUnsupportedKindAt(v.Span, "print", i+1, fmt.Sprintf("%s", ak)))
				}
			}
			return KindVoid
		}

		// user function call (supports from-import aliasing)
		if id, ok := v.Callee.(*ast.IdentExpr); ok {
			name := id.Name
			wasAlias := false
			if orig, ok := c.aliases[name]; ok {
				name = orig
				wasAlias = true
			}
			if sig, ok := c.info.Funcs[name]; ok {
				if !wasAlias && !c.info.FuncsLocal[name] {
					if (id.Span != ast.Span{}) {
						c.errors = append(c.errors, ErrNotPublicAt(id.Span, name, "unqualified cross-module use"))
					} else {
						c.errors = append(c.errors, ErrNotPublic(name, "unqualified cross-module use"))
					}
				}
				if len(sig.Params) != len(v.Args) {
					c.errors = append(c.errors, ErrCallWrongArityAt(v.Span, name, len(sig.Params), len(v.Args)))
				}
				n := _min(len(sig.Params), len(v.Args))
				for i := 0; i < n; i++ {
					ak := c.kindOfExpr(v.Args[i])
					pk := sig.Params[i]
					if _, ok := unifyKinds(pk, ak); !ok {
						c.errors = append(c.errors, ErrTypeMismatch(
							fmt.Sprintf("%s", pk), fmt.Sprintf("%s", ak), "call argument"))
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
