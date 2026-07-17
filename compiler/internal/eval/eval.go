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
		if id, ok := x.Callee.(*ast.Ident); ok {
			switch id.Name {
			case "print":
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

			case "ast_get_name":
				if len(x.Args) != 1 {
					return nil, fmt.Errorf("ast_get_name expects 1 argument")
				}
				argVal, err := Eval(x.Args[0], env)
				if err != nil {
					return nil, err
				}
				nodeVal, ok := argVal.(AstNodeValue)
				if !ok {
					return nil, fmt.Errorf("ast_get_name expects an AST node value")
				}
				if nodeVal.Node == nil {
					return StrValue{Val: ""}, nil
				}
				if fd, ok := nodeVal.Node.(*ast.FuncDecl); ok {
					return StrValue{Val: fd.Name.Name}, nil
				}
				if cd, ok := nodeVal.Node.(*ast.ClassDecl); ok {
					return StrValue{Val: cd.Name.Name}, nil
				}
				return StrValue{Val: ""}, nil

			case "ast_set_name":
				if len(x.Args) != 2 {
					return nil, fmt.Errorf("ast_set_name expects 2 arguments")
				}
				argVal, err := Eval(x.Args[0], env)
				if err != nil {
					return nil, err
				}
				nodeVal, ok := argVal.(AstNodeValue)
				if !ok {
					return nil, fmt.Errorf("ast_set_name expects an AST node value as first argument")
				}
				nameVal, err := Eval(x.Args[1], env)
				if err != nil {
					return nil, err
				}
				newName, ok := nameVal.(StrValue)
				if !ok {
					return nil, fmt.Errorf("ast_set_name expects a string as second argument")
				}
				if fd, ok := nodeVal.Node.(*ast.FuncDecl); ok {
					fd.Name.Name = newName.Val
				} else if cd, ok := nodeVal.Node.(*ast.ClassDecl); ok {
					cd.Name.Name = newName.Val
				}
				return nil, nil

			case "ast_get_body":
				if len(x.Args) != 1 {
					return nil, fmt.Errorf("ast_get_body expects 1 argument")
				}
				argVal, err := Eval(x.Args[0], env)
				if err != nil {
					return nil, err
				}
				nodeVal, ok := argVal.(AstNodeValue)
				if !ok {
					return nil, fmt.Errorf("ast_get_body expects an AST node value")
				}
				if fd, ok := nodeVal.Node.(*ast.FuncDecl); ok {
					return AstNodeValue{Node: fd.Body}, nil
				}
				return nil, fmt.Errorf("ast_get_body only supported on function declarations")

			case "ast_create_print_stmt":
				if len(x.Args) != 1 {
					return nil, fmt.Errorf("ast_create_print_stmt expects 1 argument")
				}
				msgVal, err := Eval(x.Args[0], env)
				if err != nil {
					return nil, err
				}
				msgStr, ok := msgVal.(StrValue)
				if !ok {
					return nil, fmt.Errorf("ast_create_print_stmt expects a string argument")
				}
				stmt := &ast.ExprStmt{
					Expr: &ast.CallExpr{
						Callee: &ast.Ident{Name: "print"},
						Args:   []ast.Expr{&ast.StrLit{Value: msgStr.Val}},
						ArgNodes: []ast.CallArg{
							{Expr: &ast.StrLit{Value: msgStr.Val}},
						},
					},
				}
				return AstNodeValue{Node: stmt}, nil

			case "ast_insert_stmt":
				if len(x.Args) != 3 {
					return nil, fmt.Errorf("ast_insert_stmt expects 3 arguments")
				}
				blockVal, err := Eval(x.Args[0], env)
				if err != nil {
					return nil, err
				}
				blockNode, ok := blockVal.(AstNodeValue)
				if !ok {
					return nil, fmt.Errorf("ast_insert_stmt expects an AST block as first argument")
				}
				block, ok := blockNode.Node.(*ast.Block)
				if !ok {
					return nil, fmt.Errorf("first argument is not an AST Block")
				}
				idxVal, err := Eval(x.Args[1], env)
				if err != nil {
					return nil, err
				}
				idx, ok := idxVal.(IntValue)
				if !ok {
					return nil, fmt.Errorf("second argument must be an integer index")
				}
				stmtVal, err := Eval(x.Args[2], env)
				if err != nil {
					return nil, err
				}
				stmtNode, ok := stmtVal.(AstNodeValue)
				if !ok {
					return nil, fmt.Errorf("third argument must be an AST statement")
				}
				stmt, ok := stmtNode.Node.(ast.Stmt)
				if !ok {
					return nil, fmt.Errorf("third argument is not a valid Stmt")
				}

				pos := int(idx.Val)
				if pos < 0 {
					pos = 0
				}
				if pos > len(block.Stmts) {
					pos = len(block.Stmts)
				}
				block.Stmts = append(block.Stmts, nil)
				copy(block.Stmts[pos+1:], block.Stmts[pos:])
				block.Stmts[pos] = stmt
				return nil, nil

			case "ast_add_stmt":
				if len(x.Args) != 2 {
					return nil, fmt.Errorf("ast_add_stmt expects 2 arguments")
				}
				blockVal, err := Eval(x.Args[0], env)
				if err != nil {
					return nil, err
				}
				blockNode, ok := blockVal.(AstNodeValue)
				if !ok {
					return nil, fmt.Errorf("ast_add_stmt expects an AST block as first argument")
				}
				block, ok := blockNode.Node.(*ast.Block)
				if !ok {
					return nil, fmt.Errorf("first argument is not an AST Block")
				}
				stmtVal, err := Eval(x.Args[1], env)
				if err != nil {
					return nil, err
				}
				stmtNode, ok := stmtVal.(AstNodeValue)
				if !ok {
					return nil, fmt.Errorf("second argument must be an AST statement")
				}
				stmt, ok := stmtNode.Node.(ast.Stmt)
				if !ok {
					return nil, fmt.Errorf("second argument is not a valid Stmt")
				}
				block.Stmts = append(block.Stmts, stmt)
				return nil, nil
			}
		}
		return nil, fmt.Errorf("unsupported compile-time call: %T", x.Callee)
	case *ast.FString:
		var res string
		for _, part := range x.Parts {
			val, err := Eval(part, env)
			if err != nil {
				return nil, err
			}
			if val != nil {
				res += val.String()
			}
		}
		return StrValue{Val: res}, nil

	case *ast.FStringExpr:
		return Eval(x.X, env)

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
			if l, ok := lhs.(FloatValue); ok {
				if r, ok := rhs.(FloatValue); ok {
					return FloatValue{Val: l.Val + r.Val}, nil
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
			if l, ok := lhs.(FloatValue); ok {
				if r, ok := rhs.(FloatValue); ok {
					return FloatValue{Val: l.Val - r.Val}, nil
				}
			}
		case "*":
			if l, ok := lhs.(IntValue); ok {
				if r, ok := rhs.(IntValue); ok {
					return IntValue{Val: l.Val * r.Val}, nil
				}
			}
			if l, ok := lhs.(FloatValue); ok {
				if r, ok := rhs.(FloatValue); ok {
					return FloatValue{Val: l.Val * r.Val}, nil
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
			if l, ok := lhs.(FloatValue); ok {
				if r, ok := rhs.(FloatValue); ok {
					if r.Val == 0 {
						return nil, fmt.Errorf("division by zero")
					}
					return FloatValue{Val: l.Val / r.Val}, nil
				}
			}
		case "==":
			switch l := lhs.(type) {
			case IntValue:
				if r, ok := rhs.(IntValue); ok {
					return BoolValue{Val: l.Val == r.Val}, nil
				}
			case FloatValue:
				if r, ok := rhs.(FloatValue); ok {
					return BoolValue{Val: l.Val == r.Val}, nil
				}
			case BoolValue:
				if r, ok := rhs.(BoolValue); ok {
					return BoolValue{Val: l.Val == r.Val}, nil
				}
			case StrValue:
				if r, ok := rhs.(StrValue); ok {
					return BoolValue{Val: l.Val == r.Val}, nil
				}
			}
			return BoolValue{Val: lhs.String() == rhs.String()}, nil
		case "!=":
			switch l := lhs.(type) {
			case IntValue:
				if r, ok := rhs.(IntValue); ok {
					return BoolValue{Val: l.Val != r.Val}, nil
				}
			case FloatValue:
				if r, ok := rhs.(FloatValue); ok {
					return BoolValue{Val: l.Val != r.Val}, nil
				}
			case BoolValue:
				if r, ok := rhs.(BoolValue); ok {
					return BoolValue{Val: l.Val != r.Val}, nil
				}
			case StrValue:
				if r, ok := rhs.(StrValue); ok {
					return BoolValue{Val: l.Val != r.Val}, nil
				}
			}
			return BoolValue{Val: lhs.String() != rhs.String()}, nil
		}
		return nil, fmt.Errorf("unsupported operator: %s", x.Op)

	default:
		// Default to wrapping it as an AST node value for compile-time introspection
		return AstNodeValue{Node: node}, nil
	}
}
