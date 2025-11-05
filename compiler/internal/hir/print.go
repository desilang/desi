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
		if x == nil {
			wprintf(w, "<nil>\n")
			return
		}
		for _, f := range x.Funcs {
			Print(w, f)
		}
	case *Func:
		// header with optional params
		if len(x.Params) == 0 {
			wprintf(w, "func %s\n", x.Name)
		} else {
			wprintf(w, "func %s(", x.Name)
			for i, p := range x.Params {
				if i > 0 {
					wprintf(w, ", ")
				}
				wprintf(w, "%%%s", p.Name)
			}
			wprintf(w, ")\n")
		}
		for _, b := range x.Blocks {
			Print(w, b)
		}
	case *Block:
		wprintf(w, "  block %s\n", x.Name)
		for _, st := range x.Stmts {
			switch s := st.(type) {

			// ------- core statements -------
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
			case *IncRef:
				wprintf(w, "    incref %s\n", s.Val.String())
			case *DecRef:
				wprintf(w, "    decref %s\n", s.Val.String())

			// ------- arena/refcount helpers -------
			case *ArenaAlloc:
				wprintf(w, "    %s = arena.alloc(", s.Dst.String())
				wprintf(w, "%s", s.Arena.String())
				for _, a := range s.Args {
					wprintf(w, ", %s", a.String())
				}
				wprintf(w, ")\n")
			case *DestroyArena:
				wprintf(w, "    destroy_arena %s\n", s.Arena.String())

			// ------- control flow -------
			case *If:
				if s.Else != nil {
					wprintf(w, "    if %s then %s else %s\n", s.Cond.String(), s.Then.Name, s.Else.Name)
				} else {
					wprintf(w, "    if %s then %s\n", s.Cond.String(), s.Then.Name)
				}
			case *While:
				wprintf(w, "    while %s do %s\n", s.Cond.String(), s.Body.Name)

			// ------- M8A async/futures surface -------
			case *FutureNew:
				wprintf(w, "    %s = future.new\n", s.Dst.String())
			case *Await:
				wprintf(w, "    await %s -> %s\n", s.Fut.String(), s.Dst.String())
			case *FutureComplete:
				wprintf(w, "    future.complete %s, %s\n", s.Fut.String(), s.Val.String())

			// ------- M8H frame sugar -------
			case *FrameSet:
				wprintf(w, "    frame.set %s, %s\n", s.Slot, s.Val.String())
			case *FrameGet:
				wprintf(w, "    %s = frame.get %s\n", s.Dst.String(), s.Slot)
			}
		}
	default:
		wprintf(w, "<?>\n")
	}
}
