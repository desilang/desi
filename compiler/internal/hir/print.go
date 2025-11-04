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
				// Do not recursively print blocks to avoid duplication;
				// just reference block names. Blocks themselves are printed separately.
				if s.Else != nil {
					fmt.Fprintf(w, "    if %s then %s else %s\n", s.Cond.String(), s.Then.Name, s.Else.Name)
				} else {
					fmt.Fprintf(w, "    if %s then %s\n", s.Cond.String(), s.Then.Name)
				}
			case *While:
				fmt.Fprintf(w, "    while %s do %s\n", s.Cond.String(), s.Body.Name)
			}
		}
	default:
		fmt.Fprintf(w, "<?>\n")
	}
}
