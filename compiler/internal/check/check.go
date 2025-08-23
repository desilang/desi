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
	case KindUnknown:
		fallthrough
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
		info.Funcs[fn.Name] = FuncSig{
			Name:   fn.Name,
			Params: ps,
			Ret:    mapTextType(fn.Ret),
		}
	}

	// type-check each function body
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		fnErrs := checkFunc(info, fn)
		errs = append(errs, fnErrs...)
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
	blockDepth int // >0 when inside if/elif/else/while blocks
}

func checkFunc(info *Info, fn *ast.FuncDecl) []error {
	c := &checker{
		info:  info,
		fnSig: info.Funcs[fn.Name],
		scope: &scope{vars: map[string]varInfo{}},
	}

	// params are immutable locals
	for i, p := range fn.Params {
		if err := c.scope.define(p.Name, varInfo{
			kind:     mapTextType(p.Type),
			mutable:  false,
			declName: p.Name,
		}); err != nil {
			c.errors = append(c.errors, fmt.Errorf("parameter %d %q: %v", i, p.Name, err))
		}
	}

	// body (with branch/loop scopes)
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
		if err := c.scope.define(st.Name, varInfo{
			kind:     k,
			mutable:  st.Mutable,
			declName: st.Name,
		}); err != nil {
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
			// refine unknown
			v.kind = k
			c.rebindCurrent(st.Name, v)
		}

	case *ast.ReturnStmt:
		expect := c.fnSig.Ret
		if st.Expr == nil {
			if expect != KindVoid {
				c.errors = append(c.errors, fmt.Errorf("missing return value; function returns %s", expect))
			}
			return
		}
		got := c.kindOfExpr(st.Expr)
		if expect == KindVoid {
			c.errors = append(c.errors, fmt.Errorf("return value in function returning void"))
			return
		}
		if _, ok := unifyKinds(expect, got); !ok {
			c.errors = append(c.errors, fmt.Errorf("return kind mismatch: have %s, got %s", expect, got))
		}

	case *ast.ExprStmt:
		c.kindOfExpr(st.Expr) // validate calls, etc.

	case *ast.IfStmt:
		ck := c.kindOfExpr(st.Cond)
		if ck != KindBool && ck != KindInt && ck != KindUnknown {
			c.errors = append(c.errors, fmt.Errorf("if-condition must be bool/int, got %s", ck))
		}
		c.withBlock(func() {
			for _, s2 := range st.Then {
				c.checkStmt(s2)
			}
		})
		for _, el := range st.Elifs {
			ck := c.kindOfExpr(el.Cond)
			if ck != KindBool && ck != KindInt && ck != KindUnknown {
				c.errors = append(c.errors, fmt.Errorf("elif-condition must be bool/int, got %s", ck))
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
		ck := c.kindOfExpr(st.Cond)
		if ck != KindBool && ck != KindInt && ck != KindUnknown {
			c.errors = append(c.errors, fmt.Errorf("while-condition must be bool/int, got %s", ck))
		}
		c.withBlock(func() {
			for _, s2 := range st.Body {
				c.checkStmt(s2)
			}
		})

	case *ast.DeferStmt:
		// Stage-0 restriction: only allowed at function top-level (not inside blocks)
		if c.blockDepth > 0 {
			c.errors = append(c.errors, fmt.Errorf("defer is only allowed at function top-level in Stage-0"))
		}
		// and must be a call expression
		if _, ok := st.Call.(*ast.CallExpr); !ok {
			c.errors = append(c.errors, fmt.Errorf("defer expects a call expression"))
			// still traverse to catch other errors inside
		}
		c.kindOfExpr(st.Call)

	default:
		// future statements
	}
}

func (c *checker) withChildScope(body func()) {
	child := &scope{parent: c.scope, vars: map[string]varInfo{}}
	prev := c.scope
	c.scope = child
	body()
	c.scope = prev
}

func (c *checker) withBlock(body func()) {
	c.blockDepth++
	c.withChildScope(body)
	c.blockDepth--
}

func (c *checker) rebindCurrent(name string, v varInfo) {
	if _, exists := c.scope.vars[name]; exists {
		c.scope.vars[name] = v
		return
	}
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
			return KindUnknown
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
			if lk == KindUnknown || rk == KindUnknown {
				return KindUnknown
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
		// io.println(...) strict: args must be int/str/bool
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok && id.Name == "io" && fe.Name == "println" {
				for i, a := range v.Args {
					ak := c.kindOfExpr(a)
					switch ak {
					case KindInt, KindStr, KindBool:
					case KindVoid:
						c.errors = append(c.errors, fmt.Errorf("io.println arg %d is void (no value)", i+1))
					case KindUnknown:
						c.errors = append(c.errors, fmt.Errorf("io.println arg %d has unknown kind; only int/str/bool are allowed", i+1))
					default:
						c.errors = append(c.errors, fmt.Errorf("io.println arg %d has unsupported kind %s", i+1, ak))
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

// unifyKinds returns a resulting compatible kind and whether a,b are compatible.
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
	// allow int<->bool in Stage-0
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
