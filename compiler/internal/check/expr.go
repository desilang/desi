package check

import (
	"fmt"

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

	case *ast.IdentExpr:
		if vi, ok := c.scope.lookup(v.Name); ok {
			vi.read = true
			return vi.kind
		}
		if _, isFn := c.info.Funcs[v.Name]; isFn {
			return KindUnknown
		}
		// undefined name
		c.errors = append(c.errors, ErrUndefinedName(v.Name, "identifier"))
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
		// Support nested base: either an identifier or another field expr.
		baseKind := c.kindOfExpr(v.X)
		if baseKind == KindStruct {
			// Resolve the struct name of v.X
			sname := c.structNameOfExpr(v.X)
			if sname != "" {
				if sInfo, ok := c.info.Structs[sname]; ok {
					if tText, ok := sInfo.Fields[v.Name]; ok {
						k, _ := mapTypeOrStruct(tText, c.info)
						return k
					}
					c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", v.Name, sname))
					return KindUnknown
				}
			}
		}
		// If base is unknown, propagate unknown
		if baseKind == KindUnknown {
			return KindUnknown
		}
		return KindUnknown

	case *ast.IndexExpr:
		return KindUnknown

	case *ast.StructLitExpr:
		// Treat as a value of struct type; callers can obtain sname via structNameOfExpr.
		return KindStruct

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
		}
		// user function call
		if id, ok := v.Callee.(*ast.IdentExpr); ok {
			if sig, ok := c.info.Funcs[id.Name]; ok {
				if len(sig.Params) != len(v.Args) {
					c.errors = append(c.errors, fmt.Errorf("call to %s: want %d args, got %d", id.Name, len(sig.Params), len(v.Args)))
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
