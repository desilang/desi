package ast

import (
	"fmt"
)

/* ---------- statements ---------- */

func fromJStmt(v any) (Stmt, error) {
	m, ok := asMap(v)
	if !ok {
		return nil, fmt.Errorf("AST JSON: stmt not object")
	}
	switch getString(m, "kind") {
	case "LetStmt":
		st := &LetStmt{
			Mutable:   getBool(m, "mutable"),
			GroupType: getString(m, "groupType"),
			Span:      parseSpan(getMap(m, "span")),
		}
		// binds
		if arr := getSlice(m, "binds"); arr != nil {
			for _, b := range arr {
				bm, ok := asMap(b)
				if !ok || getString(bm, "kind") != "LetBind" {
					continue
				}
				st.Binds = append(st.Binds, LetBind{
					Name: getString(bm, "name"),
					Type: getString(bm, "type"),
					Span: parseSpan(getMap(bm, "span")),
				})
			}
		}
		// values
		if arr := getSlice(m, "values"); arr != nil {
			for _, e := range arr {
				ex, err := fromJExpr(e)
				if err != nil {
					return nil, err
				}
				if ex != nil {
					st.Values = append(st.Values, ex)
				}
			}
		}
		return st, nil

	case "AssignStmt":
		st := &AssignStmt{
			Span: parseSpan(getMap(m, "span")),
		}
		// names
		if arr, ok := m["names"].([]any); ok {
			for _, nv := range arr {
				if s, ok := nv.(string); ok {
					st.Names = append(st.Names, s)
				}
			}
		}
		// exprs
		if arr := getSlice(m, "exprs"); arr != nil {
			for _, e := range arr {
				ex, err := fromJExpr(e)
				if err != nil {
					return nil, err
				}
				if ex != nil {
					st.Exprs = append(st.Exprs, ex)
				}
			}
		}
		return st, nil

	case "ReturnStmt":
		st := &ReturnStmt{Span: parseSpan(getMap(m, "span"))}
		if ev := m["expr"]; ev != nil {
			ex, err := fromJExpr(ev)
			if err != nil {
				return nil, err
			}
			st.Expr = ex
		}
		return st, nil

	case "ExprStmt":
		ex, err := fromJExpr(m["expr"])
		if err != nil {
			return nil, err
		}
		return &ExprStmt{Expr: ex, Span: parseSpan(getMap(m, "span"))}, nil

	case "IfStmt":
		st := &IfStmt{
			Span: parseSpan(getMap(m, "span")),
		}
		// cond
		if c, err := fromJExpr(m["cond"]); err == nil {
			st.Cond = c
		} else {
			return nil, err
		}
		// then
		if arr := getSlice(m, "then"); arr != nil {
			for _, s := range arr {
				ss, err := fromJStmt(s)
				if err != nil {
					return nil, err
				}
				if ss != nil {
					st.Then = append(st.Then, ss)
				}
			}
		}
		// elifs
		if arr := getSlice(m, "elifs"); arr != nil {
			for _, e := range arr {
				em, ok := asMap(e)
				if !ok || getString(em, "kind") != "ElseIf" {
					continue
				}
				el := ElseIf{
					Span: parseSpan(getMap(em, "span")),
				}
				if c, err := fromJExpr(em["cond"]); err == nil {
					el.Cond = c
				} else {
					return nil, err
				}
				if body := getSlice(em, "body"); body != nil {
					for _, s := range body {
						ss, err := fromJStmt(s)
						if err != nil {
							return nil, err
						}
						if ss != nil {
							el.Body = append(el.Body, ss)
						}
					}
				}
				st.Elifs = append(st.Elifs, el)
			}
		}
		// else
		if arr := getSlice(m, "else"); arr != nil {
			for _, s := range arr {
				ss, err := fromJStmt(s)
				if err != nil {
					return nil, err
				}
				if ss != nil {
					st.Else = append(st.Else, ss)
				}
			}
		}
		return st, nil

	case "WhileStmt":
		st := &WhileStmt{Span: parseSpan(getMap(m, "span"))}
		if c, err := fromJExpr(m["cond"]); err == nil {
			st.Cond = c
		} else {
			return nil, err
		}
		if arr := getSlice(m, "body"); arr != nil {
			for _, s := range arr {
				ss, err := fromJStmt(s)
				if err != nil {
					return nil, err
				}
				if ss != nil {
					st.Body = append(st.Body, ss)
				}
			}
		}
		return st, nil

	case "DeferStmt":
		ex, err := fromJExpr(m["call"])
		if err != nil {
			return nil, err
		}
		return &DeferStmt{Call: ex, Span: parseSpan(getMap(m, "span"))}, nil

	default:
		return nil, fmt.Errorf("AST JSON: unknown stmt kind %q", getString(m, "kind"))
	}
}
