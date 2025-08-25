package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

/* ---------- kinds ---------- */

type Kind int

const (
	KindUnknown Kind = iota
	KindInt
	KindStr
	KindBool
	KindVoid
)

func (k Kind) String() string {
	switch k {
	case KindInt:
		return "int"
	case KindStr:
		return "str"
	case KindBool:
		return "bool"
	case KindVoid:
		return "void"
	default:
		return "unknown"
	}
}

/* ---------- public info ---------- */

type FuncSig struct {
	Name   string
	Params []Kind
	Ret    Kind
}

type Info struct {
	Funcs map[string]FuncSig // function table for arity/type checks
}

func CheckFile(f *ast.File) (*Info, []error) {
	info := &Info{Funcs: map[string]FuncSig{}}
	var errs []error

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
			ps = append(ps, mapTextType(p.Type))
		}
		info.Funcs[fn.Name] = FuncSig{Name: fn.Name, Params: ps, Ret: mapTextType(fn.Ret)}
	}

	// check bodies
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			errs = append(errs, checkFunc(info, fn)...)
		}
	}
	return info, errs
}

/* ---------- function + scopes ---------- */

type varInfo struct {
	kind     Kind
	mutable  bool
	declName string
}
type scope struct {
	parent *scope
	vars   map[string]varInfo
}

func (s *scope) lookup(name string) (varInfo, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if v, ok := cur.vars[name]; ok {
			return v, true
		}
	}
	return varInfo{}, false
}
func (s *scope) define(name string, v varInfo) error {
	if _, exists := s.vars[name]; exists {
		return fmt.Errorf("redeclaration of %q", name)
	}
	s.vars[name] = v
	return nil
}

type checker struct {
	info       *Info
	fnSig      FuncSig
	scope      *scope
	errors     []error
	blockDepth int
}

func checkFunc(info *Info, fn *ast.FuncDecl) []error {
	c := &checker{
		info:  info,
		fnSig: info.Funcs[fn.Name],
		scope: &scope{vars: map[string]varInfo{}},
	}
	for i, p := range fn.Params {
		if err := c.scope.define(p.Name, varInfo{
			kind:     mapTextType(p.Type),
			mutable:  false,
			declName: p.Name,
		}); err != nil {
			c.errors = append(c.errors, fmt.Errorf("parameter %d %q: %v", i, p.Name, err))
		}
	}
	for _, s := range fn.Body {
		c.checkStmt(s)
	}
	return c.errors
}

/* ---------- statements ---------- */

func (c *checker) checkStmt(s ast.Stmt) {
	switch st := s.(type) {
	case *ast.LetStmt:
		k := c.kindOfExpr(st.Expr)
		if err := c.scope.define(st.Name, varInfo{kind: k, mutable: st.Mutable, declName: st.Name}); err != nil {
			c.errors = append(c.errors, err)
		}
	case *ast.AssignStmt:
		v, ok := c.scope.lookup(st.Name)
		if !ok {
			c.errors = append(c.errors, fmt.Errorf("assign to undeclared variable %q", st.Name))
			return
		}
		if !v.mutable {
			c.errors = append(c.errors, fmt.Errorf("cannot assign to immutable variable %q", st.Name))
		}
		rk := c.kindOfExpr(st.Expr)
		if k, ok := unifyKinds(v.kind, rk); !ok {
			c.errors = append(c.errors, fmt.Errorf("type mismatch: %q is %s but assigned %s", st.Name, v.kind, rk))
		} else if v.kind == KindUnknown {
			v.kind = k
			c.rebindCurrent(st.Name, v)
		}
	case *ast.ReturnStmt:
		exp := c.fnSig.Ret
		if st.Expr == nil {
			if exp != KindVoid {
				c.errors = append(c.errors, fmt.Errorf("missing return value; function returns %s", exp))
			}
			return
		}
		got := c.kindOfExpr(st.Expr)
		if exp == KindVoid {
			c.errors = append(c.errors, fmt.Errorf("return value in function returning void"))
			return
		}
		if _, ok := unifyKinds(exp, got); !ok {
			c.errors = append(c.errors, fmt.Errorf("return kind mismatch: have %s, got %s", exp, got))
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
		if c.blockDepth > 0 {
			c.errors = append(c.errors, fmt.Errorf("defer is only allowed at function top-level in Stage-0"))
		}
		if _, ok := st.Call.(*ast.CallExpr); !ok {
			c.errors = append(c.errors, fmt.Errorf("defer expects a call expression"))
		}
		c.kindOfExpr(st.Call)
	}
}

func (c *checker) withChildScope(body func()) {
	prev := c.scope
	c.scope = &scope{parent: prev, vars: map[string]varInfo{}}
	body()
	c.scope = prev
}
func (c *checker) withBlock(body func()) {
	c.blockDepth++
	c.withChildScope(body)
	c.blockDepth--
}
func (c *checker) rebindCurrent(name string, v varInfo) {
	for s := c.scope; s != nil; s = s.parent {
		if _, ok := s.vars[name]; ok {
			s.vars[name] = v
			return
		}
	}
}

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
			return vi.kind
		}
		if _, isFn := c.info.Funcs[v.Name]; isFn {
			return KindUnknown
		}
		c.errors = append(c.errors, fmt.Errorf("use of undeclared identifier %q", v.Name))
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
		return KindUnknown
	case *ast.IndexExpr:
		return KindUnknown
	case *ast.CallExpr:
		// std.io.println
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "io" && fe.Name == "println" {
				for i, a := range v.Args {
					ak := c.kindOfExpr(a)
					switch ak {
					case KindInt, KindStr, KindBool:
					case KindVoid:
						c.errors = append(c.errors, fmt.Errorf("io.println arg %d is void (no value)", i+1))
					default:
						c.errors = append(c.errors, fmt.Errorf("io.println arg %d has unsupported kind %s", i+1, ak))
					}
				}
				return KindVoid
			}
			// std.fs.read_all(path: str) -> str
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "fs" && fe.Name == "read_all" {
				if len(v.Args) != 1 {
					c.errors = append(c.errors, fmt.Errorf("fs.read_all: want 1 arg (path: str), got %d", len(v.Args)))
				} else {
					if ak := c.kindOfExpr(v.Args[0]); ak != KindStr && ak != KindUnknown {
						c.errors = append(c.errors, fmt.Errorf("fs.read_all: path must be str, got %s", ak))
					}
				}
				return KindStr
			}
			// std.os.exit(code: int) -> void
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "os" && fe.Name == "exit" {
				if len(v.Args) != 1 {
					c.errors = append(c.errors, fmt.Errorf("os.exit: want 1 arg (code: int), got %d", len(v.Args)))
				} else {
					if ak := c.kindOfExpr(v.Args[0]); ak != KindInt && ak != KindUnknown {
						c.errors = append(c.errors, fmt.Errorf("os.exit: code must be int, got %s", ak))
					}
				}
				return KindVoid
			}
		}
		// user function call
		if id, ok := v.Callee.(*ast.IdentExpr); ok {
			if sig, ok := c.info.Funcs[id.Name]; ok {
				if len(sig.Params) != len(v.Args) {
					c.errors = append(c.errors, fmt.Errorf("call to %s: want %d args, got %d", id.Name, len(sig.Params), len(v.Args)))
				}
				n := min(len(sig.Params), len(v.Args))
				for i := 0; i < n; i++ {
					ak := c.kindOfExpr(v.Args[i])
					pk := sig.Params[i]
					if _, ok := unifyKinds(pk, ak); !ok {
						c.errors = append(c.errors, fmt.Errorf("call to %s: arg %d kind mismatch (want %s, got %s)", id.Name, i+1, pk, ak))
					}
				}
				return sig.Ret
			}
			c.errors = append(c.errors, fmt.Errorf("call to unknown function %q", id.Name))
			return KindUnknown
		}
		return KindUnknown
	default:
		return KindUnknown
	}
}

/* ---------- helpers ---------- */

func mapTextType(t string) Kind {
	switch strings.TrimSpace(strings.ToLower(t)) {
	case "", "void":
		return KindVoid
	case "i32", "int", "u32":
		return KindInt
	case "bool":
		return KindBool
	case "str", "string":
		return KindStr
	default:
		return KindUnknown
	}
}

func unifyKinds(a, b Kind) (Kind, bool) {
	if a == KindUnknown {
		return b, true
	}
	if b == KindUnknown {
		return a, true
	}
	if a == b {
		return a, true
	}
	if (a == KindInt && b == KindBool) || (a == KindBool && b == KindInt) {
		return KindInt, true
	}
	return KindUnknown, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
