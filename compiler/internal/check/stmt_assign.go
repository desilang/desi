package check

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
)

func (c *checker) checkAssign(st *ast.AssignStmt) {
	// Preferred path: LHS []Expr only.
	if len(st.LHS) > 0 {
		if len(st.LHS) != len(st.Exprs) {
			c.errors = append(c.errors, typedErr(
				"type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
				"assignment", len(st.LHS), len(st.Exprs),
			))
		}

		_max := _min(len(st.LHS), len(st.Exprs))
		for i := 0; i < _max; i++ {
			lhs := st.LHS[i]
			rk := c.kindOfExpr(st.Exprs[i])

			switch lv := lhs.(type) {
			case *ast.IdentExpr:
				v, ok := c.scope.lookup(lv.Name)
				if !ok {
					c.errors = append(c.errors, ErrUndefinedName(lv.Name, "assignment"))
					continue
				}
				if !v.mutable {
					c.errors = append(c.errors, ErrAssignToImmutable(lv.Name, "field assignment"))
					continue
				}
				// Struct whole-value assignment.
				if v.kind == KindStruct {
					rhsStruct := c.structNameOfExpr(st.Exprs[i])
					if rhsStruct != "" && rhsStruct == v.structName {
						v.written = true
						continue
					}
					if rk != KindUnknown {
						c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible struct value", lv.Name))
					}
					v.written = true
					continue
				}
				// Enum whole-value assignment.
				if v.kind == KindEnum {
					rhsEnum := c.enumNameOfExpr(st.Exprs[i])
					if rhsEnum != "" && rhsEnum == v.structName {
						v.written = true
						continue
					}
					if rk != KindUnknown {
						c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible enum value", lv.Name))
					}
					v.written = true
					continue
				}

				if k, ok := unifyKinds(v.kind, rk); !ok {
					c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", v.kind), fmt.Sprintf("%s", rk), "assignment"))
				} else if v.kind == KindUnknown {
					v.kind = k
				}
				v.written = true

			case *ast.FieldExpr:
				// Struct field assignment checking.
				base, path := decomposeFieldChain(lv)
				if base == "" || len(path) == 0 {
					c.errors = append(c.errors, fmt.Errorf("unsupported assignment target"))
					continue
				}
				bv, ok := c.scope.lookup(base)
				if !ok {
					c.errors = append(c.errors, ErrUndefinedName(base, "assignment"))
					continue
				}
				if !bv.mutable {
					c.errors = append(c.errors, ErrAssignToImmutable(base, "field assignment"))
					continue
				}
				if bv.kind != KindStruct || bv.structName == "" {
					c.errors = append(c.errors, fmt.Errorf("cannot assign to field on non-struct %q", base))
					continue
				}
				current := bv.structName
				for j := 0; j < len(path)-1; j++ {
					si, ok := c.info.Structs[current]
					if !ok {
						c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", current))
						current = ""
						break
					}
					ftText, ok := si.Fields[path[j]]
					if !ok {
						c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", path[j], current))
						current = ""
						break
					}
					k, sname := mapTypeOrStruct(ftText, c.info)
					if k != KindStruct || sname == "" {
						c.errors = append(c.errors, fmt.Errorf("field %q on %q is not a struct", path[j], current))
						current = ""
						break
					}
					current = sname
				}
				if current == "" {
					continue
				}
				si, ok := c.info.Structs[current]
				if !ok {
					c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", current))
					continue
				}
				last := path[len(path)-1]
				ftText, ok := si.Fields[last]
				if !ok {
					c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", last, current))
					continue
				}
				want, _ := mapTypeOrStruct(ftText, c.info)
				if want != KindUnknown {
					if _, ok := unifyKinds(want, rk); !ok {
						c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", want), fmt.Sprintf("%s", rk), "field assignment"))
					}
				}
				bv.written = true

			default:
				c.errors = append(c.errors, fmt.Errorf("unsupported assignment target"))
			}
		}
		return
	}

	// ---- Legacy path removed ----
	if len(st.Names) > 0 {
		c.errors = append(c.errors, fmt.Errorf("internal: legacy AssignStmt.Names path is no longer supported; use LHS []Expr"))
	}
}

// decomposeFieldChain flattens a.b.c into ("a", ["b","c"]).
func decomposeFieldChain(e *ast.FieldExpr) (string, []string) {
	var parts []string
	cur := e
	parts = append(parts, cur.Name)
	for {
		if id, ok := cur.X.(*ast.IdentExpr); ok {
			// reverse parts
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			return id.Name, parts
		}
		if fe, ok := cur.X.(*ast.FieldExpr); ok {
			parts = append(parts, fe.Name)
			cur = fe
			continue
		}
		return "", nil
	}
}
