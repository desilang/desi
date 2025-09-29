package ast

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
)

/*** DUMP (pretty outline for CLI) ***/

func DumpFile(f *File) string {
	var b strings.Builder
	if f.Pkg != nil {
		term.Bprintf(&b, "package %s\n", f.Pkg.Name)
	}
	for _, im := range f.Imports {
		term.Bprintf(&b, "import %s\n", im.Path)
	}
	for _, d := range f.Decls {
		switch dn := d.(type) {

		case *FuncDecl:
			if dn.Pub {
				term.Bprintf(&b, "\npub def %s(", dn.Name)
			} else {
				term.Bprintf(&b, "\ndef %s(", dn.Name)
			}
			for i, p := range dn.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				term.Bprintf(&b, "%s: %s", p.Name, p.Type)
			}
			term.Bprintf(&b, ") -> %s:\n", orDefault(dn.Ret, "void"))
			for _, s := range dn.Body {
				switch st := s.(type) {
				case *LetStmt:
					// let / let mut
					if st.Mutable {
						term.Bprintf(&b, "  let mut ")
					} else {
						term.Bprintf(&b, "  let ")
					}
					// print binds (show per-name types when present)
					for i, bd := range st.Binds {
						if i > 0 {
							b.WriteString(", ")
						}
						if strings.TrimSpace(bd.Type) == "" {
							term.Bprintf(&b, "%s", bd.Name)
						} else {
							term.Bprintf(&b, "%s: %s", bd.Name, bd.Type)
						}
					}
					if strings.TrimSpace(st.GroupType) != "" {
						term.Bprintf(&b, " : %s", st.GroupType)
					}
					b.WriteString(" = ")
					for i, e := range st.Values {
						if i > 0 {
							b.WriteString(", ")
						}
						b.WriteString(exprString(e))
					}
					b.WriteString("\n")
				case *AssignStmt:
					term.Bprintf(&b, "  ")
					for i, n := range st.Names {
						if i > 0 {
							b.WriteString(", ")
						}
						b.WriteString(n)
					}
					b.WriteString(" := ")
					for i, e := range st.Exprs {
						if i > 0 {
							b.WriteString(", ")
						}
						b.WriteString(exprString(e))
					}
					b.WriteString("\n")
				case *ReturnStmt:
					if st.Expr == nil {
						term.Bprintf(&b, "  return\n")
					} else {
						term.Bprintf(&b, "  return %s\n", exprString(st.Expr))
					}
				case *ExprStmt:
					term.Bprintf(&b, "  %s\n", exprString(st.Expr))
				case *IfStmt:
					term.Bprintf(&b, "  if %s:\n", exprString(st.Cond))
					for _, s2 := range st.Then {
						term.Bprintf(&b, "    %s\n", stmtString(s2))
					}
					for _, e := range st.Elifs {
						term.Bprintf(&b, "  elif %s:\n", exprString(e.Cond))
						for _, s2 := range e.Body {
							term.Bprintf(&b, "    %s\n", stmtString(s2))
						}
					}
					if st.Else != nil {
						term.Bprintf(&b, "  else:\n")
						for _, s2 := range st.Else {
							term.Bprintf(&b, "    %s\n", stmtString(s2))
						}
					}
				case *WhileStmt:
					term.Bprintf(&b, "  while %s:\n", exprString(st.Cond))
					for _, s2 := range st.Body {
						term.Bprintf(&b, "    %s\n", stmtString(s2))
					}
				case *DeferStmt:
					term.Bprintf(&b, "  defer %s\n", exprString(st.Call))
				}
			}

		case *StructDecl:
			if dn.Pub {
				term.Bprintf(&b, "\npub struct %s:\n", dn.Name)
			} else {
				term.Bprintf(&b, "\nstruct %s:\n", dn.Name)
			}
			for _, f := range dn.Fields {
				term.Bprintf(&b, "  %s: %s\n", f.Name, f.Type)
			}

		case *ConstDecl:
			if dn.Pub {
				if strings.TrimSpace(dn.Type) == "" {
					term.Bprintf(&b, "\npub let %s = %s\n", dn.Name, exprString(dn.Value))
				} else {
					term.Bprintf(&b, "\npub let %s: %s = %s\n", dn.Name, dn.Type, exprString(dn.Value))
				}
			} else {
				if strings.TrimSpace(dn.Type) == "" {
					term.Bprintf(&b, "\nlet %s = %s\n", dn.Name, exprString(dn.Value))
				} else {
					term.Bprintf(&b, "\nlet %s: %s = %s\n", dn.Name, dn.Type, exprString(dn.Value))
				}
			}
		}
	}
	return b.String()
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

func exprString(e Expr) string {
	switch v := e.(type) {
	case *IdentExpr:
		return v.Name
	case *IntLit:
		return v.Value
	case *StrLit:
		return v.Value
	case *BoolLit:
		if v.Value {
			return "true"
		}
		return "false"
	case *CallExpr:
		var parts []string
		for _, a := range v.Args {
			parts = append(parts, exprString(a))
		}
		return exprString(v.Callee) + "(" + strings.Join(parts, ", ") + ")"
	case *IndexExpr:
		return exprString(v.Seq) + "[" + exprString(v.Index) + "]"
	case *FieldExpr:
		return exprString(v.X) + "." + v.Name
	case *UnaryExpr:
		return v.Op + " " + exprString(v.X)
	case *BinaryExpr:
		return "(" + exprString(v.Left) + " " + v.Op + " " + exprString(v.Right) + ")"
	default:
		return "<expr>"
	}
}

func stmtString(s Stmt) string {
	switch st := s.(type) {
	case *LetStmt:
		var b strings.Builder
		if st.Mutable {
			b.WriteString("let mut ")
		} else {
			b.WriteString("let ")
		}
		for i, bd := range st.Binds {
			if i > 0 {
				b.WriteString(", ")
			}
			if strings.TrimSpace(bd.Type) == "" {
				b.WriteString(bd.Name)
			} else {
				term.Bprintf(&b, "%s: %s", bd.Name, bd.Type)
			}
		}
		if strings.TrimSpace(st.GroupType) != "" {
			term.Bprintf(&b, " : %s", st.GroupType)
		}
		b.WriteString(" = ")
		for i, e := range st.Values {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(e))
		}
		return b.String()
	case *AssignStmt:
		var b strings.Builder
		for i, n := range st.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(n)
		}
		b.WriteString(" := ")
		for i, e := range st.Exprs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(e))
		}
		return b.String()
	case *ReturnStmt:
		if st.Expr == nil {
			return "return"
		}
		return "return " + exprString(st.Expr)
	case *ExprStmt:
		return exprString(st.Expr)
	case *IfStmt:
		return "if …:"
	case *WhileStmt:
		return "while …:"
	case *DeferStmt:
		return "defer " + exprString(st.Call)
	default:
		return "<stmt>"
	}
}
