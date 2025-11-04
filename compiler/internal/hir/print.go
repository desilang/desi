package hir

import (
	"fmt"
	"io"

	"github.com/desilang/desi/compiler/internal/term"
)

// small formatting helper that ignores write errors via term.Write
func wprintf(w io.Writer, format string, a ...any) {
	term.Write(w, []byte(fmt.Sprintf(format, a...)))
}

// Print pretty-prints a function-or-module HIR in a deterministic, test-friendly format.
func Print(w io.Writer, n interface{}) {
	switch x := n.(type) {
	case *Module:
		wprintf(w, "Module %s\n", x.Name)
		for _, f := range x.Funcs {
			Print(w, f)
		}
	case *Func:
		wprintf(w, "func %s\n", x.Name)
		for _, b := range x.Blocks {
			Print(w, b)
		}
	case *Block:
		wprintf(w, "  block %s\n", x.Name)
		for _, s := range x.Stmts {
			switch s := s.(type) {
			case *Let:
				if s.Init != nil {
					wprintf(w, "    let %s = %s\n", s.Name, s.Init.String())
				} else {
					wprintf(w, "    let %s\n", s.Name)
				}
			case *Assign:
				wprintf(w, "    %s = %s\n", s.LHS, s.RHS.String())
			case *Call:
				if s.Dst.Name != "" {
					wprintf(w, "    %s = call %s(", s.Dst.String(), s.Fn)
				} else {
					wprintf(w, "    call %s(", s.Fn)
				}
				for i, a := range s.Args {
					if i > 0 {
						wprintf(w, ", ")
					}
					wprintf(w, "%s", a.String())
				}
				wprintf(w, ")\n")
			case *Ret:
				if s.Val == nil {
					wprintf(w, "    ret\n")
				} else {
					wprintf(w, "    ret %s\n", s.Val.String())
				}
			case *Drop:
				wprintf(w, "    drop %s\n", s.Val.String())
			case *If:
				// Do not recursively print blocks to avoid duplication;
				// just reference block names. Blocks themselves are printed separately.
				if s.Else != nil {
					wprintf(w, "    if %s then %s else %s\n", s.Cond.String(), s.Then.Name, s.Else.Name)
				} else {
					wprintf(w, "    if %s then %s\n", s.Cond.String(), s.Then.Name)
				}
			case *While:
				wprintf(w, "    while %s do %s\n", s.Cond.String(), s.Body.Name)
			}
		}
	default:
		wprintf(w, "<?>\n")
	}
}
