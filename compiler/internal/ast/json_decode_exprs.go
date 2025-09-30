package ast

import (
	"fmt"
)

/* ---------- expressions ---------- */

func fromJExpr(v any) (Expr, error) {
	m, ok := asMap(v)
	if !ok {
		return nil, fmt.Errorf("AST JSON: expr not object")
	}
	switch getString(m, "kind") {
	case "IdentExpr":
		return &IdentExpr{Name: getString(m, "name"), Span: parseSpan(getMap(m, "span"))}, nil
	case "IntLit":
		return &IntLit{Value: getString(m, "value"), Span: parseSpan(getMap(m, "span"))}, nil
	case "StrLit":
		return &StrLit{Value: getString(m, "value"), Span: parseSpan(getMap(m, "span"))}, nil
	case "BoolLit":
		return &BoolLit{Value: getBool(m, "value"), Span: parseSpan(getMap(m, "span"))}, nil
	case "CallExpr":
		c := &CallExpr{Span: parseSpan(getMap(m, "span"))}
		cv, err := fromJExpr(m["callee"])
		if err != nil {
			return nil, err
		}
		c.Callee = cv
		if arr := getSlice(m, "args"); arr != nil {
			for _, a := range arr {
				ax, err := fromJExpr(a)
				if err != nil {
					return nil, err
				}
				c.Args = append(c.Args, ax)
			}
		}
		return c, nil
	case "IndexExpr":
		e := &IndexExpr{Span: parseSpan(getMap(m, "span"))}
		sv, err := fromJExpr(m["seq"])
		if err != nil {
			return nil, err
		}
		iv, err := fromJExpr(m["index"])
		if err != nil {
			return nil, err
		}
		e.Seq, e.Index = sv, iv
		return e, nil
	case "FieldExpr":
		return &FieldExpr{
			X:    mustExpr(fromJExpr(m["x"])),
			Name: getString(m, "name"),
			Span: parseSpan(getMap(m, "span")),
		}, nil
	case "UnaryExpr":
		return &UnaryExpr{
			Op:   getString(m, "op"),
			X:    mustExpr(fromJExpr(m["x"])),
			Span: parseSpan(getMap(m, "span")),
		}, nil
	case "BinaryExpr":
		return &BinaryExpr{
			Op:    getString(m, "op"),
			Left:  mustExpr(fromJExpr(m["left"])),
			Right: mustExpr(fromJExpr(m["right"])),
			Span:  parseSpan(getMap(m, "span")),
		}, nil
	default:
		return nil, fmt.Errorf("AST JSON: unknown expr kind %q", getString(m, "kind"))
	}
}

func mustExpr(e Expr, err error) Expr {
	if err != nil {
		return nil
	}
	return e
}
