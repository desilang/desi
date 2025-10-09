package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

/* ---------- small helper ---------- */

// resolveModuleInfo returns symbol tables for the module behind a module alias.
// Prefers DefaultModuleInfoProvider; falls back to c.info (single-file tests).
func (c *checker) resolveModuleInfo(modPath string) (*Info, bool) {
	if DefaultModuleInfoProvider != nil {
		if src, ok := DefaultModuleInfoProvider.Lookup(modPath); ok && src != nil {
			return src, true
		}
	}
	// Fallback for unit tests that construct a single ast.File with provider+consumer:
	// treat current file's Info as the "module".
	return c.info, true
}

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
		return KindStruct

	case *ast.AwaitExpr:
		// async gate + await operand type check
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
		elem := c.futureElemOfExpr(v.Expr)
		if elem == KindUnknown {
			return KindUnknown
		}
		return elem

	case *ast.IdentExpr:
		// locals first
		if vi, ok := c.scope.lookup(v.Name); ok {
			vi.read = true
			return vi.kind
		}
		// from-import alias used as bare value:
		//   - if it aliases a const, allow and use const's kind (recorded in Info.Consts when local)
		//   - if it aliases a func/type/enum, it's not a value
		if orig, ok := c.aliases[v.Name]; ok {
			if ci, ok := c.info.Consts[orig]; ok {
				return ci.Kind
			}
			c.errors = append(c.errors, ErrFunctionNotValueAt(v.Span, v.Name))
			return KindUnknown
		}
		// bare module alias as a value
		if modPath, ok := c.modAliases[v.Name]; ok {
			_ = modPath
			c.errors = append(c.errors, ErrModuleAliasNotValueAt(v.Span, v.Name, modPath))
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

	// ---------------- remaining cases ----------------

	switch v := e.(type) {
	case *ast.FieldExpr:
		// Module-alias member access: m.CONST / m.Type / m.Enum
		if id, ok := v.X.(*ast.IdentExpr); ok {
			if modPath, isAlias := c.modAliases[id.Name]; isAlias {
				if src, ok := c.resolveModuleInfo(modPath); ok && src != nil {
					name := v.Name
					switch {
					case hasConst(src, name):
						// constant access as value (enforce visibility)
						if !src.ConstsPublic[name] {
							c.errors = append(c.errors, ErrNotPublicAt(v.Span, name, "module constant access"))
						}
						return src.Consts[name].Kind
					case hasFunc(src, name):
						// function is not a value
						c.errors = append(c.errors, ErrFunctionNotValueAt(v.Span, id.Name+"."+name))
						return KindUnknown
					case hasStruct(src, name), hasType(src, name), hasEnum(src, name):
						// types/enums are not values
						c.errors = append(c.errors, ErrTypeNotValueAt(v.Span, id.Name+"."+name))
						return KindUnknown
					default:
						c.errors = append(c.errors, ErrUnknownSymbolInModuleAliasAt(v.Span, name, id.Name))
						return KindUnknown
					}
				}
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
			// if base is a module alias, the earlier branch handled it
			if _, isAlias := c.modAliases[baseName]; isAlias {
				return KindUnknown
			}
		}
		return KindUnknown

	case *ast.IndexExpr:
		return KindUnknown

	case *ast.CallExpr:
		// ---------------- builtins ----------------

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

		// enum constructors: Enum.Variant(payload?)
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
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
		}

		// ---------------- module alias calls: m.f(...) ----------------
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				// if 'id' is not a local, and is a module alias, resolve in the source module
				if _, isLocal := c.scope.lookup(id.Name); !isLocal {
					if modPath, isAlias := c.modAliases[id.Name]; isAlias {
						if src, ok := c.resolveModuleInfo(modPath); ok && src != nil {
							name := fe.Name

							// types/enums/consts are not callable
							if hasType(src, name) || hasEnum(src, name) {
								c.errors = append(c.errors, ErrTypeNotValueAt(fe.Span, id.Name+"."+name))
								return KindUnknown
							}
							if hasConst(src, name) {
								c.errors = append(c.errors, ErrCallWrongArityAt(v.Span, id.Name+"."+name, 0, len(v.Args)))
								return src.Consts[name].Kind
							}

							// function: enforce visibility + arity + argument kinds
							if sig, ok := src.Funcs[name]; ok {
								if !src.FuncsPublic[name] {
									// non-public under module alias
									c.errors = append(c.errors, ErrNotPublicAt(fe.Span, name, "module-alias call"))
								}
								if len(sig.Params) != len(v.Args) {
									c.errors = append(c.errors, ErrModuleCallWrongArityAt(v.Span, id.Name, name, len(sig.Params), len(v.Args)))
								}
								n := _min(len(sig.Params), len(v.Args))
								for i := 0; i < n; i++ {
									ak := c.kindOfExpr(v.Args[i])
									pk := sig.Params[i]
									if _, ok := unifyKinds(pk, ak); !ok && pk != KindUnknown && ak != KindUnknown {
										c.errors = append(c.errors, ErrTypeMismatch(
											fmt.Sprintf("%s", pk), fmt.Sprintf("%s", ak), "call argument"))
									}
								}
								return sig.Ret
							}

							// unknown member
							c.errors = append(c.errors, ErrUnknownSymbolInModuleAliasAt(fe.Span, name, id.Name))
							return KindUnknown
						}
					}
				}
			}
		}

		// ---------------- user function calls (unqualified / from-import alias) ----------------
		if id, ok := v.Callee.(*ast.IdentExpr); ok {
			name := id.Name
			wasAlias := false
			if orig, ok := c.aliases[name]; ok {
				name = orig
				wasAlias = true
			}
			if sig, ok := c.info.Funcs[name]; ok {
				// If this is not a from-import and not local, it's a cross-module unqualified use
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
