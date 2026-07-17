package eval

import (
	"fmt"
	"os"
	"reflect"
	"strconv"

	"github.com/desilang/desi/compiler/internal/ast"
)

// Value represents any evaluated Desi compile-time value.
type Value interface {
	String() string
}

// IntValue represents an integer value.
type IntValue struct {
	Val int64
}

func (v IntValue) String() string { return fmt.Sprintf("%d", v.Val) }

// FloatValue represents a float value.
type FloatValue struct {
	Val float64
}

func (v FloatValue) String() string { return fmt.Sprintf("%g", v.Val) }

// BoolValue represents a boolean value.
type BoolValue struct {
	Val bool
}

func (v BoolValue) String() string { return fmt.Sprintf("%t", v.Val) }

// StrValue represents a string value.
type StrValue struct {
	Val string
}

func (v StrValue) String() string { return v.Val }

// ListValue represents a list value.
type ListValue struct {
	Elements []Value
}

func (v ListValue) String() string {
	res := "["
	for i, el := range v.Elements {
		if i > 0 {
			res += ", "
		}
		res += el.String()
	}
	res += "]"
	return res
}

// AstNodeValue wraps a compiler AST node for compile-time introspection.
type AstNodeValue struct {
	Node ast.Node
}

func (v AstNodeValue) String() string {
	if v.Node == nil {
		return "Node(nil)"
	}
	return fmt.Sprintf("Node(%s)", reflect.TypeOf(v.Node).Elem().Name())
}

// Env represents the evaluation scope.
type Env struct {
	parent *Env
	vars   map[string]Value
}

func NewEnv(parent *Env) *Env {
	return &Env{
		parent: parent,
		vars:   make(map[string]Value),
	}
}

func (e *Env) Get(name string) (Value, bool) {
	val, ok := e.vars[name]
	if !ok && e.parent != nil {
		return e.parent.Get(name)
	}
	return val, ok
}

func (e *Env) Set(name string, val Value) {
	e.vars[name] = val
}

// Eval evaluates an AST expression or statement inside an environment.
func Eval(node ast.Node, env *Env) (Value, error) {
	if node == nil {
		return nil, nil
	}

	// typed-nil guard
	val := reflect.ValueOf(node)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return nil, nil
	}

	switch x := node.(type) {
	case *ast.Block:
		var lastVal Value
		for _, stmt := range x.Stmts {
			v, err := Eval(stmt, env)
			if err != nil {
				return nil, err
			}
			lastVal = v
		}
		return lastVal, nil

	case *ast.ExprStmt:
		return Eval(x.Expr, env)

	case *ast.CallExpr:
		if id, ok := x.Callee.(*ast.Ident); ok && id.Name == "print" {
			var args []string
			for _, arg := range x.Args {
				val, err := Eval(arg, env)
				if err != nil {
					return nil, err
				}
				if val != nil {
					args = append(args, val.String())
				}
			}
			fmt.Fprint(os.Stderr, "Compile-time print: ")
			for i, arg := range args {
				if i > 0 {
					fmt.Fprint(os.Stderr, " ")
				}
				fmt.Fprint(os.Stderr, arg)
			}
			fmt.Fprintln(os.Stderr)
			return nil, nil
		}
		return nil, fmt.Errorf("compile-time function call only supported for print, got: %T", x.Callee)
	case *ast.IntLit:
		i, err := strconv.ParseInt(x.Text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer literal %q: %v", x.Text, err)
		}
		return IntValue{Val: i}, nil

	case *ast.FloatLit:
		f, err := strconv.ParseFloat(x.Text, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float literal %q: %v", x.Text, err)
		}
		return FloatValue{Val: f}, nil

	case *ast.StrLit:
		return StrValue{Val: x.Value}, nil

	case *ast.BoolLit:
		return BoolValue{Val: x.Value}, nil

	case *ast.Ident:
		if v, ok := env.Get(x.Name); ok {
			return v, nil
		}
		return nil, fmt.Errorf("undefined variable: %s", x.Name)

	case *ast.LetStmt:
		rhs, err := Eval(x.Value, env)
		if err != nil {
			return nil, err
		}
		if x.Name.Name != "" {
			env.Set(x.Name.Name, rhs)
		}
		return rhs, nil

	case *ast.BinaryExpr:
		lhs, err := Eval(x.Lhs, env)
		if err != nil {
			return nil, err
		}
		rhs, err := Eval(x.Rhs, env)
		if err != nil {
			return nil, err
		}

		switch x.Op {
		case "+":
			if l, ok := lhs.(IntValue); ok {
				if r, ok := rhs.(IntValue); ok {
					return IntValue{Val: l.Val + r.Val}, nil
				}
			}
			if l, ok := lhs.(StrValue); ok {
				if r, ok := rhs.(StrValue); ok {
					return StrValue{Val: l.Val + r.Val}, nil
				}
			}
		case "-":
			if l, ok := lhs.(IntValue); ok {
				if r, ok := rhs.(IntValue); ok {
					return IntValue{Val: l.Val - r.Val}, nil
				}
			}
		case "*":
			if l, ok := lhs.(IntValue); ok {
				if r, ok := rhs.(IntValue); ok {
					return IntValue{Val: l.Val * r.Val}, nil
				}
			}
		case "/":
			if l, ok := lhs.(IntValue); ok {
				if r, ok := rhs.(IntValue); ok {
					if r.Val == 0 {
						return nil, fmt.Errorf("division by zero")
					}
					return IntValue{Val: l.Val / r.Val}, nil
				}
			}
		case "==":
			return BoolValue{Val: lhs.String() == rhs.String()}, nil
		case "!=":
			return BoolValue{Val: lhs.String() != rhs.String()}, nil
		}
		return nil, fmt.Errorf("unsupported operator: %s", x.Op)

	default:
		// Default to wrapping it as an AST node value for compile-time introspection
		return AstNodeValue{Node: node}, nil
	}
}
