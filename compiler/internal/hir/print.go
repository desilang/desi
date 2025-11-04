package hir

import (
	"fmt"
	"io"
)

// Print pretty-prints a function-or-module HIR in a deterministic, test-friendly format.
func Print(w io.Writer, n interface{}) {
	switch x := n.(type) {
	case *Module:
		fmt.Fprintf(w, "Module %s\n", x.Name)
		for _, f := range x.Funcs {
			Print(w, f)
		}
	case *Func:
		fmt.Fprintf(w, "func %s\n", x.Name)
		for _, b := range x.Blocks {
			Print(w, b)
		}
	case *Block:
		fmt.Fprintf(w, "  block %s\n", x.Name)
		for _, s := range x.Stmts {
			switch s := s.(type) {
			case *Let:
				if s.Init != nil {
					fmt.Fprintf(w, "    let %s = %s\n", s.Name, s.Init.String())
				} else {
					fmt.Fprintf(w, "    let %s\n", s.Name)
				}
			case *Assign:
				fmt.Fprintf(w, "    %s = %s\n", s.LHS, s.RHS.String())
			case *Call:
				// print like: %dst = call fn(arg1, arg2)
				if s.Dst.Name != "" {
					fmt.Fprintf(w, "    %s = call %s(", s.Dst.String(), s.Fn)
				} else {
					fmt.Fprintf(w, "    call %s(", s.Fn)
				}
				for i, a := range s.Args {
					if i > 0 {
						fmt.Fprint(w, ", ")
					}
					fmt.Fprint(w, a.String())
				}
				fmt.Fprintln(w, ")")
			case *Ret:
				if s.Val == nil {
					fmt.Fprintln(w, "    ret")
				} else {
					fmt.Fprintf(w, "    ret %s\n", s.Val.String())
				}
			case *Drop:
				fmt.Fprintf(w, "    drop %s\n", s.Val.String())
			case *If:
				fmt.Fprintf(w, "    if %s {\n", s.Cond.String())
				Print(w, s.Then)
				fmt.Fprintf(w, "    }")
				if s.Else != nil {
					fmt.Fprintln(w)
					fmt.Fprintln(w, "    else {")
					Print(w, s.Else)
					fmt.Fprintln(w, "    }")
				} else {
					fmt.Fprintln(w)
				}
			case *While:
				fmt.Fprintf(w, "    while %s {\n", s.Cond.String())
				Print(w, s.Body)
				fmt.Fprintf(w, "    }\n")
			}
		}
	default:
		fmt.Fprintf(w, "<?>\n")
	}
}
